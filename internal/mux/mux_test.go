package mux

import "testing"

func envFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Multiplexer
	}{
		{"none", map[string]string{}, None},
		{"tmux", map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"}, Tmux},
		{"zellij", map[string]string{"ZELLIJ": "0"}, Zellij},
		{"zellij session name only is not enough", map[string]string{"ZELLIJ_SESSION_NAME": "main"}, None},
		// When nested, the inner-most (zellij, checked first) wins.
		{"both prefers zellij", map[string]string{"ZELLIJ": "0", "TMUX": "/tmp/tmux"}, Zellij},
		{"empty values ignored", map[string]string{"TMUX": "", "ZELLIJ": ""}, None},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(envFrom(tt.env)); got != tt.want {
				t.Errorf("Detect() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActive(t *testing.T) {
	if None.Active() {
		t.Error("None should not be Active")
	}
	if !Tmux.Active() {
		t.Error("Tmux should be Active")
	}
	if !Zellij.Active() {
		t.Error("Zellij should be Active")
	}
}

func TestLabel(t *testing.T) {
	cases := map[Multiplexer]string{
		None:   "",
		Tmux:   "tmux",
		Zellij: "zellij",
	}
	for m, want := range cases {
		if got := m.Label(); got != want {
			t.Errorf("%v.Label() = %q, want %q", m, got, want)
		}
	}
}
