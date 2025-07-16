package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/masamerc/sidem/internal/parser"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Messages for async operations (used within TUI package) ---

type saveSuccessMsg struct{}

type errMsg struct{ err error }

// Implement the error interface for errMsg
func (e errMsg) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "<nil>"
}

// --- Action Commands ---

// saveCmd creates a command to save the current state back to the file.
func (m Model) saveCmd() tea.Cmd {
	return func() tea.Msg {
		err := saveFile(m.filePath, m.parsedData)
		if err != nil {
			return errMsg{err}
		}
		return saveSuccessMsg{}
	}
}

// saveFile reconstructs and saves the .env file.
func saveFile(filePath string, data *parser.ParsedData) error {
	// 1. Create a backup
	backupPath := filePath + ".bak"
	if err := backupFile(filePath, backupPath); err != nil {
		// Non-fatal error, but log it or notify user?
		// For now, proceed even if backup fails, but return the backup error
		// return fmt.Errorf("failed to create backup %s: %w", backupPath, err)
		// Let's log it and continue
		fmt.Fprintf(os.Stderr, "Warning: Failed to create backup %s: %v\n", backupPath, err)
	}

	// 2. Prepare the new content
	var builder strings.Builder
	for _, line := range data.Lines {
		switch line.Type {
		case parser.LineTypeBlank, parser.LineTypeComment:
			builder.WriteString(line.OriginalContent)
			builder.WriteString("\n")
		case parser.LineTypeVariable:
			group, ok := data.VariableGroups[line.Key]
			if !ok {
				// Should not happen if parsing was correct, but handle defensively
				builder.WriteString("# Error: Orphaned variable line! -> " + line.OriginalContent)
				builder.WriteString("\n")
				continue
			}

			// Find the index of this specific line within its group
			lineIndexInGroup := -1
			for i, groupLine := range group.Lines {
				if groupLine == line { // Compare pointers
					lineIndexInGroup = i
					break
				}
			}

			if lineIndexInGroup == -1 {
				// Should also not happen
				builder.WriteString("# Error: Could not find line in its group! -> " + line.OriginalContent)
				builder.WriteString("\n")
				continue
			}

			newLineContent := reconstructVariableLine(line, group, lineIndexInGroup)
			builder.WriteString(newLineContent)
			builder.WriteString("\n")

		default:
			// Preserve unknown line types?
			builder.WriteString(line.OriginalContent)
			builder.WriteString("\n")
		}
	}

	// 3. Write the new content, overwriting the original file
	// Use WriteFile for atomicity (creates temp file, then renames)
	// Need to remove trailing newline potentially added by loop if last line wasn't blank
	content := builder.String()
	// Ensure file ends with a newline as per custom instructions
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	err := os.WriteFile(filePath, []byte(content), 0644) // Use default permissions
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}

	return nil
}

// reconstructVariableLine determines the correct content for a variable line based on TUI state.
func reconstructVariableLine(line *parser.Line, group *parser.VariableGroup, lineIndexInGroup int) string {
	// Reconstruct the original Key=Value part, removing any initial comment marker
	// We stored Key and Value separately, need original spacing/quoting?
	// Simplification: Assume standard KEY=VALUE format is okay for reconstruction.
	// Let's try to use OriginalContent and add/remove '#' carefully.

	originalContent := line.OriginalContent
	hasPrefix := strings.HasPrefix(strings.TrimSpace(originalContent), "#")

	shouldBeActive := group.IsSelected && group.SelectedLineIdx == lineIndexInGroup

	if shouldBeActive {
		// Needs to be uncommented
		if hasPrefix {
			// Find the first '#' and remove it and any space after it
			idx := strings.Index(originalContent, "#")
			if idx != -1 {
				prefix := originalContent[:idx]
				suffix := originalContent[idx+1:]
				// Remove leading space from suffix if present
				suffix = strings.TrimPrefix(suffix, " ")
				return prefix + suffix
			} else {
				// '#' wasn't found where expected? Return original.
				return originalContent
			}
		} else {
			// Already uncommented, return as is
			return originalContent
		}
	} else {
		// Needs to be commented out
		if hasPrefix {
			// Already commented, return as is
			return originalContent
		} else {
			// Add '# ' prefix, preserving original indentation
			trimmedPrefix := strings.TrimLeft(originalContent, " \t")
			indentation := originalContent[:len(originalContent)-len(trimmedPrefix)]
			return indentation + "# " + trimmedPrefix
		}
	}
}

// backupFile creates a backup of the source file.
func backupFile(src, dst string) error {
	// Check if source exists
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No source file, nothing to back up
		}
		return fmt.Errorf("failed to open source file %s for backup: %w", src, err)
	}
	defer in.Close()

	// Create destination file
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create backup file %s: %w", dst, err)
	}
	defer out.Close()

	// Copy contents using buffered I/O
	reader := bufio.NewReader(in)
	writer := bufio.NewWriter(out)

	_, err = reader.WriteTo(writer) // More efficient than io.Copy for bufio
	if err != nil {
		return fmt.Errorf("failed to copy content to backup file %s: %w", dst, err)
	}

	// Ensure buffer is flushed
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush backup file %s: %w", dst, err)
	}

	return nil
}
