package github

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	gh "github.com/google/go-github/v68/github"

	"github.com/fmiskovic/softika/internal/model"
)

const (
	maxStargazersFullScan = 5000
	sampleSize            = 500
	perPage               = 100
	maxUserFetchWorkers   = 10
	maxRetries            = 3
)

// Client wraps the GitHub API with rate-limit handling and stargazer sampling.
type Client struct {
	gh  *gh.Client
	now func() time.Time // injectable clock for testing
}

// NewClient creates a Client. If token is empty, unauthenticated (60 req/hr).
func NewClient(token string) *Client {
	c := gh.NewClient(nil)
	if token != "" {
		c = c.WithAuthToken(token)
	}
	return &Client{gh: c, now: time.Now}
}

// NewClientWith creates a Client with an injected go-github client (for testing).
func NewClientWith(ghClient *gh.Client, clock func() time.Time) *Client {
	if clock == nil {
		clock = time.Now
	}
	return &Client{gh: ghClient, now: clock}
}

// FetchRepoInfo retrieves basic repository statistics.
func (c *Client) FetchRepoInfo(ctx context.Context, owner, repo string) (model.RepoInfo, error) {
	r, _, err := c.gh.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return model.RepoInfo{}, fmt.Errorf("fetch repo: %w", err)
	}

	info := model.RepoInfo{
		Owner:         owner,
		Name:          repo,
		Stars:         r.GetStargazersCount(),
		Forks:         r.GetForksCount(),
		Watchers:      r.GetSubscribersCount(), // true watchers
		OpenIssues:    r.GetOpenIssuesCount(),
		CreatedAt:     r.GetCreatedAt().Time,
		Description:   r.GetDescription(),
		DefaultBranch: r.GetDefaultBranch(),
	}

	return info, nil
}

// FetchTotalIssues returns the total number of issues (open + closed) via search.
func (c *Client) FetchTotalIssues(ctx context.Context, owner, repo string) (int, error) {
	query := fmt.Sprintf("repo:%s/%s is:issue", owner, repo)
	result, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
		ListOptions: gh.ListOptions{PerPage: 1},
	})
	if err != nil {
		return 0, fmt.Errorf("search issues: %w", err)
	}
	return result.GetTotal(), nil
}

// StargazerEntry pairs a login with the time they starred.
type StargazerEntry struct {
	Login     string
	StarredAt time.Time
}

// FetchStargazers retrieves stargazers with timestamps.
// For repos with <= maxStargazersFullScan stars, fetches all.
// For larger repos, samples pages randomly to get ~sampleSize entries.
func (c *Client) FetchStargazers(ctx context.Context, owner, repo string, totalStars int) ([]StargazerEntry, error) {
	if totalStars <= 0 {
		return nil, nil
	}

	if totalStars <= maxStargazersFullScan {
		return c.fetchAllStargazers(ctx, owner, repo)
	}

	return c.sampleStargazers(ctx, owner, repo, totalStars)
}

func (c *Client) fetchAllStargazers(ctx context.Context, owner, repo string) ([]StargazerEntry, error) {
	var all []StargazerEntry
	opts := &gh.ListOptions{PerPage: perPage}

	for {
		stargazers, resp, err := c.gh.Activity.ListStargazers(ctx, owner, repo, opts)
		if err != nil {
			retryErr := c.handleRateLimitOrRetry(ctx, err)
			if retryErr == nil {
				continue // rate limit waited, retry same page
			}
			return all, fmt.Errorf("fetch stargazers page %d: %w", opts.Page, err)
		}

		for _, s := range stargazers {
			entry := StargazerEntry{
				Login: s.GetUser().GetLogin(),
			}
			if s.StarredAt != nil {
				entry.StarredAt = s.StarredAt.Time
			}
			all = append(all, entry)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, nil
}

func (c *Client) sampleStargazers(ctx context.Context, owner, repo string, totalStars int) ([]StargazerEntry, error) {
	totalPages := (totalStars + perPage - 1) / perPage
	pagesToFetch := (sampleSize + perPage - 1) / perPage

	if pagesToFetch > totalPages {
		pagesToFetch = totalPages
	}

	pages := pickRandomPages(totalPages, pagesToFetch)

	var all []StargazerEntry
	var lastErr error
	for _, page := range pages {
		entries, err := c.fetchStargazerPage(ctx, owner, repo, page)
		if err != nil {
			lastErr = err
			continue // skip this page, try remaining
		}
		all = append(all, entries...)
	}

	if len(all) == 0 && lastErr != nil {
		return nil, fmt.Errorf("sample stargazers: all pages failed, last error: %w", lastErr)
	}

	return all, nil
}

func (c *Client) fetchStargazerPage(ctx context.Context, owner, repo string, page int) ([]StargazerEntry, error) {
	opts := &gh.ListOptions{PerPage: perPage, Page: page}

	for attempt := 0; attempt < maxRetries; attempt++ {
		stargazers, _, err := c.gh.Activity.ListStargazers(ctx, owner, repo, opts)
		if err != nil {
			retryErr := c.handleRateLimitOrRetry(ctx, err)
			if retryErr == nil {
				continue // rate limit waited, retry same page
			}
			if attempt < maxRetries-1 {
				continue // transient error, retry
			}
			return nil, fmt.Errorf("fetch stargazers page %d: %w", page, err)
		}

		var entries []StargazerEntry
		for _, s := range stargazers {
			entry := StargazerEntry{
				Login: s.GetUser().GetLogin(),
			}
			if s.StarredAt != nil {
				entry.StarredAt = s.StarredAt.Time
			}
			entries = append(entries, entry)
		}
		return entries, nil
	}

	return nil, fmt.Errorf("fetch stargazers page %d: max retries exceeded", page)
}

// pickRandomPages selects n distinct pages from [1, totalPages] spread evenly.
func pickRandomPages(totalPages, n int) []int {
	if n >= totalPages {
		pages := make([]int, totalPages)
		for i := range pages {
			pages[i] = i + 1
		}
		return pages
	}

	// Stratified sampling: divide range into n buckets, pick one random page from each
	pages := make([]int, n)
	bucketSize := float64(totalPages) / float64(n)

	for i := 0; i < n; i++ {
		low := int(float64(i) * bucketSize)
		high := int(float64(i+1) * bucketSize)
		if high > totalPages {
			high = totalPages
		}
		pages[i] = low + rand.Intn(high-low) + 1
	}

	sort.Ints(pages)
	return pages
}

// FetchUserProfile fetches a user's profile for stargazer classification.
func (c *Client) FetchUserProfile(ctx context.Context, login string) (model.StargazerProfile, error) {
	user, _, err := c.gh.Users.Get(ctx, login)
	if err != nil {
		return model.StargazerProfile{}, fmt.Errorf("fetch user %s: %w", login, err)
	}

	now := c.now()
	createdAt := user.GetCreatedAt().Time

	return model.StargazerProfile{
		Login:       login,
		AccountAge:  now.Sub(createdAt),
		CreatedAt:   createdAt,
		PublicRepos: user.GetPublicRepos(),
		Followers:   user.GetFollowers(),
		Following:   user.GetFollowing(),
	}, nil
}

// ProfileResult pairs a successfully fetched profile with its error state.
type ProfileResult struct {
	Profile model.StargazerProfile
	Err     error
}

// FetchUserProfiles fetches profiles for multiple users with concurrency control.
// Returns only successfully fetched profiles and the count of failures.
func (c *Client) FetchUserProfiles(ctx context.Context, logins []string) ([]model.StargazerProfile, int) {
	if len(logins) == 0 {
		return nil, 0
	}

	results := make([]ProfileResult, len(logins))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxUserFetchWorkers)

	for i, login := range logins {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, l string) {
			defer wg.Done()
			defer func() { <-sem }()
			p, err := c.FetchUserProfile(ctx, l)
			results[idx] = ProfileResult{Profile: p, Err: err}
		}(i, login)
	}

	wg.Wait()

	var profiles []model.StargazerProfile
	failCount := 0
	for _, r := range results {
		if r.Err != nil {
			failCount++
			continue
		}
		profiles = append(profiles, r.Profile)
	}

	return profiles, failCount
}

// handleRateLimitOrRetry waits on rate-limit errors and returns nil (caller should retry).
// For non-rate-limit errors, returns the original error (caller should not retry).
func (c *Client) handleRateLimitOrRetry(ctx context.Context, err error) error {
	var rateLimitErr *gh.RateLimitError
	if errors.As(err, &rateLimitErr) {
		waitDuration := time.Until(rateLimitErr.Rate.Reset.Time) + time.Second
		if waitDuration > 0 {
			select {
			case <-time.After(waitDuration):
				return nil // retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}

	var abuseErr *gh.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		retryAfter := abuseErr.GetRetryAfter()
		if retryAfter > 0 {
			select {
			case <-time.After(retryAfter):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		select {
		case <-time.After(60 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("github API error: %w", err)
}
