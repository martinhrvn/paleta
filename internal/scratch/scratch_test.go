package scratch

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/martinhrvn/paleta/internal/history"
)

var now = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func TestLoadMissingIsEmpty(t *testing.T) {
	l, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(l.Entries) != 0 || l.ShellPeriod != "" {
		t.Fatalf("got %+v, want empty list", l)
	}
}

func TestAddDedupesAndRemove(t *testing.T) {
	l := &List{}
	if !l.Add("/p", "pnpm cdk diff FooStack", now) {
		t.Fatal("first Add should add")
	}
	if l.Add("/p", "pnpm cdk diff FooStack  ", now) {
		t.Fatal("same dir + command (modulo whitespace) should not add twice")
	}
	if !l.Add("/p/infra", "pnpm cdk diff FooStack", now) {
		t.Fatal("same command in another dir is a separate entry")
	}
	if !l.Has("/p", "pnpm cdk diff FooStack") {
		t.Fatal("Has should find the entry")
	}
	if !l.Remove("/p", " pnpm cdk diff FooStack") {
		t.Fatal("Remove should remove an existing entry")
	}
	if l.Remove("/p", "pnpm cdk diff FooStack") {
		t.Fatal("Remove of a missing entry should report false")
	}
	if len(l.Entries) != 1 || l.Entries[0].Dir != "/p/infra" {
		t.Fatalf("entries = %+v", l.Entries)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scratch", "x.json")
	l := &List{ProjectRoot: "/p", ShellPeriod: "1h", ShellByFolder: true}
	l.Add("/p", "make deploy", now)
	if err := l.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ProjectRoot != "/p" || got.ShellPeriod != "1h" || !got.ShellByFolder || len(got.Entries) != 1 ||
		got.Entries[0].Command != "make deploy" || !got.Entries[0].Added.Equal(now) {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestPathUsesProjectKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".paleta", "scratch", history.ProjectKey("/p")+".json")
	if got := Path("/p"); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}
