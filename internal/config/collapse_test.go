package config

import (
	"reflect"
	"testing"
)

// plain builds the kind of location the wizard produces: a folder, its name, and
// its detected types, with nothing hand-written on it.
func plain(path, name string, types ...string) Location {
	return Location{Name: name, Location: path, Types: append(Types{}, types...)}
}

func TestCollapse_ThreeSiblingsBecomeGlob(t *testing.T) {
	got := CollapseSiblingsToGlobs([]Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		plain("packages/c", "c", "npm"),
	}, nil)

	want := []Location{{Location: "packages/*", Types: Types{"npm"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collapsed = %+v, want %+v", got, want)
	}
}

// TestCollapse_TwoSiblingsStayExplicit: a pair isn't worth a glob — the explicit
// form is the same length and says exactly what it means.
func TestCollapse_TwoSiblingsStayExplicit(t *testing.T) {
	in := []Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
	}
	got := CollapseSiblingsToGlobs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged", got)
	}
}

// TestCollapse_MixedTypesStayExplicit: one glob carries one type list, which
// every matched folder inherits — so a parent holding both npm and go projects
// can't be written as a glob without relabelling rows ("[npm] build").
func TestCollapse_MixedTypesStayExplicit(t *testing.T) {
	in := []Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		plain("packages/c", "c", "go"),
	}
	got := CollapseSiblingsToGlobs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged", got)
	}
}

// TestCollapse_DeselectedSiblingBecomesExclusion: the glob would pull in a folder
// the user deliberately unticked, so it is named in exclude_locations.
func TestCollapse_DeselectedSiblingBecomesExclusion(t *testing.T) {
	got := CollapseSiblingsToGlobs([]Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		plain("packages/c", "c", "npm"),
	}, []Location{plain("packages/legacy", "legacy", "npm")})

	want := []Location{{
		Location:         "packages/*",
		Types:            Types{"npm"},
		ExcludeLocations: []string{"legacy"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collapsed = %+v, want %+v", got, want)
	}
}

// TestCollapse_ManyDeselectedStaysExplicit: once there are as many exclusions as
// entries, the glob is no shorter and harder to read, so keep what was ticked.
func TestCollapse_ManyDeselectedStaysExplicit(t *testing.T) {
	in := []Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		plain("packages/c", "c", "npm"),
	}
	got := CollapseSiblingsToGlobs(in, []Location{
		plain("packages/d", "d", "npm"),
		plain("packages/e", "e", "npm"),
		plain("packages/f", "f", "npm"),
	})
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged", got)
	}
}

// TestCollapse_NameCollisionStaysExplicit: a glob's children are named after
// their folder, so a member whose name had to be disambiguated (apps/web vs
// packages/web, see assignWizardNames) can't survive collapsing.
func TestCollapse_NameCollisionStaysExplicit(t *testing.T) {
	in := []Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		plain("packages/web", "packages/web", "npm"),
	}
	got := CollapseSiblingsToGlobs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged (packages/web keeps its path name)", got)
	}
}

// TestCollapse_RootParentNeverCollapses: "*" at the root is rejected by
// validateGlobPattern, and would match every directory in the repo besides.
func TestCollapse_RootParentNeverCollapses(t *testing.T) {
	in := []Location{
		plain("a", "a", "npm"),
		plain("b", "b", "npm"),
		plain("c", "c", "npm"),
	}
	got := CollapseSiblingsToGlobs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged", got)
	}
}

// TestCollapse_CustomizedLocationStaysExplicit: hand-written commands, filters and
// env belong to one folder; a glob would spread them across every sibling.
func TestCollapse_CustomizedLocationStaysExplicit(t *testing.T) {
	custom := plain("packages/c", "c", "npm")
	custom.Commands = []Command{{Name: "deploy", Command: "./deploy.sh"}}
	in := []Location{
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		custom,
	}
	got := CollapseSiblingsToGlobs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged", got)
	}
}

// TestCollapse_KeepsOrderAndNonSiblings: the glob takes the position of the first
// folder it replaces, and everything else is passed through untouched.
func TestCollapse_KeepsOrderAndNonSiblings(t *testing.T) {
	got := CollapseSiblingsToGlobs([]Location{
		plain("infra", "infra", "make"),
		plain("packages/a", "a", "npm"),
		plain("services/api", "api", "go"),
		plain("packages/b", "b", "npm"),
		plain("packages/c", "c", "npm"),
	}, nil)

	want := []Location{
		plain("infra", "infra", "make"),
		{Location: "packages/*", Types: Types{"npm"}},
		plain("services/api", "api", "go"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collapsed = %+v, want %+v", got, want)
	}
}

// TestCollapse_AuthoredGlobPassesThrough: a location already written as a glob
// is left alone, and doesn't count towards its parent's group.
func TestCollapse_AuthoredGlobPassesThrough(t *testing.T) {
	in := []Location{
		{Location: "packages/*", Types: Types{"npm"}},
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
	}
	got := CollapseSiblingsToGlobs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("collapsed = %+v, want it unchanged", got)
	}
}

// TestPlanGlobCollapse_ReportsMembers: the wizard annotates the rows a collapse
// would absorb, so the written form is visible before Enter.
func TestPlanGlobCollapse_ReportsMembers(t *testing.T) {
	plan := PlanGlobCollapse([]Location{
		plain("infra", "infra", "make"),
		plain("packages/a", "a", "npm"),
		plain("packages/b", "b", "npm"),
		plain("packages/c", "c", "npm"),
	}, nil)

	if len(plan) != 1 {
		t.Fatalf("plan = %+v, want one group", plan)
	}
	if plan[0].Pattern != "packages/*" {
		t.Errorf("Pattern = %q, want packages/*", plan[0].Pattern)
	}
	if !reflect.DeepEqual(plan[0].Members, []int{1, 2, 3}) {
		t.Errorf("Members = %v, want the three packages rows", plan[0].Members)
	}
}
