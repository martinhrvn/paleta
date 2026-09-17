package config

import (
	"path/filepath"
	"sort"
	"strings"
)

// minGlobGroup is how many siblings it takes before a glob is worth writing. Two
// entries collapse to a one-line glob plus nothing gained; three is where the
// config gets shorter, and — more to the point — where a parent directory starts
// looking like a place new projects will keep appearing.
const minGlobGroup = 3

// GlobCollapse is one group of sibling locations that would be written as a
// single glob entry.
type GlobCollapse struct {
	// Pattern is the glob that replaces the members, e.g. "packages/*".
	Pattern string
	// Members are indices into the selected slice passed to PlanGlobCollapse, in
	// input order. The group takes the position of the first one.
	Members []int
	// Types is the shared type list every member declared.
	Types Types
	// Exclude holds the folder base names of unticked siblings the pattern would
	// otherwise pull back in.
	Exclude []string
}

// CollapseSiblingsToGlobs rewrites groups of sibling locations as glob entries:
// three npm packages under packages/ become a single `packages/*`, which is
// shorter to read and picks up tomorrow's package without re-running the wizard.
// deselected carries the locations the user left unticked, so a glob that would
// pull one back in excludes it by name.
//
// Locations that aren't collapsed are returned untouched and in their original
// order, so this is safe to run over any selection.
func CollapseSiblingsToGlobs(selected, deselected []Location) []Location {
	plan := PlanGlobCollapse(selected, deselected)
	if len(plan) == 0 {
		return selected
	}

	// absorbed[i] is the group index that swallows selected[i]; the group is
	// emitted at its first member's position so surrounding order is preserved.
	absorbed := make(map[int]int, len(selected))
	firstOf := make(map[int]int, len(plan))
	for g, group := range plan {
		for _, idx := range group.Members {
			absorbed[idx] = g
		}
		firstOf[g] = group.Members[0]
	}

	out := make([]Location, 0, len(selected))
	for i, loc := range selected {
		g, ok := absorbed[i]
		if !ok {
			out = append(out, loc)
			continue
		}
		if firstOf[g] != i {
			continue
		}
		out = append(out, Location{
			// Name is left empty on purpose: glob expansion names each matched
			// folder after itself (see expandSingleGlob), which is exactly the
			// name every member already had.
			Location:         plan[g].Pattern,
			Types:            append(Types{}, plan[g].Types...),
			ExcludeLocations: plan[g].Exclude,
		})
	}
	return out
}

// PlanGlobCollapse works out which sibling groups would be written as globs,
// without applying the rewrite. The wizard uses it to annotate the rows a
// collapse would absorb, so the written form is visible before Enter.
func PlanGlobCollapse(selected, deselected []Location) []GlobCollapse {
	byParent := make(map[string][]int)
	var parents []string // first-seen order, so the plan is deterministic
	for i, loc := range selected {
		parent, ok := collapsibleParent(loc)
		if !ok {
			continue
		}
		if _, seen := byParent[parent]; !seen {
			parents = append(parents, parent)
		}
		byParent[parent] = append(byParent[parent], i)
	}

	var plan []GlobCollapse
	for _, parent := range parents {
		members := byParent[parent]
		if len(members) < minGlobGroup {
			continue
		}
		// One glob carries one type list, and every folder it matches inherits it.
		// A parent holding both an npm and a go project can't be expressed that way
		// without relabelling every row, so it stays explicit.
		types := selected[members[0]].Types
		uniform := true
		for _, idx := range members[1:] {
			if !sameTypes(types, selected[idx].Types) {
				uniform = false
				break
			}
		}
		if !uniform {
			continue
		}

		// Unticked siblings have to be named explicitly — the pattern would
		// otherwise bring them back. Past the point where there are as many
		// exclusions as entries, the glob is no shorter and harder to read.
		excludes := excludedSiblings(parent, deselected)
		if len(excludes) >= len(members) {
			continue
		}

		plan = append(plan, GlobCollapse{
			Pattern: parent + "/*",
			Members: members,
			Types:   types,
			Exclude: excludes,
		})
	}
	return plan
}

// collapsibleParent returns the parent directory a location could be collapsed
// into, and whether it may be collapsed at all.
//
// A location is only collapsible when the glob would reproduce it exactly: its
// name has to be the one glob expansion would give it (its folder's), and it must
// carry nothing folder-specific, since a glob spreads every field it holds across
// all of its siblings.
func collapsibleParent(loc Location) (string, bool) {
	if strings.Contains(loc.Location, "*") {
		return "", false // already a glob
	}
	if len(loc.Commands) > 0 || len(loc.Include) > 0 || len(loc.Exclude) > 0 ||
		len(loc.Env) > 0 || len(loc.ExcludeLocations) > 0 || len(loc.Overrides) > 0 {
		return "", false
	}
	if len(loc.Types) == 0 {
		return "", false // nothing to inherit; the entry is doing no work
	}

	path := filepath.Clean(loc.Location)
	base := filepath.Base(path)
	// A name that isn't the folder's own was chosen for a reason — either the user
	// wrote it, or assignWizardNames had to disambiguate two folders sharing a base
	// name. Collapsing would silently hand the folder its base name back and
	// reintroduce the collision.
	if loc.Name != "" && loc.Name != base {
		return "", false
	}

	parent := filepath.Dir(path)
	if parent == "." || parent == "" || parent == string(filepath.Separator) {
		return "", false // "*" at the root is rejected by validateGlobPattern
	}
	return parent, true
}

// excludedSiblings lists the base names of unticked locations sitting directly
// under parent, sorted so the written config is stable.
func excludedSiblings(parent string, deselected []Location) []string {
	var out []string
	for _, loc := range deselected {
		path := filepath.Clean(loc.Location)
		if strings.Contains(path, "*") || filepath.Dir(path) != parent {
			continue
		}
		out = append(out, filepath.Base(path))
	}
	sort.Strings(out)
	return out
}

func sameTypes(a, b Types) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
