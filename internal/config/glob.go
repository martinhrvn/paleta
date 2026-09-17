package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExpandGlobPatterns replaces each glob location with one location per matching
// directory, applying that location's `exclude_locations` and `overrides`. It
// returns the expanded list plus any non-fatal notices (keys that had no effect),
// which the caller surfaces rather than failing the load.
func ExpandGlobPatterns(locations []Location) ([]Location, []Warning, error) {
	return expandGlobPatterns(locations, "")
}

// expandGlobPatterns is ExpandGlobPatterns with the patterns matched under
// baseDir (empty means the process working directory). The expanded locations
// keep the authored, relative form so later stages treat them like any other.
func expandGlobPatterns(locations []Location, baseDir string) ([]Location, []Warning, error) {
	var result []Location
	var warnings []Warning

	for _, loc := range locations {
		// Check if location contains glob pattern
		if strings.Contains(loc.Location, "*") {
			// Validate glob pattern before expansion
			if err := validateGlobPattern(loc.Location); err != nil {
				return nil, nil, err
			}

			expanded, expandWarnings, err := expandSingleGlob(loc, baseDir)
			if err != nil {
				return nil, nil, err
			}
			result = append(result, expanded...)
			warnings = append(warnings, expandWarnings...)
		} else {
			// No glob pattern, add as-is. The expansion-only keys can't do anything
			// here, so say so rather than failing silently.
			warnings = append(warnings, nonGlobDirectiveWarnings(loc)...)
			result = append(result, loc)
		}
	}

	return result, warnings, nil
}

// nonGlobDirectiveWarnings reports `exclude_locations` / `overrides` authored on a
// location that isn't a glob pattern, where they have no effect.
func nonGlobDirectiveWarnings(loc Location) []Warning {
	var warnings []Warning
	label := loc.DisplayName()
	if len(loc.ExcludeLocations) > 0 {
		warnings = append(warnings, Warning{
			Kind:    "ignored",
			Scope:   "location",
			Context: label,
			Name:    "exclude_locations",
			Reason:  "only applies to a glob location (e.g. \"services/*\")",
		})
	}
	if len(loc.Overrides) > 0 {
		warnings = append(warnings, Warning{
			Kind:    "ignored",
			Scope:   "location",
			Context: label,
			Name:    "overrides",
			Reason:  "only applies to a glob location (e.g. \"services/*\")",
		})
	}
	return warnings
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

func expandSingleGlob(loc Location, baseDir string) ([]Location, []Warning, error) {
	// Match under baseDir, but hand back the authored (relative) form.
	pattern := loc.Location
	joined := baseDir != "" && !filepath.IsAbs(pattern)
	if joined {
		pattern = filepath.Join(baseDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, err
	}

	// Keep only directories, in the same form the pattern was written in.
	var dirMatches []string
	for _, match := range matches {
		if !isDirectory(match) {
			continue
		}
		if joined {
			if rel, rerr := filepath.Rel(baseDir, match); rerr == nil {
				match = rel
			}
		}
		dirMatches = append(dirMatches, match)
	}

	// Sort matches for consistent output
	sort.Strings(dirMatches)

	// Remember every folder the pattern matched before exclusions, so an override
	// for a folder the user deliberately excluded isn't reported as a typo.
	excludedBases := make(map[string]bool)
	kept := dirMatches[:0:0]
	for _, match := range dirMatches {
		if matchesAnyPattern(filepath.Base(match), loc.ExcludeLocations) {
			excludedBases[filepath.Base(match)] = true
			continue
		}
		kept = append(kept, match)
	}
	dirMatches = kept

	// Create new Location for each match. Every authored Location field has to be
	// accounted for here — a field left out of this literal is silently dropped
	// from every expanded location.
	usedOverrides := make(map[string]bool)
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
			Types:    append(Types{}, loc.Types...),
			Commands: append([]Command{}, loc.Commands...),
			Include:  append([]string{}, loc.Include...),
			Exclude:  append([]string{}, loc.Exclude...),
			Env:      copyEnv(loc.Env),
			Focused:  loc.Focused,
			// ExcludeLocations and Overrides are consumed here, so they are
			// deliberately not carried onto an expanded child.
		}

		base := filepath.Base(match)
		for _, key := range sortedOverrideKeys(loc.Overrides) {
			if !matchesPattern(base, key) {
				continue
			}
			usedOverrides[key] = true
			newLoc = applyLocationOverride(newLoc, loc.Overrides[key])
		}

		result = append(result, newLoc)
	}

	// An override key that matched nothing is usually a typo or a renamed folder.
	var warnings []Warning
	label := loc.DisplayName()
	for _, key := range sortedOverrideKeys(loc.Overrides) {
		if usedOverrides[key] || excludedBases[key] {
			continue
		}
		warnings = append(warnings, Warning{
			Kind:    "ignored",
			Scope:   "location",
			Context: label,
			Name:    key,
			Reason:  fmt.Sprintf("override key matches no folder expanded from %q", loc.Location),
		})
	}

	return result, warnings, nil
}

// applyLocationOverride merges one folder's override into its expanded location:
// commands merge by name (a same-named command replaces the inherited one in
// place, the rest append), env merges per key, and every other field replaces the
// inherited value when set.
func applyLocationOverride(loc Location, ov LocationOverride) Location {
	if ov.Name != "" {
		loc.Name = ov.Name
	}
	if len(ov.Types) > 0 {
		loc.Types = append(Types{}, ov.Types...)
	}
	if len(ov.Include) > 0 {
		loc.Include = append([]string{}, ov.Include...)
	}
	if len(ov.Exclude) > 0 {
		loc.Exclude = append([]string{}, ov.Exclude...)
	}
	if len(ov.Env) > 0 {
		merged := copyEnv(loc.Env)
		if merged == nil {
			merged = make(map[string]string, len(ov.Env))
		}
		for k, v := range ov.Env {
			merged[k] = v // override wins, as command-level env does over location-level
		}
		loc.Env = merged
	}
	if len(ov.Commands) > 0 {
		loc.Commands = mergeCommands(loc.Commands, ov.Commands)
	}
	return loc
}

// mergeCommands overlays extra onto base: a named command replaces the same-named
// base command in place (keeping its position); everything else appends in order.
// Unnamed commands always append — they have no identity to match on.
func mergeCommands(base, extra []Command) []Command {
	merged := append([]Command{}, base...)
	for _, cmd := range extra {
		replaced := false
		if cmd.Name != "" {
			for i := range merged {
				if merged[i].Name == cmd.Name {
					merged[i] = cmd
					replaced = true
					break
				}
			}
		}
		if !replaced {
			merged = append(merged, cmd)
		}
	}
	return merged
}

// copyEnv returns a fresh copy of an env map (nil when empty), so expanded
// siblings never share one map and an override can't leak into its neighbours.
func copyEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}

// matchesAnyPattern reports whether name matches any of the patterns.
func matchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesPattern(name, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern matches a folder name against one filepath.Match pattern. A
// malformed pattern simply matches nothing, as in filterCommands.
func matchesPattern(name, pattern string) bool {
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
