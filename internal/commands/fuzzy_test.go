package commands

import (
	"testing"
)

func TestFuzzySelect(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		search   string
		expected []string
	}{
		{
			name: "exact match",
			items: []string{
				"[frontend] start",
				"[frontend] build",
				"[backend] run",
			},
			search: "start",
			expected: []string{
				"[frontend] start",
			},
		},
		{
			name: "fuzzy match",
			items: []string{
				"[frontend] start",
				"[frontend] build",
				"[backend] test",
			},
			search: "frst",
			expected: []string{
				"[frontend] start",
			},
		},
		{
			name: "multiple matches",
			items: []string{
				"[frontend] test",
				"[backend] test",
				"[scripts] deploy",
			},
			search: "test",
			expected: []string{
				"[frontend] test",
				"[backend] test",
			},
		},
		{
			name: "no matches",
			items: []string{
				"[frontend] start",
				"[backend] run",
			},
			search:   "xyz",
			expected: []string{},
		},
		{
			name: "empty search",
			items: []string{
				"[frontend] start",
				"[backend] run",
			},
			search: "",
			expected: []string{
				"[frontend] start",
				"[backend] run",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := FuzzyFilter(tt.items, tt.search)

			if len(results) != len(tt.expected) {
				t.Errorf("Expected %d results, got %d", len(tt.expected), len(results))
				return
			}

			for i, expected := range tt.expected {
				if results[i] != expected {
					t.Errorf("Result[%d] = %q, expected %q", i, results[i], expected)
				}
			}
		})
	}
}
