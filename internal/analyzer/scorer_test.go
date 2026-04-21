package analyzer

import (
	"testing"

	"github.com/fmiskovic/softika/internal/model"
)

func TestScore_empty(t *testing.T) {
	score, verdict := Score(nil)
	if score != 0 {
		t.Errorf("score = %f, want 0", score)
	}
	if verdict != model.VerdictHighRisk {
		t.Errorf("verdict = %q, want high_risk", verdict)
	}
}

func TestScore_allOrganic(t *testing.T) {
	signals := []model.Signal{
		{Name: "fork_ratio", Score: 90, Weight: 0.25},
		{Name: "ghost_pct", Score: 95, Weight: 0.20},
		{Name: "velocity", Score: 100, Weight: 0.15},
		{Name: "watcher_ratio", Score: 85, Weight: 0.10},
		{Name: "account_age", Score: 92, Weight: 0.10},
		{Name: "zero_follower", Score: 88, Weight: 0.10},
		{Name: "issue_ratio", Score: 90, Weight: 0.10},
	}

	score, verdict := Score(signals)
	if score < 70 {
		t.Errorf("score = %f, expected >= 70 for organic signals", score)
	}
	if verdict != model.VerdictLikelyOrganic {
		t.Errorf("verdict = %q, want likely_organic", verdict)
	}
}

func TestScore_allSuspicious(t *testing.T) {
	signals := []model.Signal{
		{Name: "fork_ratio", Score: 5, Weight: 0.25},
		{Name: "ghost_pct", Score: 0, Weight: 0.20},
		{Name: "velocity", Score: 10, Weight: 0.15},
		{Name: "watcher_ratio", Score: 0, Weight: 0.10},
		{Name: "account_age", Score: 15, Weight: 0.10},
		{Name: "zero_follower", Score: 0, Weight: 0.10},
		{Name: "issue_ratio", Score: 5, Weight: 0.10},
	}

	score, verdict := Score(signals)
	if score >= 40 {
		t.Errorf("score = %f, expected < 40 for suspicious signals", score)
	}
	if verdict != model.VerdictHighRisk {
		t.Errorf("verdict = %q, want high_risk", verdict)
	}
}

func TestScore_moderate(t *testing.T) {
	signals := []model.Signal{
		{Name: "fork_ratio", Score: 60, Weight: 0.25},
		{Name: "ghost_pct", Score: 50, Weight: 0.20},
		{Name: "velocity", Score: 50, Weight: 0.15},
		{Name: "watcher_ratio", Score: 55, Weight: 0.10},
		{Name: "account_age", Score: 60, Weight: 0.10},
		{Name: "zero_follower", Score: 45, Weight: 0.10},
		{Name: "issue_ratio", Score: 55, Weight: 0.10},
	}

	score, verdict := Score(signals)
	if score < 40 || score >= 70 {
		t.Errorf("score = %f, expected 40-70 for moderate signals", score)
	}
	if verdict != model.VerdictModerate {
		t.Errorf("verdict = %q, want moderate", verdict)
	}
}

func TestScore_weightsAreRespected(t *testing.T) {
	// One very heavy signal at 0, rest at 100
	signals := []model.Signal{
		{Name: "heavy", Score: 0, Weight: 0.90},
		{Name: "light", Score: 100, Weight: 0.10},
	}

	score, _ := Score(signals)
	if score > 15 {
		t.Errorf("score = %f, expected < 15 when heavy signal is 0", score)
	}
}
