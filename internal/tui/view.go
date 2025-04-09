package tui

import (
	"fmt"
	"strings"

	"dotenv-manager/internal/parser"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI based on the model state.
func (m Model) View() string {
	if m.quitting {
		// If quitting, show final status message if any, then clear
		if m.statusMessage != "" {
			finalMsg := m.statusMessage
			// m.statusMessage = "" // Model is copied, cannot modify here
			return finalMsg + "\n"
		}
		return ""
	}
	// Viewport readiness check removed, handled by initialization
	// if m.width == 0 || !m.viewport.Ready() {
	if m.width == 0 {
		return "Initializing..."
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// Viewport content is updated in Update loop via updateViewportContent
	// which calls m.renderList()

	// Combine header, viewport, and footer
	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

// renderHeader renders the top header bar.
func (m *Model) renderHeader() string { // Pointer receiver for consistency
	title := "dotenv-manager"
	filePath := m.filePath
	modifiedStatus := ""
	if m.modified {
		modifiedStatus = m.styles.ModifiedStatus.Render(" [MODIFIED]")
	}

	fileInfo := fmt.Sprintf("File: %s%s", filePath, modifiedStatus)
	titleWidth := lipgloss.Width(title)
	fileInfoWidth := lipgloss.Width(fileInfo)

	spaces := max(0, m.width-titleWidth-fileInfoWidth-m.styles.Header.GetHorizontalPadding())

	headerStr := fmt.Sprintf("%s%s%s", title, strings.Repeat(" ", spaces), fileInfo)

	return m.styles.Header.Width(m.width).Render(headerStr)
}

// renderFooter renders the bottom help/status bar.
func (m *Model) renderFooter() string { // Pointer receiver for consistency
	help := "↑/↓/j/k: Navigate | Space: Toggle/Select | Ctrl+S: Save | q/Ctrl+C: Quit"
	quitPrompt := "Save changes before quitting? ([Y]es/[N]o/[C]ancel)"
	reloadPrompt := "File changed externally. [R]eload (lose TUI changes) / [K]eep TUI changes?"

	var content string
	var style lipgloss.Style = m.styles.Footer // Default style

	if m.showQuitPrompt {
		content = quitPrompt
		style = m.styles.PromptStyle
	} else if m.showReloadPrompt {
		content = reloadPrompt
		style = m.styles.PromptStyle
	} else if m.statusMessage != "" {
		// Display status message instead of help when present
		if strings.HasPrefix(m.statusMessage, "Error:") {
			content = m.statusMessage
			style = m.styles.ErrorMessage
		} else {
			content = m.statusMessage
			style = m.styles.StatusMessage
		}
	} else {
		content = help
	}

	// TODO: Add hot reload prompt display

	return style.Width(m.width).Render(content)
}

// renderList generates the string content for the scrollable list view.
func (m *Model) renderList() string {
	var builder strings.Builder
	listItems := m.buildListItems()

	for i, item := range listItems {
		pointer := "  "
		lineStyle := m.styles.NormalLine
		prefixIconStyle := lineStyle  // Style for checkbox/radio icon
		valueStyle := lineStyle       // Style for value text
		keyStyle := m.styles.KeyStyle // Base key style

		if i == m.cursor {
			pointer = m.styles.FocusedLine.Render(iconPointer)
			lineStyle = m.styles.FocusedLine
			prefixIconStyle = lineStyle // Focused icon takes focus style
			valueStyle = lineStyle
			keyStyle = keyStyle.Inherit(lineStyle) // Inherit focus fg/bg for key
		} else {
			// Non-focused styles
			if item.isDisabled {
				lineStyle = m.styles.DisabledLine
				prefixIconStyle = lineStyle
				valueStyle = lineStyle
				keyStyle = keyStyle.Inherit(lineStyle)
				if item.isEmptyValue {
					valueStyle = m.styles.EmptyValueStyle.Copy().Faint(true)
				}
			} else {
				// Active but not focused
				lineStyle = m.styles.NormalLine
				prefixIconStyle = lineStyle
				valueStyle = lineStyle
				keyStyle = keyStyle.Inherit(lineStyle)
				if item.isEmptyValue {
					valueStyle = m.styles.EmptyValueStyle
				}
			}
		}

		// Apply specific color to "on" icons if not disabled
		if !item.isDisabled {
			if (item.isGroupHeader && item.isActive) || (!item.isGroupHeader && item.isActive) {
				// If it's the checkbox/radio for an active state, color it green
				prefixIconStyle = prefixIconStyle.Copy().Foreground(m.styles.StatusMessage.GetForeground())
			}
		}

		var lineContent strings.Builder
		lineContent.WriteString(pointer)

		if item.isGroupHeader {
			lineContent.WriteString(prefixIconStyle.Render(item.prefix))
			lineContent.WriteString(keyStyle.Render(item.key))
		} else {
			lineContent.WriteString(prefixIconStyle.Render(item.prefix))
			lineContent.WriteString(valueStyle.Render(item.valueDisplay))
		}

		builder.WriteString(lineContent.String())
		builder.WriteString("\n")
	}

	finalStr := builder.String()
	if len(finalStr) > 0 {
		finalStr = finalStr[:len(finalStr)-1]
	}
	return finalStr
}

// ListItem represents a single renderable line in the TUI list.
type ListItem struct {
	// Common
	isDisabled bool
	groupIndex int
	valueIndex int
	isActive   bool // Is this the active checkbox/radio?

	// Header specific
	isGroupHeader bool
	prefix        string // Checkbox
	key           string

	// Value specific
	// prefix used here too (Radio)
	valueDisplay string
	isEmptyValue bool
}

// buildListItems constructs the flat list of items to be displayed.
func (m *Model) buildListItems() []ListItem {
	items := []ListItem{}
	if m.parsedData == nil {
		return items
	}

	for groupIdx, key := range m.parsedData.GroupOrder {
		group := m.parsedData.VariableGroups[key]

		// Group Header
		checkboxMarker := iconCheckboxOff // Default icon
		if group.IsActive {
			checkboxMarker = iconCheckboxOn
		}
		headerPrefix := checkboxMarker + " " // Prefix includes marker and space
		items = append(items, ListItem{
			prefix:        headerPrefix,
			key:           group.Key, // Key is separate from prefix
			isDisabled:    false,
			isGroupHeader: true,
			groupIndex:    groupIdx,
			valueIndex:    -1,
			isActive:      group.IsActive, // Is the group active?
		})

		// Value Lines
		if len(group.Lines) > 0 {
			valuesDisabled := !group.IsActive // Values are disabled if group is inactive
			checkedIndex := -1                // The index that should display (*)

			if group.IsActive {
				checkedIndex = group.ActiveLineIdx
			} else {
				// If inactive, the last active line should show (*), but disabled
				checkedIndex = group.LastActiveLineIdx
				// Validate LastActiveLineIdx is actually a variable in this group
				if checkedIndex < 0 || checkedIndex >= len(group.Lines) || group.Lines[checkedIndex].Type != parser.LineTypeVariable {
					checkedIndex = -1 // Reset if invalid
				}
			}

			for valueIdx, line := range group.Lines {
				if line.Type == parser.LineTypeVariable {
					radioMarker := iconRadioOff // Default icon
					if valueIdx == checkedIndex {
						radioMarker = iconRadioOn // Use filled icon if this is the checked one
					}
					valuePrefix := fmt.Sprintf("   %s ", radioMarker) // Indent + marker + space

					// Handle display for empty values
					isEmpty := line.Value == ""
					valDisplay := line.Value
					if isEmpty {
						valDisplay = iconEmptyValue
					}

					items = append(items, ListItem{
						prefix:        valuePrefix,
						valueDisplay:  valDisplay, // Display value (or placeholder)
						isDisabled:    valuesDisabled,
						isEmptyValue:  isEmpty,
						isGroupHeader: false,
						groupIndex:    groupIdx,
						valueIndex:    valueIdx,
						isActive:      group.IsActive && group.ActiveLineIdx == valueIdx, // Is this the *currently* active value?
					})
				}
			}
		}
	}
	return items
}

// Helper for max(int, int)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
