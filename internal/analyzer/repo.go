package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	ghclient "github.com/fmiskovic/softika/internal/github"
	"github.com/fmiskovic/softika/internal/model"
)

// GitHubClient defines the GitHub API surface needed by the analyzer.
type GitHubClient interface {
	FetchRepoInfo(ctx context.Context, owner, repo string) (model.RepoInfo, error)
	FetchTotalIssues(ctx context.Context, owner, repo string) (int, error)
	FetchStargazers(ctx context.Context, owner, repo string, totalStars int) ([]ghclient.StargazerEntry, error)
	FetchUserProfiles(ctx context.Context, logins []string) ([]model.StargazerProfile, int)
}

// RepoAnalyzer orchestrates the full analysis pipeline for a repository.
type RepoAnalyzer struct {
	client GitHubClient
}

// NewRepoAnalyzer creates a new analyzer with the given GitHub client.
func NewRepoAnalyzer(client GitHubClient) *RepoAnalyzer {
	return &RepoAnalyzer{client: client}
}

// ParseRepo splits "owner/repo" into its parts.
func ParseRepo(input string) (owner, repo string, err error) {
	// Strip GitHub URL prefix if present
	input = strings.TrimPrefix(input, "https://github.com/")
	input = strings.TrimPrefix(input, "http://github.com/")
	input = strings.TrimPrefix(input, "github.com/")
	input = strings.TrimSuffix(input, "/")

	parts := strings.SplitN(input, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format %q, expected owner/repo", input)
	}
	return parts[0], parts[1], nil
}

// Analyze runs the full analysis pipeline on a repo.
func (a *RepoAnalyzer) Analyze(ctx context.Context, owner, repo string, onProgress func(string)) (model.AnalysisResult, error) {
	progress := func(msg string) {
		if onProgress != nil {
			onProgress(msg)
		}
	}

	result := model.AnalysisResult{
		AnalyzedAt: time.Now(),
	}

	// Step 1: Fetch repo info
	progress("Fetching repository info...")
	repoInfo, err := a.client.FetchRepoInfo(ctx, owner, repo)
	if err != nil {
		return result, fmt.Errorf("fetch repo info: %w", err)
	}
	result.Repo = repoInfo

	// Step 2: Fetch total issues via search; fall back to open issue count
	progress("Counting total issues...")
	totalIssues, err := a.client.FetchTotalIssues(ctx, owner, repo)
	if err != nil || totalIssues == 0 {
		if err != nil {
			progress(fmt.Sprintf("Warning: could not fetch issue count: %v, falling back to open issues", err))
		}
		totalIssues = repoInfo.OpenIssues
	}
	result.Repo.TotalIssues = totalIssues

	// Step 3: Fetch stargazers with sampling
	progress(fmt.Sprintf("Fetching stargazers (%d stars)...", repoInfo.Stars))
	stargazers, err := a.client.FetchStargazers(ctx, owner, repo, repoInfo.Stars)
	if err != nil {
		return result, fmt.Errorf("fetch stargazers: %w", err)
	}

	if repoInfo.Stars > 0 {
		result.SampleRate = float64(len(stargazers)) / float64(repoInfo.Stars)
	}

	// Step 4: Profile stargazers
	progress(fmt.Sprintf("Profiling %d stargazers...", len(stargazers)))
	logins := make([]string, len(stargazers))
	starredAtMap := make(map[string]time.Time, len(stargazers))
	for i, s := range stargazers {
		logins[i] = s.Login
		starredAtMap[s.Login] = s.StarredAt
	}

	profiles, failCount := a.client.FetchUserProfiles(ctx, logins)

	if failCount > 0 {
		progress(fmt.Sprintf("Warning: %d/%d user profiles failed to fetch", failCount, len(logins)))
	}

	// Classify and attach StarredAt
	classified := make([]model.StargazerProfile, 0, len(profiles))
	for _, p := range profiles {
		p.StarredAt = starredAtMap[p.Login]
		p.Class = ClassifyStargazer(p)
		classified = append(classified, p)
	}

	// Step 5: Summarize
	progress("Calculating signals...")
	summary := SummarizeStargazers(classified, repoInfo.Stars)
	result.StargazerSummary = summary

	// Step 6: Calculate signals and score
	signals := CalculateSignals(repoInfo, summary)
	result.Signals = signals

	score, verdict := Score(signals)
	result.TrustScore = score
	result.Verdict = verdict

	progress("Analysis complete.")
	return result, nil
}
