package analyzer

import (
	"math"
	"testing"
	"time"

	"github.com/fmiskovic/softika/internal/model"
)

func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestScoreHigherIsBetter(t *testing.T) {
	r := signalRange{organicLow: 0.09, organicHigh: 0.235, suspicious: 0.05}

	tests := []struct {
		name  string
		raw   float64
		want  float64
		desc  string
	}{
		{"at suspicious threshold", 0.05, 0, "should be 0 at suspicious"},
		{"below suspicious", 0.02, 0, "should be 0 below suspicious"},
		{"midpoint between suspicious and organic", 0.07, 40, "should be ~40"},
		{"at organic low", 0.09, 80, "should be 80 at organic low"},
		{"at organic high", 0.235, 100, "should be 100 at organic high"},
		{"above organic high", 0.5, 100, "should be capped at 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreHigherIsBetter(tt.raw, r)
			if !almostEqual(got, tt.want, 1.0) {
				t.Errorf("scoreHigherIsBetter(%f) = %f, want ~%f (%s)", tt.raw, got, tt.want, tt.desc)
			}
		})
	}
}

func TestScoreLowerIsBetter(t *testing.T) {
	r := signalRange{organicLow: 0.01, organicHigh: 0.06, suspicious: 0.20}

	tests := []struct {
		name string
		raw  float64
		want float64
	}{
		{"below organic low (best)", 0.005, 100},
		{"at organic low", 0.01, 100},
		{"at organic high", 0.06, 80},
		{"at suspicious", 0.20, 0},
		{"above suspicious", 0.30, 0},
		{"midpoint organic range", 0.035, 90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreLowerIsBetter(tt.raw, r)
			if !almostEqual(got, tt.want, 1.5) {
				t.Errorf("scoreLowerIsBetter(%f) = %f, want ~%f", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCalcForkRatio(t *testing.T) {
	// Organic repo: 10k stars, 1.5k forks = ratio 0.15
	s := calcForkRatio(1500, 10000)
	if s.Name != "fork_to_star_ratio" {
		t.Errorf("name = %q", s.Name)
	}
	if !almostEqual(s.RawValue, 0.15, 0.001) {
		t.Errorf("raw = %f, want ~0.15", s.RawValue)
	}
	if s.Score < 80 {
		t.Errorf("score = %f, expected >= 80 for organic ratio", s.Score)
	}

	// Suspicious: 50k stars, 500 forks = ratio 0.01
	s = calcForkRatio(500, 50000)
	if s.Score > 20 {
		t.Errorf("score = %f, expected < 20 for suspicious ratio", s.Score)
	}
}

func TestCalcGhostPct(t *testing.T) {
	// Organic: 3% ghost
	s := calcGhostPct(15, 500)
	if s.Score < 80 {
		t.Errorf("score = %f, expected >= 80 for 3%% ghost", s.Score)
	}

	// Suspicious: 30% ghost
	s = calcGhostPct(150, 500)
	if s.Score > 10 {
		t.Errorf("score = %f, expected < 10 for 30%% ghost", s.Score)
	}
}

func TestCalcStarVelocity_noData(t *testing.T) {
	s := calcStarVelocity(nil)
	if s.Score != 50 {
		t.Errorf("score = %f, want 50 for no data", s.Score)
	}
}

func TestCalcStarVelocity_organic(t *testing.T) {
	// Steady growth: all weeks ~10 stars
	timeline := make([]model.StarVelocityPoint, 20)
	for i := range timeline {
		timeline[i] = model.StarVelocityPoint{
			Date:  time.Now().Add(time.Duration(-i) * 7 * 24 * time.Hour),
			Count: 10 + (i % 3), // 10, 11, 12 - steady
		}
	}

	s := calcStarVelocity(timeline)
	if s.Score < 80 {
		t.Errorf("score = %f, expected >= 80 for steady growth", s.Score)
	}
}

func TestCalcStarVelocity_suspicious(t *testing.T) {
	// Most weeks 5 stars, then 5 huge spikes of 500
	timeline := make([]model.StarVelocityPoint, 20)
	for i := range timeline {
		count := 5
		if i < 5 {
			count = 500
		}
		timeline[i] = model.StarVelocityPoint{
			Date:  time.Now().Add(time.Duration(-i) * 7 * 24 * time.Hour),
			Count: count,
		}
	}

	s := calcStarVelocity(timeline)
	if s.Score > 30 {
		t.Errorf("score = %f, expected < 30 for spiky growth", s.Score)
	}
}

func TestCalculateSignals_returns7(t *testing.T) {
	repo := model.RepoInfo{Stars: 1000, Forks: 150, Watchers: 20, TotalIssues: 50}
	summary := model.StargazerSummary{
		SampleSize:        100,
		TotalStars:        1000,
		GhostCount:        5,
		ZeroFollowerCount: 15,
		MedianAccountAge:  3000 * 24 * time.Hour,
	}

	signals := CalculateSignals(repo, summary)
	if len(signals) != 7 {
		t.Errorf("got %d signals, want 7", len(signals))
	}

	totalWeight := 0.0
	for _, s := range signals {
		totalWeight += s.Weight
		if s.Score < 0 || s.Score > 100 {
			t.Errorf("signal %q score %f out of [0,100]", s.Name, s.Score)
		}
	}

	if !almostEqual(totalWeight, 1.0, 0.001) {
		t.Errorf("total weight = %f, want 1.0", totalWeight)
	}
}

func TestCalculateSignals_organicRepo(t *testing.T) {
	repo := model.RepoInfo{Stars: 10000, Forks: 1500, Watchers: 200, TotalIssues: 500}
	summary := model.StargazerSummary{
		SampleSize:        500,
		TotalStars:        10000,
		GhostCount:        20,  // 4%
		ZeroFollowerCount: 75,  // 15%
		MedianAccountAge:  3500 * 24 * time.Hour,
	}

	signals := CalculateSignals(repo, summary)
	for _, s := range signals {
		// star_velocity gets 50 (neutral) with no timeline — that's expected
		if s.Name == "star_velocity" {
			continue
		}
		if s.Score < 60 {
			t.Errorf("organic repo: signal %q score too low: %f", s.Name, s.Score)
		}
	}
}

func TestCalculateSignals_suspiciousRepo(t *testing.T) {
	repo := model.RepoInfo{Stars: 50000, Forks: 500, Watchers: 30, TotalIssues: 20}
	summary := model.StargazerSummary{
		SampleSize:        500,
		TotalStars:        50000,
		GhostCount:        140,  // 28%
		ZeroFollowerCount: 300,  // 60%
		MedianAccountAge:  800 * 24 * time.Hour,
	}

	signals := CalculateSignals(repo, summary)
	lowScoreCount := 0
	for _, s := range signals {
		if s.Score < 30 {
			lowScoreCount++
		}
	}
	if lowScoreCount < 3 {
		t.Errorf("suspicious repo: expected >= 3 low-score signals, got %d", lowScoreCount)
	}
}
