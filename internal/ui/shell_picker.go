package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
	"github.com/martinhrvn/paleta/internal/scratch"
	"github.com/martinhrvn/paleta/internal/shelllog"
)

// scratchType is the badge pinned temporary commands carry in the palette.
const scratchType = "tmp"

// shellRow is one line of the Ctrl+R picker: a pinned temporary command or a
// group of recent shell commands, with where it runs, when it was pinned / last
// run, and how often it ran in the period.
type shellRow struct {
	Dir     string
	Command string
	Pinned  bool
	When    string // "5m ago", or "pinned"
	Count   int    // runs in the period; 0 for a pinned row
}

// shellDir is the directory a shell command is listed and run under. Grouped
// by folder it is where the command was typed; grouped by location (the
// default) it is the directory of the location containing that folder, so runs
// from anywhere inside one project count together and run from its root. A
// folder outside every location keeps its own directory.
func (m Model) shellDir(dir string) string {
	if m.shellByFolder {
		return dir
	}
	if loc, ok := m.config.LocationFor(dir); ok {
		return loc.Location
	}
	return dir
}

func (m Model) shellKey(dir, command string) string {
	return m.shellDir(dir) + "\x00" + strings.TrimSpace(command)
}

// shellRows lists the picker's rows: pinned commands first, then the recent
// shell commands in the current period grouped by location or by folder,
// newest first, leaving out any that are already pinned.
func (m Model) shellRows() []shellRow {
	var rows []shellRow
	pinned := map[string]bool{}
	for _, e := range m.backend.Scratch {
		rows = append(rows, shellRow{Dir: e.Dir, Command: e.Command, Pinned: true, When: "pinned"})
		pinned[m.shellKey(e.Dir, e.Command)] = true
	}
	now := m.now()
	key := func(r shelllog.Record) string { return m.shellKey(r.Dir, r.Command) }
	for _, g := range shelllog.Recent(m.backend.ShellHistory, m.shellPeriod.Since(now), shelllog.DefaultShown, key) {
		if pinned[key(g.Record)] {
			continue
		}
		rows = append(rows, shellRow{
			Dir:     m.shellDir(g.Dir),
			Command: g.Command,
			When:    history.FormatSince(now.Sub(g.Time)),
			Count:   g.Count,
		})
	}
	return rows
}

// enterShellPicker opens the Ctrl+R list on the remembered period and grouping.
func (m *Model) enterShellPicker() {
	m.mode = modeShellPick
	m.shellCursor = 0
	m.shellHint = ""
	m.shellPeriod = shelllog.ParsePeriod(m.backend.ShellPeriod)
	m.shellByFolder = m.backend.ShellByFolder
	m.searchInput.Blur()
}

func (m Model) updateShellPickMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.shellRows())
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlK:
		m.shellCursor = moveCursor(m.shellCursor, -1, n)
	case tea.KeyDown, tea.KeyCtrlJ:
		m.shellCursor = moveCursor(m.shellCursor, 1, n)
	case tea.KeyLeft:
		m.setShellPeriod(m.shellPeriod.Prev())
	case tea.KeyRight, tea.KeyTab:
		m.setShellPeriod(m.shellPeriod.Next())
	case tea.KeyEnter:
		if row, ok := m.currentShellRow(); ok {
			m.results = []SelectionResult{{Directory: row.Dir, Command: row.Command, Action: "shell"}}
			m.quitting = true
			return m, tea.Quit
		}
	case tea.KeyCtrlS:
		m.startShellPromote()
	case tea.KeyEscape:
		m.shellHint = ""
		m.leaveMode()
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "p":
			m.toggleShellPin()
		case "f":
			m.shellByFolder = !m.shellByFolder
			m.saveShellView()
		case "a":
			m.startShellPromote()
		}
	}
	return m, nil
}

func (m Model) currentShellRow() (shellRow, bool) {
	rows := m.shellRows()
	if m.shellCursor < 0 || m.shellCursor >= len(rows) {
		return shellRow{}, false
	}
	return rows[m.shellCursor], true
}

// setShellPeriod switches the recent list's time window.
func (m *Model) setShellPeriod(p shelllog.Period) {
	m.shellPeriod = p
	m.saveShellView()
}

// saveShellView remembers the current period and grouping (for this session
// and, through the backend, the next) and keeps the cursor on the possibly
// shorter list.
func (m *Model) saveShellView() {
	m.backend.ShellPeriod = m.shellPeriod.String()
	m.backend.ShellByFolder = m.shellByFolder
	if m.backend.SaveShellView != nil {
		_ = m.backend.SaveShellView(m.shellPeriod.String(), m.shellByFolder)
	}
	m.shellCursor = moveCursor(m.shellCursor, 0, len(m.shellRows()))
}

// toggleShellPin pins a recent command or unpins a pinned one, and refreshes
// the palette's tmp rows to match.
func (m *Model) toggleShellPin() {
	row, ok := m.currentShellRow()
	if !ok {
		return
	}
	if m.backend.PinShell == nil || m.backend.UnpinShell == nil {
		m.shellHint = "pinning is unavailable here"
		return
	}
	list := scratch.List{Entries: append([]scratch.Entry(nil), m.backend.Scratch...)}
	var err error
	if row.Pinned {
		err = m.backend.UnpinShell(row.Dir, row.Command)
		list.Remove(row.Dir, row.Command)
		m.shellHint = "unpinned"
	} else {
		err = m.backend.PinShell(row.Dir, row.Command)
		list.Add(row.Dir, row.Command, m.now())
		m.shellHint = "pinned: it now shows in the palette as tmp"
	}
	if err != nil {
		m.shellHint = "pin failed: " + err.Error()
		return
	}
	m.backend.Scratch = list.Entries
	m.shellCursor = moveCursor(m.shellCursor, 0, len(m.shellRows()))
	m.loadCommands()
	m.updateFilteredCommands()
}

// startShellPromote opens the name prompt for saving the row under the cursor
// to .pltrc, prefilled with a suggested name.
func (m *Model) startShellPromote() {
	row, ok := m.currentShellRow()
	if !ok {
		return
	}
	if m.backend.PromoteShell == nil {
		m.shellHint = "saving to .pltrc is unavailable here"
		return
	}
	m.shellPromote = row
	m.shellHint = ""
	m.mode = modeShellPromote
	m.saveInput.SetValue(suggestShellName(row.Command))
	m.saveInput.CursorEnd()
	m.saveInput.Focus()
}

func (m Model) updateShellPromoteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return m, m.confirmShellPromote()
	case tea.KeyEscape:
		m.mode = modeShellPick
		m.saveInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

// confirmShellPromote saves the command to .pltrc under the name typed, drops
// it from the pinned list (it is a real command now), and reloads the config
// so it appears in the palette. The picker stays open to confirm the save.
func (m *Model) confirmShellPromote() tea.Cmd {
	name := strings.TrimSpace(m.saveInput.Value())
	if name == "" {
		m.shellHint = "enter a name for the command"
		return nil
	}
	row := m.shellPromote
	m.mode = modeShellPick
	m.saveInput.Blur()
	if err := m.backend.PromoteShell(row.Dir, row.Command, name); err != nil {
		m.shellHint = "save failed: " + err.Error()
		return nil
	}
	if row.Pinned && m.backend.UnpinShell != nil && m.backend.UnpinShell(row.Dir, row.Command) == nil {
		list := scratch.List{Entries: append([]scratch.Entry(nil), m.backend.Scratch...)}
		list.Remove(row.Dir, row.Command)
		m.backend.Scratch = list.Entries
	}
	cmd := m.reloadConfig()
	m.loadCommands()
	m.updateFilteredCommands()
	m.shellCursor = moveCursor(m.shellCursor, 0, len(m.shellRows()))
	m.shellHint = fmt.Sprintf("saved %q to .pltrc", name)
	return cmd
}

// runnerWords are command prefixes that say how something runs rather than
// what it does, so a suggested name skips them (`pnpm cdk diff` -> cdk-diff).
var runnerWords = map[string]bool{
	"pnpm": true, "npm": true, "yarn": true, "npx": true, "bun": true, "bunx": true,
	"run": true, "exec": true, "dlx": true, "sudo": true, "env": true,
}

// suggestShellName proposes a .pltrc command name: up to three of the
// command's words that start with a letter, up to its first flag, skipping
// env assignments and runner words, joined with '-' and made alias-safe.
func suggestShellName(command string) string {
	var words []string
	for _, w := range strings.Fields(command) {
		if len(words) == 3 {
			break
		}
		if strings.HasPrefix(w, "-") {
			break // flags and their values are too specific for a name
		}
		if strings.Contains(w, "=") || runnerWords[w] {
			continue
		}
		r := []rune(w)
		if !unicode.IsLetter(r[0]) {
			continue
		}
		words = append(words, w)
	}
	if len(words) == 0 {
		return "command"
	}
	return config.SanitizeName(strings.Join(words, "-"), true)
}

// insertScratchRows adds a palette row per pinned temporary command, ahead of
// any tool rows (which always stay last). A row takes the display name of the
// location containing its directory, so it sorts and records like that
// location's commands; outside every location it shows the directory.
func (m *Model) insertScratchRows() {
	if len(m.backend.Scratch) == 0 {
		return
	}
	var rows []CommandInfo
	for _, e := range m.backend.Scratch {
		name := m.relDir(e.Dir)
		if loc, ok := m.config.LocationFor(e.Dir); ok {
			name = loc.DisplayName()
		}
		info := CommandInfo{
			Display:     name + ": " + e.Command,
			Directory:   e.Dir,
			Command:     e.Command,
			DisplayName: name,
			Type:        scratchType,
			IsScratch:   true,
		}
		if m.history != nil && m.frecencyEnabled {
			info.FrecencyScore = m.history.GetScore(name, e.Command)
		}
		rows = append(rows, info)
	}
	at := len(m.commands)
	for i, c := range m.commands {
		if c.IsTool {
			at = i
			break
		}
	}
	merged := append(append(append([]CommandInfo(nil), m.commands[:at]...), rows...), m.commands[at:]...)
	m.commands = merged
}

// relDir shows dir relative to the project root ("." for the root itself), or
// as is when it lies outside it.
func (m Model) relDir(dir string) string {
	if m.backend.RootDir == "" {
		return dir
	}
	rel, err := filepath.Rel(m.backend.RootDir, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return dir
	}
	return rel
}

func (m Model) renderShellPicker() string {
	rows := m.shellRows()
	var lines []string
	grouping := "by location"
	if m.shellByFolder {
		grouping = "by folder"
	}
	lines = append(lines, previewTitleStyle.Render("Recent shell commands · "+m.shellPeriod.Label()+" · "+grouping))
	lines = append(lines, statusStyle.Render("Commands typed in this project's shell. Pin the ones you keep re-running, or save them to .pltrc."))
	lines = append(lines, "")

	// Column widths: age and directory are padded so commands line up.
	whenW, dirW, countW := 0, 0, 0
	for _, r := range rows {
		whenW = max(whenW, len(r.When))
		countW = max(countW, utf8.RuneCountInString(shellCount(r)))
		dirW = max(dirW, len(m.relDir(r.Dir)))
	}

	var body []string
	cursorLine := 0
	for i, r := range rows {
		if i == 0 && r.Pinned {
			body = append(body, listLocationStyle.Render("Pinned"))
		}
		if !r.Pinned && (i == 0 || rows[i-1].Pinned) && i > 0 {
			body = append(body, listLocationStyle.Render("Recent"))
		}
		when := fmt.Sprintf("%-*s  %*s", whenW, r.When, countW, shellCount(r))
		dir := fmt.Sprintf("%-*s", dirW, m.relDir(r.Dir))
		command := strings.ReplaceAll(r.Command, "\n", " ⏎ ")
		row := statusStyle.Render(when) + "  " + listLocationStyle.Render(dir) + "  " + listCommandStyle.Render(command)
		if i == m.shellCursor {
			cursorLine = len(body)
			row = cursorLineStyle.Render(when + "  " + dir + "  " + command)
		}
		body = append(body, "  "+row)
	}
	if len(rows) == 0 {
		body = append(body, statusStyle.Render("  Nothing in "+m.shellPeriod.Label()+". Press ←/→ to widen the period."))
		body = append(body, statusStyle.Render("  Commands you type inside this project appear here when plt-integration.zsh is sourced in zsh."))
	}

	// Keep the cursor line in view; the rest of the screen is title, prompt and help.
	visible := m.height - 8
	if visible < 3 {
		visible = len(body)
	}
	start, end := visibleWindow(scrollToCursor(cursorLine, 0, visible), visible, len(body))
	lines = append(lines, body[start:end]...)

	lines = append(lines, "")
	if m.mode == modeShellPromote {
		lines = append(lines, "  "+m.saveInput.View())
		lines = append(lines, m.renderHelp([][2]string{{"Enter", "save to .pltrc"}, {"Esc", "back"}}))
	} else {
		if m.shellHint != "" {
			lines = append(lines, "  "+statusYellowStyle.Render(m.shellHint))
		}
		lines = append(lines, m.renderHelp([][2]string{
			{"Enter", "run"},
			{"p", "pin/unpin"},
			{"a", "add to .pltrc"},
			{"←/→", "period"},
			{"f", "by folder/location"},
			{"Esc", "back"},
		}))
	}
	if m.mode == modeShellPromote && m.shellHint != "" {
		lines = append(lines, "  "+statusYellowStyle.Render(m.shellHint))
	}
	return strings.Join(lines, "\n")
}

// shellCount renders a row's run count ("3×"), or nothing for a pinned row.
func shellCount(r shellRow) string {
	if r.Count == 0 {
		return ""
	}
	return fmt.Sprintf("%d×", r.Count)
}
