package analyzer

import (
	"fmt"
	"sort"
	"time"

	"github.com/fmiskovic/softika/internal/model"
)

const (
	dormantThresholdDays = 365
	lowSignalRepoMax     = 3
	lowSignalFollowerMax = 5
)

// ClassifyStargazer assigns a StargazerClass based on the profile.
func ClassifyStargazer(p model.StargazerProfile) model.StargazerClass {
	if p.PublicRepos == 0 && p.Followers == 0 {
		return model.ClassGhost
	}

	if p.AccountAge > dormantThresholdDays*24*time.Hour &&
		p.PublicRepos <= 1 && p.Followers == 0 && p.Following <= 1 {
		return model.ClassDormant
	}

	if p.PublicRepos < lowSignalRepoMax && p.Followers < lowSignalFollowerMax {
		return model.ClassLowSignal
	}

	return model.ClassOrganic
}

// SummarizeStargazers builds a StargazerSummary from classified profiles.
func SummarizeStargazers(profiles []model.StargazerProfile, totalStars int) model.StargazerSummary {
	summary := model.StargazerSummary{
		SampleSize: len(profiles),
		TotalStars: totalStars,
	}

	if len(profiles) == 0 {
		return summary
	}

	ages := make([]time.Duration, 0, len(profiles))

	for _, p := range profiles {
		ages = append(ages, p.AccountAge)

		if p.Followers == 0 {
			summary.ZeroFollowerCount++
		}

		switch p.Class {
		case model.ClassGhost:
			summary.GhostCount++
		case model.ClassDormant:
			summary.DormantCount++
		case model.ClassLowSignal:
			summary.LowSignalCount++
		case model.ClassOrganic:
			summary.OrganicCount++
		}
	}

	summary.AccountAges = ages
	summary.MedianAccountAge = medianDuration(ages)

	summary.StarTimeline = buildTimeline(profiles)

	return summary
}

// medianDuration returns the median of a slice of durations.
func medianDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// buildTimeline groups star events by week and returns a timeline.
func buildTimeline(profiles []model.StargazerProfile) []model.StarVelocityPoint {
	if len(profiles) == 0 {
		return nil
	}

	// Filter profiles with valid StarredAt
	type entry struct {
		starredAt time.Time
	}
	var entries []entry
	for _, p := range profiles {
		if !p.StarredAt.IsZero() {
			entries = append(entries, entry{starredAt: p.StarredAt})
		}
	}

	if len(entries) == 0 {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].starredAt.Before(entries[j].starredAt)
	})

	// Group by ISO week
	weekCounts := make(map[string]int)
	for _, e := range entries {
		year, week := e.starredAt.ISOWeek()
		key := weekKey(year, week)
		weekCounts[key]++
	}

	// Convert to sorted timeline
	keys := make([]string, 0, len(weekCounts))
	for k := range weekCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	timeline := make([]model.StarVelocityPoint, 0, len(keys))
	for _, k := range keys {
		// Parse back to approximate date (Monday of that ISO week)
		var year, week int
		_, _ = parseWeekKey(k, &year, &week)
		t := isoWeekStart(year, week)
		timeline = append(timeline, model.StarVelocityPoint{
			Date:  t,
			Count: weekCounts[k],
		})
	}

	return timeline
}

func weekKey(year, week int) string {
	return fmt.Sprintf("%04d-W%02d", year, week)
}

func parseWeekKey(key string, year, week *int) (int, error) {
	n, err := fmt.Sscanf(key, "%04d-W%02d", year, week)
	return n, err
}

// isoWeekStart returns the Monday of the given ISO week.
func isoWeekStart(year, week int) time.Time {
	// Jan 4 is always in week 1
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	weekday := jan4.Weekday()
	if weekday == 0 {
		weekday = 7
	}
	// Monday of week 1
	mondayW1 := jan4.AddDate(0, 0, -int(weekday)+1)
	return mondayW1.AddDate(0, 0, (week-1)*7)
}
