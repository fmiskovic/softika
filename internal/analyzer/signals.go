package analyzer

import (
	"fmt"
	"math"
	"time"

	"github.com/fmiskovic/softika/internal/model"
)

// Signal weight constants — must sum to 1.0.
const (
	WeightForkRatio      = 0.25
	WeightGhostPct       = 0.20
	WeightStarVelocity   = 0.15
	WeightWatcherRatio   = 0.10
	WeightAccountAge     = 0.10
	WeightZeroFollower   = 0.10
	WeightIssueRatio     = 0.10
)

// Organic/suspicious range anchors (from CMU study).
type signalRange struct {
	organicLow  float64 // healthy lower bound
	organicHigh float64 // healthy upper bound
	suspicious  float64 // clearly bad threshold
}

var (
	forkRatioRange      = signalRange{organicLow: 0.09, organicHigh: 0.235, suspicious: 0.05}
	watcherRatioRange   = signalRange{organicLow: 0.005, organicHigh: 0.03, suspicious: 0.001}
	ghostPctRange       = signalRange{organicLow: 0.01, organicHigh: 0.06, suspicious: 0.20}
	zeroFollowerRange   = signalRange{organicLow: 0.10, organicHigh: 0.20, suspicious: 0.50}
	accountAgeDaysRange = signalRange{organicLow: 2500, organicHigh: 4800, suspicious: 1000}
	issueRatioRange     = signalRange{organicLow: 0.01, organicHigh: 0.10, suspicious: 0.002}
)

// CalculateSignals computes all 7 signals from repo info and stargazer summary.
func CalculateSignals(repo model.RepoInfo, summary model.StargazerSummary) []model.Signal {
	stars := float64(repo.Stars)
	if stars == 0 {
		stars = 1 // avoid division by zero
	}

	sampleSize := float64(summary.SampleSize)
	if sampleSize == 0 {
		sampleSize = 1
	}

	return []model.Signal{
		calcForkRatio(float64(repo.Forks), stars),
		calcGhostPct(float64(summary.GhostCount), sampleSize),
		calcStarVelocity(summary.StarTimeline),
		calcWatcherRatio(float64(repo.Watchers), stars),
		calcAccountAge(summary.MedianAccountAge),
		calcZeroFollowerPct(float64(summary.ZeroFollowerCount), sampleSize),
		calcIssueRatio(float64(repo.TotalIssues), stars),
	}
}

func calcForkRatio(forks, stars float64) model.Signal {
	raw := forks / stars
	return model.Signal{
		Name:        "fork_to_star_ratio",
		RawValue:    raw,
		Score:       scoreHigherIsBetter(raw, forkRatioRange),
		Weight:      WeightForkRatio,
		Description: "Ratio of forks to stars (organic: 0.09-0.23, suspicious: <0.05)",
	}
}

func calcWatcherRatio(watchers, stars float64) model.Signal {
	raw := watchers / stars
	return model.Signal{
		Name:        "watcher_to_star_ratio",
		RawValue:    raw,
		Score:       scoreHigherIsBetter(raw, watcherRatioRange),
		Weight:      WeightWatcherRatio,
		Description: "Ratio of watchers to stars (organic: 0.005-0.03, suspicious: <0.001)",
	}
}

func calcGhostPct(ghosts, sampleSize float64) model.Signal {
	raw := ghosts / sampleSize
	return model.Signal{
		Name:        "ghost_account_pct",
		RawValue:    raw,
		Score:       scoreLowerIsBetter(raw, ghostPctRange),
		Weight:      WeightGhostPct,
		Description: "Percentage of ghost accounts (organic: 1-6%, suspicious: >20%)",
	}
}

func calcZeroFollowerPct(zeroFollower, sampleSize float64) model.Signal {
	raw := zeroFollower / sampleSize
	return model.Signal{
		Name:        "zero_follower_pct",
		RawValue:    raw,
		Score:       scoreLowerIsBetter(raw, zeroFollowerRange),
		Weight:      WeightZeroFollower,
		Description: "Percentage of zero-follower accounts (organic: 10-20%, suspicious: >50%)",
	}
}

func calcAccountAge(medianAge time.Duration) model.Signal {
	raw := medianAge.Hours() / 24 // days
	return model.Signal{
		Name:        "median_account_age_days",
		RawValue:    raw,
		Score:       scoreHigherIsBetter(raw, accountAgeDaysRange),
		Weight:      WeightAccountAge,
		Description: "Median stargazer account age in days (organic: >2500, suspicious: <1000)",
	}
}

func calcIssueRatio(issues, stars float64) model.Signal {
	raw := issues / stars
	return model.Signal{
		Name:        "issue_to_star_ratio",
		RawValue:    raw,
		Score:       scoreHigherIsBetter(raw, issueRatioRange),
		Weight:      WeightIssueRatio,
		Description: "Ratio of total issues to stars (organic: >0.01, suspicious: <0.002)",
	}
}

// calcStarVelocity detects spikes in the star timeline.
// A spike is a week with >3x the median weekly star count.
// More spikes = more suspicious.
func calcStarVelocity(timeline []model.StarVelocityPoint) model.Signal {
	if len(timeline) < 4 {
		return model.Signal{
			Name:        "star_velocity",
			RawValue:    0,
			Score:       50, // insufficient data, neutral
			Weight:      WeightStarVelocity,
			Description: "Star velocity spike detection (insufficient data)",
		}
	}

	counts := make([]float64, len(timeline))
	for i, p := range timeline {
		counts[i] = float64(p.Count)
	}

	median := medianFloat64(counts)
	if median == 0 {
		median = 1
	}

	spikeThreshold := median * 3
	spikeCount := 0
	for _, c := range counts {
		if c > spikeThreshold {
			spikeCount++
		}
	}

	spikeRatio := float64(spikeCount) / float64(len(counts))

	// 0% spikes = 100 score, >20% spikes = 0 score
	score := clamp(100-spikeRatio*500, 0, 100)

	return model.Signal{
		Name:        "star_velocity",
		RawValue:    spikeRatio,
		Score:       score,
		Weight:      WeightStarVelocity,
		Description: fmt.Sprintf("Star velocity: %.0f%% of weeks are spikes (>3x median)", spikeRatio*100),
	}
}

// scoreHigherIsBetter maps a raw value to 0-100 where higher raw = better score.
// Below suspicious threshold = 0, within organic range = 80-100, above = 100.
func scoreHigherIsBetter(raw float64, r signalRange) float64 {
	if raw >= r.organicLow {
		// Within or above organic range
		if raw >= r.organicHigh {
			return 100
		}
		return 80 + 20*(raw-r.organicLow)/(r.organicHigh-r.organicLow)
	}

	if raw <= r.suspicious {
		return 0
	}

	// Between suspicious and organic low: linear 0 -> 80
	return 80 * (raw - r.suspicious) / (r.organicLow - r.suspicious)
}

// scoreLowerIsBetter maps a raw value to 0-100 where lower raw = better score.
// Below organic low = 100, within organic range = 80-100, above suspicious = 0.
func scoreLowerIsBetter(raw float64, r signalRange) float64 {
	if raw <= r.organicLow {
		return 100
	}

	if raw <= r.organicHigh {
		return 100 - 20*(raw-r.organicLow)/(r.organicHigh-r.organicLow)
	}

	if raw >= r.suspicious {
		return 0
	}

	// Between organic high and suspicious: linear 80 -> 0
	return 80 * (r.suspicious - raw) / (r.suspicious - r.organicHigh)
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sortFloat64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func sortFloat64s(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
