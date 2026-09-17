package ui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette (truecolor). Forced on via
// lipgloss.SetColorProfile(termenv.TrueColor) in Run(), so these hex colors
// render even though the shell wrapper captures stdout as a pipe.
const (
	ccBase     = "#1e1e2e"
	ccSurface0 = "#313244"
	ccOverlay0 = "#6c7086"
	ccText     = "#cdd6f4"
	ccLavender = "#b4befe"
	ccBlue     = "#89b4fa"
	ccGreen    = "#a6e3a1"
	ccYellow   = "#f9e2af"
	ccPeach    = "#fab387"
)

var (
	searchPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
	selectedMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Bold(true)
	// cursorLineStyle is the selected-row highlight shared by the focus picker,
	// queue editor, and init wizard: a plain surface fill (no inner styling, so
	// the background never gets punched out by ANSI resets).
	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(ccSurface0)).
			Foreground(lipgloss.Color(ccText)).
			Bold(true)
	// Selected-row segments for the main palette list, which supports the
	// lavender accent bar and per-character fuzzy-match highlighting. Every
	// segment carries the surface background so no gaps appear between them.
	selBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Background(lipgloss.Color(ccBase))
	selBaseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	selHlStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	selBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	// Checked (queued) rows that are not under the cursor: a subtle surface fill
	// with lavender accent text and a green position badge, so checked commands
	// stand out from the list without competing with the cursor row's accent bar.
	// Matches highlight in bright text so they still pop against the lavender base.
	queuedBaseStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Background(lipgloss.Color(ccSurface0))
	queuedHlStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	queuedBadgeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	previewBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ccOverlay0))
	previewLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccBlue))
	previewValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(ccOverlay0)).Faint(true)
	statusGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Bold(true)
	statusYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccYellow)).Bold(true)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color(ccOverlay0)).Faint(true)
	helpKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
	editPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ccPeach)).Bold(true)
	listLocationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccOverlay0)).Faint(true)
	listCommandStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText))
	previewTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
	// matchStyle highlights fuzzy-matched characters in list rows.
	matchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
)
