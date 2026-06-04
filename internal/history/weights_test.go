package history

import (
	"math"
	"testing"
	"time"
)

func TestNewWeights(t *testing.T) {
	tests := []struct {
		name         string
		freq, rec    float64
		wantF, wantR float64
	}{
		{"already normalized", 0.5, 0.5, 0.5, 0.5},
		{"50/50 scale normalizes", 50, 50, 0.5, 0.5},
		{"frequency heavy", 80, 20, 0.8, 0.2},
		{"recency heavy", 0.2, 0.8, 0.2, 0.8},
		{"zero falls back to default", 0, 0, 0.5, 0.5},
		{"negative falls back to default", -1, -2, 0.5, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWeights(tt.freq, tt.rec)
			if math.Abs(w.FrequencyWeight-tt.wantF) > 1e-9 || math.Abs(w.RecencyWeight-tt.wantR) > 1e-9 {
				t.Errorf("NewWeights(%v,%v) = {f:%v r:%v}, want {f:%v r:%v}",
					tt.freq, tt.rec, w.FrequencyWeight, w.RecencyWeight, tt.wantF, tt.wantR)
			}
		})
	}
}

// TestScoreEntryRespectsWeights verifies the configured weights actually steer the
// ranking: a frequently-but-long-ago command beats a just-used-once command under
// frequency-heavy weights, and the reverse under recency-heavy weights.
func TestScoreEntryRespectsWeights(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	frequent := CommandEntry{Count: 100, LastAccess: now.Add(-10 * 24 * time.Hour)}
	recent := CommandEntry{Count: 1, LastAccess: now}

	freqHeavy := &History{weights: NewWeights(0.95, 0.05)}
	if freqHeavy.ScoreEntry(frequent, now) <= freqHeavy.ScoreEntry(recent, now) {
		t.Error("frequency-heavy weights: frequent command should outscore the recent one")
	}

	recHeavy := &History{weights: NewWeights(0.05, 0.95)}
	if recHeavy.ScoreEntry(recent, now) <= recHeavy.ScoreEntry(frequent, now) {
		t.Error("recency-heavy weights: recent command should outscore the frequent one")
	}
}

// TestScoreEntryDefaultsWhenUnset confirms a zero-value weights field falls back to
// the balanced default rather than scoring everything zero.
func TestScoreEntryDefaultsWhenUnset(t *testing.T) {
	now := time.Now()
	h := &History{} // no weights set
	entry := CommandEntry{Count: 5, LastAccess: now}
	if h.ScoreEntry(entry, now) <= 0 {
		t.Error("expected a positive score using default weights when none configured")
	}
}
