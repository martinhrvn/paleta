package history

import (
	"math"
	"time"
)

// FrecencyWeights defines the balance between frequency and recency
type FrecencyWeights struct {
	FrequencyWeight float64
	RecencyWeight   float64
}

// DefaultWeights provides a 50/50 balance
var DefaultWeights = FrecencyWeights{
	FrequencyWeight: 0.5,
	RecencyWeight:   0.5,
}

// NewWeights builds normalized weights from raw frequency/recency values so that
// only their ratio matters — 50/50 and 0.5/0.5 behave identically. A non-positive
// total falls back to DefaultWeights.
func NewWeights(frequency, recency float64) FrecencyWeights {
	total := frequency + recency
	if total <= 0 {
		return DefaultWeights
	}
	return FrecencyWeights{
		FrequencyWeight: frequency / total,
		RecencyWeight:   recency / total,
	}
}

// effectiveWeights returns w, or DefaultWeights when w is the zero value (e.g. a
// History constructed without weights).
func effectiveWeights(w FrecencyWeights) FrecencyWeights {
	if w.FrequencyWeight == 0 && w.RecencyWeight == 0 {
		return DefaultWeights
	}
	return w
}

// calculateFrecencyScoreWithWeights computes a score based on frequency and
// recency:
//   - Frequency: logarithmic scale to prevent very frequent commands from dominating
//   - Recency: inverse decay based on days since last access
func calculateFrecencyScoreWithWeights(entry CommandEntry, now time.Time, weights FrecencyWeights) float64 {
	// Frequency score: logarithmic to prevent runaway scaling
	// log(count + 1) normalized to 0-100 scale
	frequencyScore := math.Log(float64(entry.Count+1)) * 100

	// Recency score: inverse decay over time
	// 100 / (1 + days_since_access)
	daysSinceAccess := now.Sub(entry.LastAccess).Hours() / 24
	recencyScore := 100.0 / (1.0 + daysSinceAccess)

	// Combine with configured weights
	finalScore := (frequencyScore * weights.FrequencyWeight) +
		(recencyScore * weights.RecencyWeight)

	return finalScore
}

// CalculateScore is a public API for testing different weight configurations
func CalculateScore(count int, lastAccess time.Time, now time.Time, weights FrecencyWeights) float64 {
	entry := CommandEntry{
		Count:      count,
		LastAccess: lastAccess,
	}
	return calculateFrecencyScoreWithWeights(entry, now, weights)
}
