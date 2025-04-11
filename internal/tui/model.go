package tui

import (
	"context"
	"dotenv-manager/internal/parser"
	"dotenv-manager/internal/watcher"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Icons ---
const (
	iconCheckboxOff = "[ ]"
	iconCheckboxOn  = "[✓]"
	iconRadioOff    = " "
	iconRadioOn     = "*"
	iconPointer     = "> "
	iconEmptyValue  = "<empty>"
)

// Model represents the state of the TUI application.
type Model struct {
	parsedData *parser.ParsedData // The parsed .env file data
	filePath   string             // Path to the .env file being managed

	cursor     int // Current row index in the logical list (includes group headers and value lines)
	focusIndex int // Index of the currently focused VariableGroup in parsedData.GroupOrder

	// TUI rendering properties
	viewport viewport.Model // Used for scrolling the list
	width    int
	height   int

	styles Styles // Styling for different UI elements

	// State flags
	modified          bool // True if there are unsaved changes
	quitting          bool // True when the user has initiated quit sequence
	showQuitPrompt    bool // True when showing the "Save before quitting?" prompt
	quittingAfterSave bool // Set to true when quit is initiated via 'Save & Quit'

	statusMessage string // To display feedback like "Saved", "Error", etc.

	// Hot Reload state
	watcher             *watcher.Watcher
	watcherCtx          context.Context    // Context for managing watcher lifecycle
	watcherCancel       context.CancelFunc // Function to cancel the context
	showReloadPrompt    bool               // True when showing "File changed externally..." prompt
	pendingReloadAction func() tea.Msg     // Action to take after reload prompt (reload or keep)
}

// Styles defines the lipgloss styles used in the TUI.
type Styles struct {
	NormalLine      lipgloss.Style
	FocusedLine     lipgloss.Style
	DisabledLine    lipgloss.Style
	EmptyValueStyle lipgloss.Style // Style for <empty> placeholder
	KeyStyle        lipgloss.Style // Style for variable keys
	HeaderTitle     lipgloss.Style
	HeaderFileInfo  lipgloss.Style
	Header          lipgloss.Style
	Footer          lipgloss.Style
	ModifiedStatus  lipgloss.Style
	StatusMessage   lipgloss.Style
	ErrorMessage    lipgloss.Style
	PromptStyle     lipgloss.Style
}

// DefaultStyles creates a default set of styles.
func DefaultStyles() Styles {
	// Dracula color palette
	var (
		// draculaBackground  = lipgloss.AdaptiveColor{Light: "#282a36", Dark: "#282a36"} // Not directly used for base, but good reference
		draculaForeground = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#f8f8f2"}
		draculaComment    = lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#6272a4"}
		// draculaCyan         = lipgloss.AdaptiveColor{Light: "#8be9fd", Dark: "#8be9fd"}
		draculaGreen  = lipgloss.AdaptiveColor{Light: "#50fa7b", Dark: "#50fa7b"}
		draculaOrange = lipgloss.AdaptiveColor{Light: "#ffb86c", Dark: "#ffb86c"}
		draculaPink   = lipgloss.AdaptiveColor{Light: "#ff79c7", Dark: "#ff79c7"}
		draculaPurple = lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#bd93f9"}
		draculaRed    = lipgloss.AdaptiveColor{Light: "#ff5555", Dark: "#ff5555"}
		draculaYellow = lipgloss.AdaptiveColor{Light: "#f1fa8c", Dark: "#f1fa8c"}
	)

	// Base styles using Dracula colors
	base := lipgloss.NewStyle().Foreground(draculaForeground) // Use Foreground as the base text color

	return Styles{
		NormalLine:   base,                                    // Use base directly
		FocusedLine:  base.Foreground(draculaPink).Bold(true), // Bright FG on CurrentLine BG
		DisabledLine: base.Foreground(draculaComment),         // Comment color for disabled

		// Style for '<empty>' value placeholder
		EmptyValueStyle: base.Foreground(draculaYellow), // Yellow for empty values

		HeaderTitle: lipgloss.NewStyle().
			Foreground(draculaPurple).
			Padding(0, 1).
			Bold(true),
		HeaderFileInfo: lipgloss.NewStyle().
			Foreground(draculaComment).
			Padding(0, 1),
		Header: lipgloss.NewStyle().
			Padding(0, 0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(draculaComment). // Comment color for footer
			MarginTop(1),

		ModifiedStatus: lipgloss.NewStyle().Foreground(draculaOrange).Bold(true), // Orange for modified
		StatusMessage:  lipgloss.NewStyle().Foreground(draculaGreen),             // Green for success/status
		ErrorMessage:   lipgloss.NewStyle().Foreground(draculaRed).Bold(true),    // Red for errors
		PromptStyle:    lipgloss.NewStyle().Foreground(draculaPink).Bold(true),   // Pink for prompts

		KeyStyle: base.Bold(true), // Keep Key style bold with base foreground
	}
}

// InitialModel creates the initial model for the Bubble Tea program.
func InitialModel(filePath string, pd *parser.ParsedData, w *watcher.Watcher) Model {
	// Create a cancellable context for the watcher
	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		parsedData:        pd,
		filePath:          filePath,
		cursor:            0,
		focusIndex:        0,
		styles:            DefaultStyles(),
		modified:          false,
		quitting:          false,
		showQuitPrompt:    false,
		quittingAfterSave: false,
		statusMessage:     "",
		watcher:           w,
		watcherCtx:        ctx,
		watcherCancel:     cancel,
		showReloadPrompt:  false,
		// Viewport initialized in first Update with WindowSizeMsg
	}
}

// Init is the first command ran by the Bubble Tea program.
func (m Model) Init() tea.Cmd {
	if m.watcher != nil {
		// Start the watcher in a goroutine
		m.watcher.Start(m.watcherCtx, m.filePath)
		// Return the command to listen for watcher events
		return m.watcher.WatchFileCmd()
	}
	return nil
}
