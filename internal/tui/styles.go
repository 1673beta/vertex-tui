package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds all Lip Gloss styles used by the Vertex AI Coding Agent TUI.
type Styles struct {
	HeaderBar       lipgloss.Style
	HeaderTitle     lipgloss.Style
	HeaderBadge     lipgloss.Style
	ModelBadge      lipgloss.Style
	StatusConnected lipgloss.Style
	StatusWarn      lipgloss.Style

	UserLabel      lipgloss.Style
	ModelLabel     lipgloss.Style
	SystemLabel    lipgloss.Style
	ToolLabel      lipgloss.Style
	ThinkingStyle  lipgloss.Style
	ErrorStyle     lipgloss.Style
	DimStyle       lipgloss.Style
	HighlightStyle lipgloss.Style

	InputBoxFocused lipgloss.Style
	InputBoxBlurred lipgloss.Style
	SuggestionBar   lipgloss.Style

	ModalBox          lipgloss.Style
	ModalTitle        lipgloss.Style
	ModalItem         lipgloss.Style
	ModalItemSelected lipgloss.Style
	ModalActiveBadge  lipgloss.Style
	ModalDesc         lipgloss.Style

	FooterBar lipgloss.Style
}

// DefaultStyles returns the color palette and styling for the Vertex AI TUI.
func DefaultStyles() Styles {
	return Styles{
		HeaderBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#1A1B26")).
			Foreground(lipgloss.Color("#C0CAF5")).
			Padding(0, 1),
		HeaderTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7")),
		HeaderBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ECE6A")).
			Bold(true),
		ModelBadge: lipgloss.NewStyle().
			Background(lipgloss.Color("#3B4261")).
			Foreground(lipgloss.Color("#7DCFFF")).
			Bold(true).
			Padding(0, 1),
		StatusConnected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ECE6A")).
			Bold(true),
		StatusWarn: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0AF68")).
			Bold(true),

		UserLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AA2F7")).
			Bold(true),
		ModelLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BB9AF7")).
			Bold(true),
		SystemLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7DCFFF")).
			Bold(true),
		ToolLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0AF68")).
			Bold(true),
		ThinkingStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565F89")).
			Italic(true),
		ErrorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F7768E")).
			Bold(true),
		DimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565F89")),
		HighlightStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7DCFFF")).
			Bold(true),

		InputBoxFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7AA2F7")).
			Padding(0, 1),
		InputBoxBlurred: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3B4261")).
			Padding(0, 1),
		SuggestionBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A9B1D6")).
			Background(lipgloss.Color("#1F2335")).
			Padding(0, 1),

		ModalBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#7DCFFF")).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7DCFFF")).
			Bold(true),
		ModalItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C0CAF5")).
			PaddingLeft(2),
		ModalItemSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AA2F7")).
			Bold(true),
		ModalActiveBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ECE6A")).
			Bold(true),
		ModalDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565F89")).
			PaddingLeft(4),

		FooterBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565F89")).
			Padding(0, 1),
	}
}
