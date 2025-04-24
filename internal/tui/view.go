package tui

import (
	"fmt"
	"strings"

	"sidem/internal/parser"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View renders the TUI based on the model state.
func (m Model) View() string {
	if m.quitting {
		// If quitting, show final status message if any, then clear
		if m.statusMessage != "" {
			finalMsg := m.statusMessage
			return finalMsg + "\n"
		}
		return ""
	}
	if m.width == 0 {
		return "Initializing..."
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// Combine header, viewport, and footer
	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

// renderHeader renders the top header bar.
func (m *Model) renderHeader() string { // Pointer receiver for consistency
	version := "v0.1.0" // TODO: Get version from build
	title := fmt.Sprintf("sidem %s", version)
	filePath := m.filePath
	modifiedStatus := ""
	if m.modified {
		modifiedStatus = m.styles.ModifiedStatus.Render(" [MODIFIED]")
	}

	fileInfo := fmt.Sprintf("%s%s", filePath, modifiedStatus)
	titleWidth := lipgloss.Width(title)
	fileInfoWidth := lipgloss.Width(fileInfo)

	spaces := max(0, m.width-titleWidth-fileInfoWidth-m.styles.HeaderTitle.GetHorizontalPadding()-m.styles.HeaderFileInfo.GetHorizontalPadding())

	header := fmt.Sprintf("%s%s%s", m.styles.HeaderTitle.Render(title), strings.Repeat(" ", spaces), m.styles.HeaderFileInfo.Render(fileInfo))

	return m.styles.Header.Width(m.width).Render(header)
}

// renderFooter renders the bottom help/status bar.
func (m *Model) renderFooter() string { // Pointer receiver for consistency
	help := "↑/↓/j/k: Navigate | Space: Toggle/Select | y: Copy | Ctrl+S: Save | q/Ctrl+C: Quit"
	quitPrompt := "Save changes before quitting? ([Y]es/[N]o/[C]ancel)"
	reloadPrompt := "File changed externally. [R]eload (lose TUI changes) / [K]eep TUI changes?"

	var content string
	var style lipgloss.Style = m.styles.Footer // Default style

	if m.showQuitPrompt {
		content = m.styles.PromptStyle.Render(quitPrompt)
	} else if m.showReloadPrompt {
		content = m.styles.PromptStyle.Render(reloadPrompt)
	} else if m.statusMessage != "" {
		// Display status message instead of help when present
		if strings.HasPrefix(m.statusMessage, "Error:") {
			content = m.styles.ErrorMessage.Render(m.statusMessage)
		} else {
			content = m.styles.StatusMessage.Render(m.statusMessage)
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
		var prefixIcon string
		var prefixIconStyle, textStyle lipgloss.Style

		// Determine correct prefix icon
		if item.isGroupHeader {
			prefixIcon = iconCheckboxOff
			if item.isSelected {
				prefixIcon = iconCheckboxOn
			}
			prefixIcon += " "
		} else {
			prefixIcon = iconRadioOff
			if item.isSelected {
				prefixIcon = iconRadioOn
			}
			prefixIcon = fmt.Sprintf("	%s ", prefixIcon)
		}

		if i == m.cursor {
			// Focused
			pointer = m.styles.FocusedLine.Render(iconPointer)
			prefixIconStyle = m.styles.FocusedLine
			textStyle = m.styles.FocusedLine
		} else {
			// Non-focused
			if item.isDisabled {
				prefixIconStyle = m.styles.DisabledLine
				textStyle = m.styles.DisabledLine
				if item.isEmptyValue {
					textStyle = m.styles.EmptyValueStyle.Faint(true)
				}
			} else {
				prefixIconStyle = m.styles.SelectedIcon
				textStyle = m.styles.NormalLine
				if item.isEmptyValue {
					textStyle = m.styles.EmptyValueStyle
				}
			}
		}

		var lineContent strings.Builder
		lineContent.WriteString(pointer)

		lineContent.WriteString(prefixIconStyle.Render(prefixIcon))

		// Render key or value
		var content string
		if item.isGroupHeader {
			content = item.key
		} else {
			if item.isEmptyValue {
				content = iconEmptyValue
			} else {
				content = item.value
			}
		}
		lineContent.WriteString(textStyle.Render(content))

		// Truncate line if it's too long
		// TODO: Implement proper wrapping
		truncatedLine := ansi.Truncate(lineContent.String(), m.width, "…")

		builder.WriteString(truncatedLine)
		builder.WriteString("\n")
	}

	finalStr := builder.String()

	// Remove the last newline
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
	isSelected bool

	// Header specific
	isGroupHeader bool
	key           string

	// Value specific
	value        string
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
		items = append(items, ListItem{
			key:           group.Key,
			isDisabled:    !group.IsSelected,
			isGroupHeader: true,
			groupIndex:    groupIdx,
			valueIndex:    -1,
			isSelected:    group.IsSelected, // Mirrors isDisabled
		})

		// Value Lines
		if len(group.Lines) > 0 {
			for valueIdx, line := range group.Lines {
				if line.Type == parser.LineTypeVariable {
					items = append(items, ListItem{
						value:         line.Value,
						isDisabled:    !group.IsSelected,
						isEmptyValue:  line.Value == "",
						isGroupHeader: false,
						groupIndex:    groupIdx,
						valueIndex:    valueIdx,
						isSelected:    group.SelectedLineIdx == valueIdx,
					})
				}
			}
		}
	}
	return items
}
