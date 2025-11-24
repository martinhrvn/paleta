package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ExpandGlobPatterns(locations []Location) ([]Location, error) {
	var result []Location
	
	for _, loc := range locations {
		// Check if location contains glob pattern
		if strings.Contains(loc.Location, "*") {
			// Validate glob pattern before expansion
			if err := validateGlobPattern(loc.Location); err != nil {
				return nil, err
			}
			
			expanded, err := expandSingleGlob(loc)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded...)
		} else {
			// No glob pattern, add as-is
			result = append(result, loc)
		}
	}
	
	return result, nil
}

func validateGlobPattern(pattern string) error {
	// Count asterisks
	asteriskCount := strings.Count(pattern, "*")
	if asteriskCount > 1 {
		return fmt.Errorf("invalid glob pattern %q: multiple asterisks not supported, only simple patterns like 'path/*' are allowed", pattern)
	}
	
	// Check if asterisk is at the end of the pattern
	if !strings.HasSuffix(pattern, "*") {
		return fmt.Errorf("invalid glob pattern %q: asterisk must be at the end of the pattern", pattern)
	}
	
	// Check if asterisk is preceded by a slash
	if !strings.HasSuffix(pattern, "/*") {
		return fmt.Errorf("invalid glob pattern %q: asterisk must be preceded by a slash (e.g., 'path/*')", pattern)
	}
	
	return nil
}

func expandSingleGlob(loc Location) ([]Location, error) {
	matches, err := filepath.Glob(loc.Location)
	if err != nil {
		return nil, err
	}
	
	// Filter to only include directories
	var dirMatches []string
	for _, match := range matches {
		if isDirectory(match) {
			dirMatches = append(dirMatches, match)
		}
	}
	
	// Sort matches for consistent output
	sort.Strings(dirMatches)
	
	// Create new Location for each match
	var result []Location
	for _, match := range dirMatches {
		// Use the directory name as the name if original name is generic
		name := loc.Name
		if loc.Name == "" || loc.Name == filepath.Base(filepath.Dir(loc.Location)) {
			name = filepath.Base(match)
		}
		
		newLoc := Location{
			Name:     name,
			Location: match,
			Type:     loc.Type,
			Commands: append([]Command{}, loc.Commands...),
			Include:  append([]string{}, loc.Include...),
			Exclude:  append([]string{}, loc.Exclude...),
		}
		result = append(result, newLoc)
	}
	
	return result, nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}