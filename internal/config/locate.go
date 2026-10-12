package config

import (
	"path/filepath"
	"strings"
)

// LocationFor returns the location whose directory contains dir, preferring the
// deepest one (so a command run in packages/web/src belongs to packages/web,
// not to the root). Locations must be loaded (absolute). When no location
// matches the paths as given, both sides are compared again with symlinks
// resolved, since the shell's $PWD is the logical path.
func (c *Config) LocationFor(dir string) (*Location, bool) {
	if loc, ok := c.longestPrefix(dir, filepath.Clean); ok {
		return loc, true
	}
	return c.longestPrefix(dir, resolvePath)
}

func (c *Config) longestPrefix(dir string, norm func(string) string) (*Location, bool) {
	dir = norm(dir)
	var best *Location
	bestLen := -1
	for i := range c.Locations {
		loc := &c.Locations[i]
		if loc.Location == "" {
			continue
		}
		base := norm(loc.Location)
		if !isWithin(dir, base) || len(base) <= bestLen {
			continue
		}
		best, bestLen = loc, len(base)
	}
	return best, best != nil
}

// isWithin reports whether dir is base or below it, on a path boundary.
func isWithin(dir, base string) bool {
	if dir == base {
		return true
	}
	if base == string(filepath.Separator) {
		return strings.HasPrefix(dir, base)
	}
	return strings.HasPrefix(dir, base+string(filepath.Separator))
}
