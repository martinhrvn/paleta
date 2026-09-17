package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run("1.2.3", []string{"version"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if got, want := strings.TrimSpace(out.String()), "paleta version 1.2.3"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"bogus"}, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Unknown command: bogus") {
		t.Errorf("stderr = %q, want it to name the unknown command", errOut.String())
	}
	if !strings.Contains(out.String(), "USAGE:") {
		t.Error("expected usage on stdout after an unknown command")
	}
}

func TestRun_NoArgsShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run("dev", nil, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "USAGE:") {
		t.Error("expected usage on stdout")
	}
}

// A subcommand rejects an option it does not know instead of ignoring it.
func TestRun_UnknownOptionFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"lint", "--bogus"}, &out, &errOut); code == 0 {
		t.Error("expected a non-zero exit for an unknown lint option")
	}
	if !strings.Contains(errOut.String(), "bogus") {
		t.Errorf("stderr = %q, want it to mention the bad option", errOut.String())
	}
}
