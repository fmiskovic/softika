package analyzer

import (
	"context"
	"errors"
	"testing"
	"time"

	ghclient "github.com/fmiskovic/softika/internal/github"
	"github.com/fmiskovic/softika/internal/model"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"google/go-github", "google", "go-github", false},
		{"https://github.com/google/go-github", "google", "go-github", false},
		{"http://github.com/google/go-github", "google", "go-github", false},
		{"github.com/google/go-github", "google", "go-github", false},
		{"https://github.com/google/go-github/", "google", "go-github", false},
		{"google/go-github/tree/main", "google", "go-github", false},
		{"invalid", "", "", true},
		{"", "", "", true},
		{"/repo", "", "", true},
		{"owner/", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, err := ParseRepo(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepo(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

// mockClient implements GitHubClient for testing.
type mockClient struct {
	repoInfo    model.RepoInfo
	repoErr     error
	totalIssues int
	issuesErr   error
	stargazers  []ghclient.StargazerEntry
	starsErr    error
	profiles    []model.StargazerProfile
	failCount   int
}

func (m *mockClient) FetchRepoInfo(_ context.Context, _, _ string) (model.RepoInfo, error) {
	return m.repoInfo, m.repoErr
}

func (m *mockClient) FetchTotalIssues(_ context.Context, _, _ string) (int, error) {
	return m.totalIssues, m.issuesErr
}

func (m *mockClient) FetchStargazers(_ context.Context, _, _ string, _ int) ([]ghclient.StargazerEntry, error) {
	return m.stargazers, m.starsErr
}

func (m *mockClient) FetchUserProfiles(_ context.Context, _ []string) ([]model.StargazerProfile, int) {
	return m.profiles, m.failCount
}

func TestAnalyze_organicRepo(t *testing.T) {
	now := time.Now()
	mock := &mockClient{
		repoInfo: model.RepoInfo{
			Owner: "test", Name: "repo",
			Stars: 1000, Forks: 150, Watchers: 20, OpenIssues: 50,
		},
		totalIssues: 200,
		stargazers: []ghclient.StargazerEntry{
			{Login: "user1", StarredAt: now.Add(-30 * 24 * time.Hour)},
			{Login: "user2", StarredAt: now.Add(-20 * 24 * time.Hour)},
		},
		profiles: []model.StargazerProfile{
			{Login: "user1", PublicRepos: 20, Followers: 50, AccountAge: 3000 * 24 * time.Hour},
			{Login: "user2", PublicRepos: 15, Followers: 30, AccountAge: 2800 * 24 * time.Hour},
		},
	}

	a := NewRepoAnalyzer(mock)
	result, err := a.Analyze(context.Background(), "test", "repo", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TrustScore == 0 {
		t.Error("expected non-zero trust score")
	}
	if result.Verdict == "" {
		t.Error("expected a verdict")
	}
	if len(result.Signals) != 7 {
		t.Errorf("got %d signals, want 7", len(result.Signals))
	}
	if result.Repo.TotalIssues != 200 {
		t.Errorf("TotalIssues = %d, want 200", result.Repo.TotalIssues)
	}
}

func TestAnalyze_repoInfoError(t *testing.T) {
	mock := &mockClient{
		repoErr: errors.New("not found"),
	}

	a := NewRepoAnalyzer(mock)
	_, err := a.Analyze(context.Background(), "bad", "repo", nil)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAnalyze_stargazerError(t *testing.T) {
	mock := &mockClient{
		repoInfo: model.RepoInfo{Stars: 100},
		starsErr: errors.New("rate limited"),
	}

	a := NewRepoAnalyzer(mock)
	_, err := a.Analyze(context.Background(), "test", "repo", nil)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAnalyze_issuesFallback(t *testing.T) {
	mock := &mockClient{
		repoInfo:  model.RepoInfo{Stars: 100, OpenIssues: 42},
		issuesErr: errors.New("search failed"),
		profiles:  []model.StargazerProfile{},
	}

	a := NewRepoAnalyzer(mock)
	result, err := a.Analyze(context.Background(), "test", "repo", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Repo.TotalIssues != 42 {
		t.Errorf("TotalIssues = %d, want 42 (fallback to OpenIssues)", result.Repo.TotalIssues)
	}
}

func TestAnalyze_progressCallback(t *testing.T) {
	mock := &mockClient{
		repoInfo: model.RepoInfo{Stars: 10},
		profiles: []model.StargazerProfile{},
	}

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	a := NewRepoAnalyzer(mock)
	_, err := a.Analyze(context.Background(), "test", "repo", progress)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) == 0 {
		t.Error("expected progress messages, got none")
	}
}
