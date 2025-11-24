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

// calculateFrecencyScore computes a score based on frequency and recency
// Uses a 50/50 balanced approach:
//   - Frequency: logarithmic scale to prevent very frequent commands from dominating
//   - Recency: inverse decay based on days since last access
func calculateFrecencyScore(entry CommandEntry, now time.Time) float64 {
	return calculateFrecencyScoreWithWeights(entry, now, DefaultWeights)
}

// calculateFrecencyScoreWithWeights allows custom weight configuration
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
