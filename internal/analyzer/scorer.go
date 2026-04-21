package analyzer

import (
	"math"

	"github.com/fmiskovic/softika/internal/model"
)

// Score computes the weighted composite trust score from signals and assigns a verdict.
func Score(signals []model.Signal) (float64, model.Verdict) {
	if len(signals) == 0 {
		return 0, model.VerdictHighRisk
	}

	weighted := 0.0
	totalWeight := 0.0

	for _, s := range signals {
		weighted += s.Score * s.Weight
		totalWeight += s.Weight
	}

	if totalWeight == 0 {
		return 0, model.VerdictHighRisk
	}

	score := math.Round(weighted/totalWeight*10) / 10 // one decimal place

	return score, verdictFromScore(score)
}

func verdictFromScore(score float64) model.Verdict {
	switch {
	case score < 40:
		return model.VerdictHighRisk
	case score < 70:
		return model.VerdictModerate
	default:
		return model.VerdictLikelyOrganic
	}
}
