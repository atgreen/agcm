package styles

import "github.com/charmbracelet/lipgloss"

// ColorScheme holds colors for a theme
type ColorScheme struct {
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Accent      lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
	Muted       lipgloss.Color
	Background  lipgloss.Color
	Foreground  lipgloss.Color
	Border      lipgloss.Color
	Highlight   lipgloss.Color
	Severity1   lipgloss.Color // Critical
	Severity2   lipgloss.Color // High
	Severity3   lipgloss.Color // Normal
	Severity4   lipgloss.Color // Low
}

// DarkColors returns the dark theme color scheme
func DarkColors() ColorScheme {
	return ColorScheme{
		Primary:    lipgloss.Color("#7C3AED"), // Purple
		Secondary:  lipgloss.Color("#06B6D4"), // Cyan
		Accent:     lipgloss.Color("#F59E0B"), // Amber
		Success:    lipgloss.Color("#10B981"), // Emerald
		Warning:    lipgloss.Color("#F59E0B"), // Amber
		Error:      lipgloss.Color("#EF4444"), // Red
		Muted:      lipgloss.Color("#9CA3AF"), // Gray (lighter for dark bg)
		Background: lipgloss.Color("#1F2937"), // Dark gray
		Foreground: lipgloss.Color("#F9FAFB"), // Light gray
		Border:     lipgloss.Color("#4B5563"), // Medium gray
		Highlight:  lipgloss.Color("#3B82F6"), // Blue
		Severity1:  lipgloss.Color("#EF4444"), // Red
		Severity2:  lipgloss.Color("#F59E0B"), // Amber
		Severity3:  lipgloss.Color("#3B82F6"), // Blue
		Severity4:  lipgloss.Color("#9CA3AF"), // Gray
	}
}

// LightColors returns the light theme color scheme
func LightColors() ColorScheme {
	return ColorScheme{
		Primary:    lipgloss.Color("#6D28D9"), // Purple (darker for light bg)
		Secondary:  lipgloss.Color("#0891B2"), // Cyan (darker)
		Accent:     lipgloss.Color("#D97706"), // Amber (darker)
		Success:    lipgloss.Color("#059669"), // Emerald (darker)
		Warning:    lipgloss.Color("#D97706"), // Amber (darker)
		Error:      lipgloss.Color("#DC2626"), // Red (darker)
		Muted:      lipgloss.Color("#6B7280"), // Gray
		Background: lipgloss.Color("#FFFFFF"), // White
		Foreground: lipgloss.Color("#111827"), // Near black
		Border:     lipgloss.Color("#D1D5DB"), // Light gray
		Highlight:  lipgloss.Color("#2563EB"), // Blue (darker)
		Severity1:  lipgloss.Color("#DC2626"), // Red
		Severity2:  lipgloss.Color("#D97706"), // Amber
		Severity3:  lipgloss.Color("#2563EB"), // Blue
		Severity4:  lipgloss.Color("#6B7280"), // Gray
	}
}

// Legacy color variables for backwards compatibility
var (
	ColorPrimary     = lipgloss.Color("#7C3AED")
	ColorSecondary   = lipgloss.Color("#06B6D4")
	ColorAccent      = lipgloss.Color("#F59E0B")
	ColorSuccess     = lipgloss.Color("#10B981")
	ColorWarning     = lipgloss.Color("#F59E0B")
	ColorError       = lipgloss.Color("#EF4444")
	ColorMuted       = lipgloss.Color("#6B7280")
	ColorBackground  = lipgloss.Color("#1F2937")
	ColorForeground  = lipgloss.Color("#F9FAFB")
	ColorBorder      = lipgloss.Color("#374151")
	ColorHighlight   = lipgloss.Color("#3B82F6")
	ColorSeverity1   = lipgloss.Color("#EF4444")
	ColorSeverity2   = lipgloss.Color("#F59E0B")
	ColorSeverity3   = lipgloss.Color("#3B82F6")
	ColorSeverity4   = lipgloss.Color("#6B7280")
)

// Styles contains all the application styles
type Styles struct {
	App           lipgloss.Style
	Header        lipgloss.Style
	Footer        lipgloss.Style
	Sidebar       lipgloss.Style
	Content       lipgloss.Style
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Label         lipgloss.Style
	Value         lipgloss.Style
	Muted         lipgloss.Style
	Selected      lipgloss.Style
	Focused       lipgloss.Style
	Border        lipgloss.Style
	StatusBar     lipgloss.Style
	HelpKey       lipgloss.Style
	HelpDesc      lipgloss.Style
	Error         lipgloss.Style
	Success       lipgloss.Style
	Warning       lipgloss.Style
	ListItem      lipgloss.Style
	ListItemSelected lipgloss.Style
	CaseNumber    lipgloss.Style
	Severity1     lipgloss.Style
	Severity2     lipgloss.Style
	Severity3     lipgloss.Style
	Severity4     lipgloss.Style
	StatusOpen    lipgloss.Style
	StatusClosed  lipgloss.Style
	StatusWaiting lipgloss.Style
	Comment       lipgloss.Style
	CommentAuthor lipgloss.Style
	CommentDate   lipgloss.Style
	Attachment    lipgloss.Style
}

// DefaultStyles returns styles based on auto-detected terminal background
func DefaultStyles() *Styles {
	if lipgloss.HasDarkBackground() {
		return NewStyles(DarkColors())
	}
	return NewStyles(LightColors())
}

// DarkStyles returns explicit dark theme styles
func DarkStyles() *Styles {
	return NewStyles(DarkColors())
}

// LightStyles returns explicit light theme styles
func LightStyles() *Styles {
	return NewStyles(LightColors())
}

// NewStyles creates styles from a color scheme
func NewStyles(c ColorScheme) *Styles {
	return &Styles{
		App: lipgloss.NewStyle(),
		// Don't set background - let terminal handle it

		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(c.Primary).
			Bold(true).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(c.Muted).
			Padding(0, 1),

		Sidebar: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.Border).
			Padding(0, 1),

		Content: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.Border).
			Padding(0, 1),

		Title: lipgloss.NewStyle().
			Foreground(c.Foreground).
			Bold(true),

		Subtitle: lipgloss.NewStyle().
			Foreground(c.Secondary),

		Label: lipgloss.NewStyle().
			Foreground(c.Muted),

		Value: lipgloss.NewStyle().
			Foreground(c.Foreground),

		Muted: lipgloss.NewStyle().
			Foreground(c.Muted),

		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(c.Highlight).
			Bold(true),

		Focused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.Primary),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.Border),

		StatusBar: lipgloss.NewStyle().
			Foreground(c.Muted),

		HelpKey: lipgloss.NewStyle().
			Foreground(c.Secondary).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(c.Muted),

		Error: lipgloss.NewStyle().
			Foreground(c.Error),

		Success: lipgloss.NewStyle().
			Foreground(c.Success),

		Warning: lipgloss.NewStyle().
			Foreground(c.Warning),

		ListItem: lipgloss.NewStyle().
			Foreground(c.Foreground),

		ListItemSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(c.Highlight).
			Bold(true),

		CaseNumber: lipgloss.NewStyle().
			Foreground(c.Secondary).
			Bold(true),

		Severity1: lipgloss.NewStyle().
			Foreground(c.Severity1).
			Bold(true),

		Severity2: lipgloss.NewStyle().
			Foreground(c.Severity2).
			Bold(true),

		Severity3: lipgloss.NewStyle().
			Foreground(c.Severity3),

		Severity4: lipgloss.NewStyle().
			Foreground(c.Severity4),

		StatusOpen: lipgloss.NewStyle().
			Foreground(c.Success),

		StatusClosed: lipgloss.NewStyle().
			Foreground(c.Muted),

		StatusWaiting: lipgloss.NewStyle().
			Foreground(c.Warning),

		Comment: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), false, false, false, true).
			BorderForeground(c.Border).
			PaddingLeft(1).
			MarginBottom(1),

		CommentAuthor: lipgloss.NewStyle().
			Foreground(c.Secondary).
			Bold(true),

		CommentDate: lipgloss.NewStyle().
			Foreground(c.Muted).
			Italic(true),

		Attachment: lipgloss.NewStyle().
			Foreground(c.Accent),
	}
}

// SeverityStyle returns the appropriate style for a severity level
func (s *Styles) SeverityStyle(severity string) lipgloss.Style {
	switch severity {
	case "1 (Urgent)", "1":
		return s.Severity1
	case "2 (High)", "2":
		return s.Severity2
	case "3 (Normal)", "3":
		return s.Severity3
	default:
		return s.Severity4
	}
}

// StatusStyle returns the appropriate style for a case status
func (s *Styles) StatusStyle(status string) lipgloss.Style {
	switch status {
	case "Closed":
		return s.StatusClosed
	case "Waiting on Red Hat", "Waiting on Customer":
		return s.StatusWaiting
	default:
		return s.StatusOpen
	}
}
