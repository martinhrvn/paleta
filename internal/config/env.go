package config

import "os"

// EffectiveEnv computes the environment variables that apply to a command.
// Location-level env provides defaults; command-level env overrides them per
// key. Values may reference the ambient process environment and sibling
// gopm-defined variables (e.g. "${HOME}/bin" or "${BIN}:$PATH"); references are
// resolved before the command runs. Returns nil when no env is defined.
func EffectiveEnv(loc Location, cmd Command) map[string]string {
	if len(loc.Env) == 0 && len(cmd.Env) == 0 {
		return nil
	}

	merged := make(map[string]string, len(loc.Env)+len(cmd.Env))
	for k, v := range loc.Env {
		merged[k] = v
	}
	for k, v := range cmd.Env {
		merged[k] = v // command-level overrides location-level
	}

	return resolveEnv(merged)
}

// resolveEnv expands ${VAR}/$VAR references in each value. A reference to
// another key in the map is resolved recursively; anything else falls back to
// the ambient environment. Undefined variables expand to an empty string, and
// reference cycles are broken (resolving to empty) so resolution always
// terminates.
func resolveEnv(raw map[string]string) map[string]string {
	resolved := make(map[string]string, len(raw))
	for key := range raw {
		resolved[key] = expandValue(key, raw, map[string]bool{})
	}
	return resolved
}

// expandValue resolves the value of a single key. The visiting set tracks keys
// currently being resolved on this path to detect cycles.
func expandValue(key string, raw map[string]string, visiting map[string]bool) string {
	if visiting[key] {
		return "" // cycle: stop recursing
	}
	visiting[key] = true
	defer delete(visiting, key)

	return os.Expand(raw[key], func(name string) string {
		if _, ok := raw[name]; ok {
			return expandValue(name, raw, visiting)
		}
		return os.Getenv(name)
	})
}
