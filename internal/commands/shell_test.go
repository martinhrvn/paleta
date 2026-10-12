package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scratch"
	"github.com/martinhrvn/paleta/internal/shelllog"
)

// shellProject makes a project root with a .pltrc and an infra/ subdirectory,
// and points HOME at a temp dir so the shell log and scratch list land there.
func shellProject(t *testing.T) (root, sub string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".pltrc"), []byte("locations:\n  - location: .\n    commands: [\"echo hi\"]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sub = filepath.Join(root, "infra")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	return config.FindProjectRoot(root), sub
}

func TestRun_ShellRecord_AppendsUnderProjectRoot(t *testing.T) {
	root, sub := shellProject(t)
	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"shell-record", sub, "pnpm cdk diff FooStack"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	recs, err := shelllog.Load(shelllog.LogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Dir != sub || recs[0].Command != "pnpm cdk diff FooStack" || recs[0].Time.IsZero() {
		t.Fatalf("records = %+v", recs)
	}
}

func TestRun_ShellRecord_IgnoresTrivialAndPlt(t *testing.T) {
	root, sub := shellProject(t)
	for _, cmd := range []string{"ls -la", "plt select", "  "} {
		var out, errOut bytes.Buffer
		if code := Run("dev", []string{"shell-record", sub, cmd}, &out, &errOut); code != 0 {
			t.Fatalf("%q: exit %d", cmd, code)
		}
	}
	if _, err := os.Stat(shelllog.LogPath(root)); !os.IsNotExist(err) {
		t.Fatalf("no log should be written for ignored commands (stat err %v)", err)
	}
}

func TestRun_ShellRecord_UsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"shell-record", "/tmp"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}

func TestRun_Add_PinsAndDedupes(t *testing.T) {
	root, sub := shellProject(t)
	t.Chdir(sub)

	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"add", "pnpm", "cdk", "diff", "FooStack"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "pnpm cdk diff FooStack") {
		t.Errorf("stdout = %q, want it to echo the pinned command", out.String())
	}
	out.Reset()
	if code := Run("dev", []string{"add", "pnpm cdk diff FooStack"}, &out, &errOut); code != 0 {
		t.Fatalf("second add exit %d", code)
	}
	if !strings.Contains(out.String(), "already pinned") {
		t.Errorf("stdout = %q, want 'already pinned'", out.String())
	}

	l, err := scratch.Load(scratch.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if len(l.Entries) != 1 || l.Entries[0].Dir != wd || l.Entries[0].Command != "pnpm cdk diff FooStack" {
		t.Fatalf("entries = %+v", l.Entries)
	}
}

func TestRun_Add_OutsideProjectFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"add", "make deploy"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "not inside a project") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRun_Add_NoCommandIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run("dev", []string{"add"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}

func TestUsageListsAdd(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	if !strings.Contains(out.String(), "add <command>") {
		t.Error("usage should document plt add")
	}
	if strings.Contains(out.String(), "shell-record") {
		t.Error("shell-record is internal and should stay out of usage")
	}
}

// promoteProject writes a .pltrc into a fresh project dir (with the given
// subdirectories) and loads it the way plt select does.
func promoteProject(t *testing.T, pltrc string, dirs ...string) (*config.Config, string) {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, ".pltrc")
	if err := os.WriteFile(path, []byte(pltrc), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.LoadOptions{Workdir: root})
	if err != nil {
		t.Fatal(err)
	}
	return cfg, root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func authoredCommands(t *testing.T, path string) map[string][]config.Command {
	t.Helper()
	cfg, err := config.LoadAuthored(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]config.Command{}
	for _, loc := range cfg.Locations {
		out[filepath.Clean(loc.Location)] = loc.Commands
	}
	return out
}

func TestPromoteShellCommand_ExactLocationKeepsComments(t *testing.T) {
	cfg, root := promoteProject(t, `# project commands
locations:
  - name: infra # the CDK app
    location: infra
    commands:
      - pnpm i
`, "infra")
	if err := PromoteShellCommand(cfg, cfg.Path, filepath.Join(root, "infra"), "pnpm cdk diff FooStack", "cdk-diff"); err != nil {
		t.Fatalf("PromoteShellCommand: %v", err)
	}
	text := readFile(t, cfg.Path)
	if !strings.Contains(text, "# project commands") || !strings.Contains(text, "# the CDK app") {
		t.Errorf("comments should be preserved:\n%s", text)
	}
	cmds := authoredCommands(t, cfg.Path)["infra"]
	if len(cmds) != 2 || cmds[1].Name != "cdk-diff" || cmds[1].Command != "pnpm cdk diff FooStack" {
		t.Fatalf("infra commands = %+v", cmds)
	}
}

func TestPromoteShellCommand_SubdirectoryIsCdPrefixed(t *testing.T) {
	cfg, root := promoteProject(t, `locations:
  - name: infra
    location: infra
`, "infra/stacks/my stack")
	if err := PromoteShellCommand(cfg, cfg.Path, filepath.Join(root, "infra", "stacks", "my stack"), "make plan", "plan"); err != nil {
		t.Fatal(err)
	}
	cmds := authoredCommands(t, cfg.Path)["infra"]
	if len(cmds) != 1 || cmds[0].Command != "cd 'stacks/my stack' && make plan" {
		t.Fatalf("infra commands = %+v", cmds)
	}
}

func TestPromoteShellCommand_RootEntryWithoutLocationKey(t *testing.T) {
	cfg, root := promoteProject(t, `locations:
  - commands:
      - make all
`)
	if err := PromoteShellCommand(cfg, cfg.Path, root, "make deploy", "deploy"); err != nil {
		t.Fatal(err)
	}
	authored, _ := config.LoadAuthored(cfg.Path)
	if len(authored.Locations) != 1 {
		t.Fatalf("should append to the root entry, not add a location:\n%s", readFile(t, cfg.Path))
	}
	cmds := authored.Locations[0].Commands
	if len(cmds) != 2 || cmds[1].Command != "make deploy" {
		t.Fatalf("root commands = %+v", cmds)
	}
}

func TestPromoteShellCommand_GlobChildAddsRelativeLocation(t *testing.T) {
	cfg, root := promoteProject(t, `locations:
  - location: packages/*
    commands: ["pnpm build"]
`, "packages/web", "packages/api")
	if err := PromoteShellCommand(cfg, cfg.Path, filepath.Join(root, "packages", "web"), "pnpm dev --port 4000", "dev-4000"); err != nil {
		t.Fatal(err)
	}
	cmds := authoredCommands(t, cfg.Path)
	if got := cmds["packages/web"]; len(got) != 1 || got[0].Command != "pnpm dev --port 4000" {
		t.Fatalf("expected a new packages/web location, got %+v\n%s", cmds, readFile(t, cfg.Path))
	}
	if text := readFile(t, cfg.Path); strings.Contains(text, root) {
		t.Errorf("new location should be relative, not absolute:\n%s", text)
	}
}

func TestPromoteShellCommand_UncoveredDirAddsUnnamedLocation(t *testing.T) {
	cfg, root := promoteProject(t, `locations:
  - name: web
    location: web
`, "web", "tools/x")
	if err := PromoteShellCommand(cfg, cfg.Path, filepath.Join(root, "tools", "x"), "./gen.sh", "gen"); err != nil {
		t.Fatal(err)
	}
	authored, _ := config.LoadAuthored(cfg.Path)
	if len(authored.Locations) != 2 {
		t.Fatalf("want a second location:\n%s", readFile(t, cfg.Path))
	}
	loc := authored.Locations[1]
	if loc.Location != "tools/x" || loc.Name != "" || len(loc.Commands) != 1 || loc.Commands[0].Command != "./gen.sh" {
		t.Fatalf("new location = %+v", loc)
	}
}

func TestPromoteShellCommand_OutsideProjectFails(t *testing.T) {
	cfg, _ := promoteProject(t, "locations:\n  - location: .\n")
	if err := PromoteShellCommand(cfg, cfg.Path, t.TempDir(), "make x", "x"); err == nil {
		t.Fatal("a directory outside the project should be rejected")
	}
}

func TestSelectorBackend_WiresShellHistoryAndScratch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, root := promoteProject(t, "locations:\n  - name: infra\n    location: infra\n", "infra")
	t.Chdir(root)
	key := config.FindProjectRoot(root)
	infra := filepath.Join(root, "infra")

	_ = shelllog.Append(shelllog.LogPath(key), shelllog.Record{Time: time.Now(), Dir: infra, Command: "pnpm cdk diff"})
	if _, err := PinCommand(key, infra, "make plan"); err != nil {
		t.Fatal(err)
	}
	if err := SetShellView(key, "30d", true); err != nil {
		t.Fatal(err)
	}

	be := selectorBackend(cfg)
	if len(be.ShellHistory) != 1 || be.ShellHistory[0].Command != "pnpm cdk diff" {
		t.Errorf("ShellHistory = %+v", be.ShellHistory)
	}
	if len(be.Scratch) != 1 || be.Scratch[0].Command != "make plan" {
		t.Errorf("Scratch = %+v", be.Scratch)
	}
	if be.ShellPeriod != "30d" || !be.ShellByFolder {
		t.Errorf("ShellPeriod = %q, ShellByFolder = %v", be.ShellPeriod, be.ShellByFolder)
	}
	if be.PinShell == nil || be.UnpinShell == nil || be.SaveShellView == nil || be.PromoteShell == nil {
		t.Fatal("pin, unpin, period and promote should all be wired for a config with a file")
	}

	if err := be.PinShell(infra, "pnpm cdk diff"); err != nil {
		t.Fatal(err)
	}
	if err := be.UnpinShell(infra, "make plan"); err != nil {
		t.Fatal(err)
	}
	if err := be.SaveShellView("1h", false); err != nil {
		t.Fatal(err)
	}
	l, _ := scratch.Load(scratch.Path(key))
	if len(l.Entries) != 1 || l.Entries[0].Command != "pnpm cdk diff" || l.ShellPeriod != "1h" || l.ShellByFolder {
		t.Errorf("scratch after pin/unpin/period = %+v", l)
	}

	if err := be.PromoteShell(infra, "pnpm cdk diff", "cdk-diff"); err != nil {
		t.Fatal(err)
	}
	if cmds := authoredCommands(t, cfg.Path)["infra"]; len(cmds) != 1 || cmds[0].Name != "cdk-diff" {
		t.Errorf("promoted commands = %+v", cmds)
	}
}

func TestSelectorBackend_NoConfigFileDisablesPromote(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	be := selectorBackend(&config.Config{})
	if be.PromoteShell != nil {
		t.Error("promote needs a config file to write to")
	}
}
