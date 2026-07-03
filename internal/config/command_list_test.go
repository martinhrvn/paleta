package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCommand_UnmarshalYAML_List covers the list form of the `command` field: a
// sequence of shell segments joined with " && " for execution, with the original
// parts retained for round-tripping.
func TestCommand_UnmarshalYAML_List(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantName   string
		wantJoined string
		wantParts  []string
	}{
		{
			name: "block list",
			yaml: `name: ci-and-dev
command:
  - pnpm i
  - pnpm dev`,
			wantName:   "ci-and-dev",
			wantJoined: "pnpm i && pnpm dev",
			wantParts:  []string{"pnpm i", "pnpm dev"},
		},
		{
			name:       "flow list",
			yaml:       `command: [a, b, c]`,
			wantJoined: "a && b && c",
			wantParts:  []string{"a", "b", "c"},
		},
		{
			name: "drops empty entries",
			yaml: `command:
  - a
  - ""
  - "   "
  - b`,
			wantJoined: "a && b",
			wantParts:  []string{"a", "b"},
		},
		{
			name:       "single-item list keeps scalar-like joined value",
			yaml:       `command: [only]`,
			wantJoined: "only",
			wantParts:  []string{"only"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd Command
			if err := yaml.Unmarshal([]byte(tt.yaml), &cmd); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if cmd.Command != tt.wantJoined {
				t.Errorf("Command = %q, want %q", cmd.Command, tt.wantJoined)
			}
			if !reflect.DeepEqual(cmd.parts, tt.wantParts) {
				t.Errorf("parts = %#v, want %#v", cmd.parts, tt.wantParts)
			}
		})
	}
}

// TestCommand_UnmarshalYAML_Scalar confirms the existing scalar / bare-string
// forms still parse and carry no parts (so they re-marshal as a scalar).
func TestCommand_UnmarshalYAML_Scalar(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"bare string", `npm run build`},
		{"object scalar", `command: npm run build`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd Command
			if err := yaml.Unmarshal([]byte(tt.yaml), &cmd); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if cmd.Command != "npm run build" {
				t.Errorf("Command = %q, want %q", cmd.Command, "npm run build")
			}
			if cmd.parts != nil {
				t.Errorf("parts = %#v, want nil for scalar form", cmd.parts)
			}
		})
	}
}

// TestCommand_MarshalYAML checks that a multi-item command re-marshals as a YAML
// sequence while single-item and scalar-authored commands stay scalar.
func TestCommand_MarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		wantSeq bool
	}{
		{"multi-item list", NewCommand("chain", []string{"a", "b"}), true},
		{"single-item list", NewCommand("solo", []string{"a"}), false},
		{"scalar", Command{Name: "solo", Command: "a"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := yaml.Marshal(tt.cmd)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(out)
			// A sequence renders `command:` on its own line followed by `- ` items.
			isSeq := strings.Contains(s, "command:\n") && strings.Contains(s, "- a")
			if isSeq != tt.wantSeq {
				t.Errorf("marshal sequence = %v, want %v; got:\n%s", isSeq, tt.wantSeq, s)
			}
		})
	}
}

// TestCommand_RoundTrip confirms a list-authored command survives an
// unmarshal → marshal → unmarshal cycle as a list.
func TestCommand_RoundTrip(t *testing.T) {
	const src = `name: chain
command:
  - a
  - b
  - c`
	var cmd Command
	if err := yaml.Unmarshal([]byte(src), &cmd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := yaml.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back Command
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back.parts, []string{"a", "b", "c"}) {
		t.Errorf("round-tripped parts = %#v, want [a b c]; marshaled as:\n%s", back.parts, out)
	}
	if back.Command != "a && b && c" {
		t.Errorf("round-tripped Command = %q, want %q", back.Command, "a && b && c")
	}
}

// TestNewCommand covers the constructor used by the save path.
func TestNewCommand(t *testing.T) {
	multi := NewCommand("chain", []string{"a", "", "b"})
	if multi.Command != "a && b" {
		t.Errorf("Command = %q, want %q", multi.Command, "a && b")
	}
	if !reflect.DeepEqual(multi.parts, []string{"a", "b"}) {
		t.Errorf("parts = %#v, want [a b]", multi.parts)
	}

	single := NewCommand("solo", []string{"a"})
	if single.Command != "a" {
		t.Errorf("Command = %q, want %q", single.Command, "a")
	}
	if !reflect.DeepEqual(single.parts, []string{"a"}) {
		t.Errorf("parts = %#v, want [a]", single.parts)
	}
}
