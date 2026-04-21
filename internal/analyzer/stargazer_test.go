package analyzer

import (
	"testing"
	"time"

	"github.com/fmiskovic/softika/internal/model"
)

func TestClassifyStargazer(t *testing.T) {
	tests := []struct {
		name     string
		profile  model.StargazerProfile
		expected model.StargazerClass
	}{
		{
			name: "ghost: zero repos and zero followers",
			profile: model.StargazerProfile{
				PublicRepos: 0,
				Followers:   0,
				AccountAge:  90 * 24 * time.Hour,
			},
			expected: model.ClassGhost,
		},
		{
			name: "dormant: old account, no activity",
			profile: model.StargazerProfile{
				PublicRepos: 1,
				Followers:   0,
				Following:   0,
				AccountAge:  800 * 24 * time.Hour,
			},
			expected: model.ClassDormant,
		},
		{
			name: "low signal: few repos and followers",
			profile: model.StargazerProfile{
				PublicRepos: 2,
				Followers:   3,
				Following:   10,
				AccountAge:  200 * 24 * time.Hour,
			},
			expected: model.ClassLowSignal,
		},
		{
			name: "organic: meaningful activity",
			profile: model.StargazerProfile{
				PublicRepos: 15,
				Followers:   50,
				Following:   30,
				AccountAge:  1500 * 24 * time.Hour,
			},
			expected: model.ClassOrganic,
		},
		{
			name: "ghost takes priority over dormant",
			profile: model.StargazerProfile{
				PublicRepos: 0,
				Followers:   0,
				Following:   0,
				AccountAge:  2000 * 24 * time.Hour,
			},
			expected: model.ClassGhost,
		},
		{
			name: "not dormant if account too young",
			profile: model.StargazerProfile{
				PublicRepos: 1,
				Followers:   0,
				Following:   0,
				AccountAge:  100 * 24 * time.Hour,
			},
			expected: model.ClassLowSignal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyStargazer(tt.profile)
			if got != tt.expected {
				t.Errorf("ClassifyStargazer() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSummarizeStargazers(t *testing.T) {
	now := time.Now()
	profiles := []model.StargazerProfile{
		{Class: model.ClassGhost, Followers: 0, AccountAge: 100 * 24 * time.Hour, StarredAt: now.Add(-30 * 24 * time.Hour)},
		{Class: model.ClassGhost, Followers: 0, AccountAge: 200 * 24 * time.Hour, StarredAt: now.Add(-29 * 24 * time.Hour)},
		{Class: model.ClassDormant, Followers: 0, AccountAge: 500 * 24 * time.Hour, StarredAt: now.Add(-20 * 24 * time.Hour)},
		{Class: model.ClassLowSignal, Followers: 2, AccountAge: 300 * 24 * time.Hour, StarredAt: now.Add(-15 * 24 * time.Hour)},
		{Class: model.ClassOrganic, Followers: 50, AccountAge: 1500 * 24 * time.Hour, StarredAt: now.Add(-10 * 24 * time.Hour)},
	}

	summary := SummarizeStargazers(profiles, 10000)

	if summary.SampleSize != 5 {
		t.Errorf("SampleSize = %d, want 5", summary.SampleSize)
	}
	if summary.TotalStars != 10000 {
		t.Errorf("TotalStars = %d, want 10000", summary.TotalStars)
	}
	if summary.GhostCount != 2 {
		t.Errorf("GhostCount = %d, want 2", summary.GhostCount)
	}
	if summary.DormantCount != 1 {
		t.Errorf("DormantCount = %d, want 1", summary.DormantCount)
	}
	if summary.LowSignalCount != 1 {
		t.Errorf("LowSignalCount = %d, want 1", summary.LowSignalCount)
	}
	if summary.OrganicCount != 1 {
		t.Errorf("OrganicCount = %d, want 1", summary.OrganicCount)
	}
	if summary.ZeroFollowerCount != 3 {
		t.Errorf("ZeroFollowerCount = %d, want 3", summary.ZeroFollowerCount)
	}

	// Median of [100d, 200d, 300d, 500d, 1500d] = 300d
	expectedMedian := 300 * 24 * time.Hour
	if summary.MedianAccountAge != expectedMedian {
		t.Errorf("MedianAccountAge = %v, want %v", summary.MedianAccountAge, expectedMedian)
	}
}

func TestSummarizeStargazers_empty(t *testing.T) {
	summary := SummarizeStargazers(nil, 0)
	if summary.SampleSize != 0 {
		t.Errorf("SampleSize = %d, want 0", summary.SampleSize)
	}
}

func TestMedianDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    []time.Duration
		expected time.Duration
	}{
		{"empty", nil, 0},
		{"single", []time.Duration{5 * time.Hour}, 5 * time.Hour},
		{"odd count", []time.Duration{1, 3, 5}, 3},
		{"even count", []time.Duration{1, 3, 5, 7}, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := medianDuration(tt.input)
			if got != tt.expected {
				t.Errorf("medianDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}
