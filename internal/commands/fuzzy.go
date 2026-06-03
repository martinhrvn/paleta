package commands

import (
	"strings"
)

// FuzzyFilter filters items based on fuzzy matching with the search term
func FuzzyFilter(items []string, search string) []string {
	if search == "" {
		return items
	}

	var results []string
	searchLower := strings.ToLower(search)

	for _, item := range items {
		if fuzzyMatch(strings.ToLower(item), searchLower) {
			results = append(results, item)
		}
	}

	return results
}

// fuzzyMatch checks if all characters in the search term appear in order in the item
func fuzzyMatch(item, search string) bool {
	if search == "" {
		return true
	}

	searchIndex := 0
	searchLen := len(search)

	for i := 0; i < len(item) && searchIndex < searchLen; i++ {
		if item[i] == search[searchIndex] {
			searchIndex++
		}
	}

	return searchIndex == searchLen
}

// FuzzyScore calculates a score for how well an item matches the search term
// Lower scores are better matches
func FuzzyScore(item, search string) int {
	if search == "" {
		return 0
	}

	itemLower := strings.ToLower(item)
	searchLower := strings.ToLower(search)

	// Exact match gets best score
	if strings.Contains(itemLower, searchLower) {
		return 0
	}

	// Calculate score based on character distances
	score := 0
	searchIndex := 0
	lastIndex := -1
	searchLen := len(searchLower)

	for i := 0; i < len(itemLower) && searchIndex < searchLen; i++ {
		if itemLower[i] == searchLower[searchIndex] {
			if lastIndex != -1 {
				// Add distance between matched characters to score
				score += i - lastIndex - 1
			}
			lastIndex = i
			searchIndex++
		}
	}

	// If not all characters matched, return high score (bad match)
	if searchIndex != searchLen {
		return 999999
	}

	return score
}

// FuzzyFilterWithScores filters and sorts items by fuzzy match score
func FuzzyFilterWithScores(items []string, search string) []string {
	if search == "" {
		return items
	}

	type scoredItem struct {
		item  string
		score int
	}

	var scored []scoredItem
	for _, item := range items {
		score := FuzzyScore(item, search)
		if score < 999999 { // Only include items that matched
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// Sort by score (simple bubble sort for small lists)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score < scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Extract sorted items
	results := make([]string, len(scored))
	for i, s := range scored {
		results[i] = s.item
	}

	return results
}
