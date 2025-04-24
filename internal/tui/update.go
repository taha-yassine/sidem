package tui

import (
	"fmt"
	"strings"
	"time"

	"sidem/internal/parser"
	"sidem/internal/watcher"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Custom Message Types (errMsg, saveSuccessMsg defined in actions.go) ---

type clearStatusMsg struct{ originalMsg string }
type confirmedReloadMsg struct{}
type fileReloadedMsg struct {
	parsedData *parser.ParsedData
}

// --- Update Function ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		headerHeight := lipgloss.Height(m.renderHeader())
		footerHeight := lipgloss.Height(m.renderFooter())
		if m.viewport.Width == 0 || m.viewport.Height == 0 {
			m.viewport = viewport.New(m.width, m.height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = m.height - headerHeight - footerHeight
		}
		m.updateViewportContent()
		m.ensureCursorVisible()

	case saveSuccessMsg:
		m.modified = false
		if m.quittingAfterSave {
			m.quitting = true
			m.quittingAfterSave = false
			m.statusMessage = "Saved successfully! Quitting..."
			if m.watcherCancel != nil {
				m.watcherCancel()
			}
			return m, tea.Quit
		}
		m.statusMessage = "Saved successfully!"
		cmd = tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{originalMsg: "Saved successfully!"}
		})
		cmds = append(cmds, cmd)

	case errMsg:
		m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		m.quittingAfterSave = false
		m.showQuitPrompt = false
		m.showReloadPrompt = false

	case clearStatusMsg:
		if m.statusMessage == msg.originalMsg {
			m.statusMessage = ""
		}

	case watcher.FileChangedMsg:
		if m.modified {
			m.showReloadPrompt = true
			m.pendingReloadAction = func() tea.Msg { return confirmedReloadMsg{} }
			m.statusMessage = ""
		} else {
			m.statusMessage = "File changed, reloading..."
			cmd = m.reloadFileCmd()
			cmds = append(cmds, cmd)
		}
		if m.watcher != nil {
			cmds = append(cmds, m.watcher.WatchFileCmd())
		}

	case watcher.WatcherErrMsg:
		m.statusMessage = fmt.Sprintf("Watcher Error: %v", msg.Error())
		if m.watcher != nil {
			cmds = append(cmds, m.watcher.WatchFileCmd())
		}

	case confirmedReloadMsg:
		m.statusMessage = "Reloading..."
		m.showReloadPrompt = false
		m.modified = false
		cmd = m.reloadFileCmd()
		cmds = append(cmds, cmd)

	case fileReloadedMsg:
		m.parsedData = msg.parsedData
		m.modified = false
		m.cursor = 0
		m.focusIndex = 0
		m.statusMessage = "File reloaded successfully."
		m.updateViewportContent()
		m.ensureCursorVisible()
		cmd = tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{originalMsg: "File reloaded successfully."}
		})
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if m.statusMessage != "" && !strings.HasPrefix(m.statusMessage, "Error:") {
			m.statusMessage = ""
		}

		if m.showQuitPrompt {
			return m.handleQuitPrompt(msg)
		}
		if m.showReloadPrompt {
			return m.handleReloadPrompt(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.modified {
				m.showQuitPrompt = true
				return m, nil
			}
			m.quitting = true
			if m.watcherCancel != nil {
				m.watcherCancel()
			}
			return m, tea.Quit

		case "up", "k":
			m = m.moveUp()
		case "down", "j":
			m = m.moveDown()

		case " ": // Spacebar
			var changed bool
			m, changed = m.toggleSelection()
			if changed {
				m.modified = true
			}

		case "ctrl+s":
			if m.modified {
				m.statusMessage = "Saving..."
				cmd = m.saveCmd()
				cmds = append(cmds, cmd)
			} else {
				m.statusMessage = "No changes to save."
				cmd = tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return clearStatusMsg{originalMsg: "No changes to save."}
				})
				cmds = append(cmds, cmd)
			}

		case "y": // Copy selected line content
			textToCopy := m.getSelectedLineContent()
			if textToCopy != "" {
				err := clipboard.WriteAll(textToCopy)
				if err != nil {
					m.statusMessage = fmt.Sprintf("Error copying: %v", err)
				} else {
					m.statusMessage = "Copied to clipboard!"
					cmd = tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
						return clearStatusMsg{originalMsg: "Copied to clipboard!"}
					})
					cmds = append(cmds, cmd)
				}
			} else {
				m.statusMessage = "The selected line is empty."
				cmd = tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return clearStatusMsg{originalMsg: "The selected line is empty."}
				})
				cmds = append(cmds, cmd)
			}
		}
	}

	m.updateViewportContent()

	return m, tea.Batch(cmds...)
}

// --- Helper functions for Update --- (Will be expanded)

// getCurrentListItems is a helper to get the dynamically generated list.
func (m *Model) getCurrentListItems() []ListItem {
	return m.buildListItems()
}

// moveUp moves the cursor up, handling wrapping and viewport.
func (m Model) moveUp() Model {
	if m.cursor > 0 {
		m.cursor--
		m.ensureCursorVisible()
	}
	return m
}

// moveDown moves the cursor down, handling wrapping and viewport.
func (m Model) moveDown() Model {
	listItems := m.getCurrentListItems()
	listLen := len(listItems)

	if m.cursor < listLen-1 {
		m.cursor++
		m.ensureCursorVisible()
	}
	return m
}

// ensureCursorVisible adjusts the viewport's YOffset to keep the cursor visible.
func (m *Model) ensureCursorVisible() {
	listItems := m.getCurrentListItems()
	listLen := len(listItems)

	if m.cursor < 0 {
		m.cursor = 0
	} else if m.cursor >= listLen {
		m.cursor = listLen - 1
	}

	// Viewport readiness is handled by initialization check
	if listLen == 0 /* || !m.viewport.Ready() */ {
		return
	}

	scrollOff := 2
	minVisible := m.viewport.YOffset
	maxVisible := m.viewport.YOffset + m.viewport.Height - 1

	if m.cursor < minVisible+scrollOff {
		m.viewport.SetYOffset(max(0, m.cursor-scrollOff))
	} else if m.cursor > maxVisible-scrollOff {
		m.viewport.SetYOffset(min(listLen-m.viewport.Height, m.cursor-m.viewport.Height+1+scrollOff))
	}

	if m.cursor >= 0 && m.cursor < listLen {
		m.focusIndex = listItems[m.cursor].groupIndex
	}
}

// toggleSelection handles the spacebar press to toggle group activity or select a value.
func (m Model) toggleSelection() (Model, bool) {
	listItems := m.getCurrentListItems()
	if m.cursor < 0 || m.cursor >= len(listItems) || m.parsedData == nil {
		return m, false
	}

	selectedItem := listItems[m.cursor]
	if selectedItem.groupIndex < 0 || selectedItem.groupIndex >= len(m.parsedData.GroupOrder) {
		return m, false
	}
	groupKey := m.parsedData.GroupOrder[selectedItem.groupIndex]
	group, ok := m.parsedData.VariableGroups[groupKey]
	if !ok {
		return m, false
	}

	if selectedItem.isGroupHeader {
		// --- Toggle Group Header --- //
		group.IsSelected = !group.IsSelected

		return m, true // State changed
	} else {
		// --- Select Value Line --- //
		if selectedItem.valueIndex < 0 || selectedItem.valueIndex >= len(group.Lines) {
			return m, false // Invalid value index
		}

		if group.IsSelected {
			// Group is ACTIVE: Select this value if it's not already the active one
			if group.SelectedLineIdx != selectedItem.valueIndex {
				group.SelectedLineIdx = selectedItem.valueIndex
				return m, true // State changed
			}
		} else {
			// Group is INACTIVE: Activate the group AND select this value
			group.IsSelected = true
			group.SelectedLineIdx = selectedItem.valueIndex
			return m, true // State changed
		}
	}

	return m, false // No change
}

// updateViewportContent prepares the content string for the viewport.
func (m *Model) updateViewportContent() {
	// Viewport readiness is handled by initialization check
	// if !m.viewport.Ready() {
	// 	 return
	// }
	listContent := m.renderList() // This now uses the model's current state
	m.viewport.SetContent(listContent)
}

// handleQuitPrompt handles key presses when the quit confirmation is shown.
func (m Model) handleQuitPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.statusMessage = "Saving..."
		m.quittingAfterSave = true
		return m, m.saveCmd()
	case "n", "N":
		m.quitting = true
		if m.watcherCancel != nil {
			m.watcherCancel()
		}
		return m, tea.Quit
	case "c", "C", "esc":
		m.showQuitPrompt = false
		m.quittingAfterSave = false
		m.statusMessage = ""
		return m, nil
	}
	// Ignore other keys when prompt is active
	return m, nil
}

// handleReloadPrompt handles key presses when the reload confirmation is shown.
func (m Model) handleReloadPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) { // Case-insensitive
	case "r": // Reload (lose changes)
		if m.pendingReloadAction != nil {
			// Execute the stored action (which sends confirmedReloadMsg)
			cmd := m.pendingReloadAction
			m.pendingReloadAction = nil // Clear the pending action
			m.showReloadPrompt = false
			return m, cmd
		} else {
			// Should not happen, but reset state if it does
			m.showReloadPrompt = false
			m.statusMessage = "Error: No reload action pending."
			return m, nil
		}
	case "k": // Keep TUI changes (ignore file change for now)
		m.showReloadPrompt = false
		m.pendingReloadAction = nil
		m.statusMessage = "Kept local changes. File change ignored."
		// Re-queue the watcher command to listen for the *next* change
		var cmd tea.Cmd
		if m.watcher != nil {
			cmd = m.watcher.WatchFileCmd()
		}
		return m, cmd
	case "esc": // Same as keep
		m.showReloadPrompt = false
		m.pendingReloadAction = nil
		m.statusMessage = "Kept local changes. File change ignored."
		var cmd tea.Cmd
		if m.watcher != nil {
			cmd = m.watcher.WatchFileCmd()
		}
		return m, cmd
	}
	return m, nil // Ignore other keys
}

// reloadFileCmd creates a command to re-parse the file and update the model.
func (m Model) reloadFileCmd() tea.Cmd {
	return func() tea.Msg {
		pd, err := parser.ParseFile(m.filePath)
		if err != nil {
			return errMsg{fmt.Errorf("failed to reload file: %w", err)}
		}
		// Return new parsed data in a message (or update model directly?)
		// Let's create a new message type for this.
		return fileReloadedMsg{parsedData: pd}
	}
}

// Custom min/max removed, using built-in Go 1.21+ versions.

// saveCmd is defined in actions.go

func (m *Model) getSelectedLineContent() string {
	listItems := m.getCurrentListItems()

	selectedItem := listItems[m.cursor]
	if selectedItem.isGroupHeader {
		return selectedItem.key
	}
	return selectedItem.value
}
