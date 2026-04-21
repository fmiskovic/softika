package model

import "time"

// StargazerClass categorizes a stargazer by engagement quality.
type StargazerClass string

const (
	ClassGhost     StargazerClass = "ghost"      // 0 repos AND 0 followers
	ClassDormant   StargazerClass = "dormant"    // account age >1yr, no recent activity
	ClassLowSignal StargazerClass = "low_signal" // <3 repos, <5 followers
	ClassOrganic   StargazerClass = "organic"    // meaningful activity
)

// Verdict summarizes the trust assessment.
type Verdict string

const (
	VerdictHighRisk      Verdict = "high_risk"
	VerdictModerate      Verdict = "moderate"
	VerdictLikelyOrganic Verdict = "likely_organic"
)

// RepoInfo holds basic repository statistics fetched from GitHub.
type RepoInfo struct {
	Owner           string    `json:"owner"`
	Name            string    `json:"name"`
	Stars           int       `json:"stars"`
	Forks           int       `json:"forks"`
	Watchers        int       `json:"watchers"` // subscribers_count (true watchers)
	OpenIssues      int       `json:"open_issues"`
	CreatedAt       time.Time `json:"created_at"`
	Description     string    `json:"description,omitempty"`
	DefaultBranch   string    `json:"default_branch,omitempty"`
	TotalIssues int `json:"total_issues"` // open + closed via search
}

// StargazerProfile holds data about an individual stargazer.
type StargazerProfile struct {
	Login       string         `json:"login"`
	AccountAge  time.Duration  `json:"account_age"`
	CreatedAt   time.Time      `json:"created_at"`
	PublicRepos int            `json:"public_repos"`
	Followers   int            `json:"followers"`
	Following   int            `json:"following"`
	StarredAt   time.Time      `json:"starred_at"`
	Class       StargazerClass `json:"class"`
}

// Signal represents a single analysis metric with its raw value and normalized score.
type Signal struct {
	Name        string  `json:"name"`
	RawValue    float64 `json:"raw_value"`
	Score       float64 `json:"score"`        // 0-100, higher = more organic
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// StarVelocityPoint represents star count at a point in time (for spike detection).
type StarVelocityPoint struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

// StargazerSummary aggregates stargazer profiling results.
type StargazerSummary struct {
	SampleSize       int            `json:"sample_size"`
	TotalStars       int            `json:"total_stars"`
	GhostCount       int            `json:"ghost_count"`
	DormantCount     int            `json:"dormant_count"`
	LowSignalCount   int            `json:"low_signal_count"`
	OrganicCount     int            `json:"organic_count"`
	ZeroFollowerCount int           `json:"zero_follower_count"`
	MedianAccountAge time.Duration  `json:"median_account_age"`
	AccountAges      []time.Duration `json:"-"` // raw ages for stats
	StarTimeline     []StarVelocityPoint `json:"star_timeline,omitempty"`
}

// AnalysisResult is the complete output of a repo analysis.
type AnalysisResult struct {
	Repo             RepoInfo         `json:"repo"`
	StargazerSummary StargazerSummary `json:"stargazer_summary"`
	Signals          []Signal         `json:"signals"`
	TrustScore       float64          `json:"trust_score"` // 0-100
	Verdict          Verdict          `json:"verdict"`
	AnalyzedAt       time.Time        `json:"analyzed_at"`
	SampleRate       float64          `json:"sample_rate"` // fraction of stars sampled
}
