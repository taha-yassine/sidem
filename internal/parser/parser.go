package parser

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// LineType defines the type of a line in the .env file.
type LineType int

const (
	LineTypeBlank LineType = iota
	LineTypeComment
	LineTypeVariable
)

// Line represents a single line from the .env file.
type Line struct {
	OriginalContent string   // The raw line content as read from the file.
	Type            LineType // Type of the line (Blank, Comment, Variable).
	LineNumber      int      // Original 1-based line number.

	// Fields specific to Variable lines
	Key            string // Variable name (e.g., "DATABASE_URL").
	Value          string // Variable value (e.g., "postgres://...").
	IsCommentedOut bool   // True if the variable line starts with '#'.
}

// VariableGroup holds all occurrences of a variable with the same key.
type VariableGroup struct {
	Key               string  // The variable name.
	Lines             []*Line // Pointers to the original Line objects in ParsedData.Lines.
	IsActive          bool    // Master toggle state for the TUI ([x] / [ ]).
	ActiveLineIdx     int     // Index within Lines pointing to the currently active value (if IsActive is true). -1 if inactive.
	LastActiveLineIdx int     // Index of the last selected value before becoming inactive.
}

// ParsedData holds the complete parsed information from the .env file.
type ParsedData struct {
	Lines          []*Line                   // All lines in their original order.
	VariableGroups map[string]*VariableGroup // Variables grouped by key.
	GroupOrder     []string                  // Order in which variable groups should be displayed.
}

// variableRegex matches potential variable lines (commented or uncommented).
// It captures: 1=(#?), 2=(KEY), 3=(VALUE)
// Allows optional whitespace around '#' and '='.
// TODO: Allow comment at the end
var variableRegex = regexp.MustCompile(`^\s*(#)?\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)

// ParseFile reads and parses the specified .env file.
func ParseFile(filePath string) (*ParsedData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()

	parsedData := &ParsedData{
		Lines:          []*Line{},
		VariableGroups: make(map[string]*VariableGroup),
		GroupOrder:     []string{},
	}
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		originalLine := scanner.Text()
		trimmedLine := strings.TrimSpace(originalLine)

		line := &Line{
			OriginalContent: originalLine,
			LineNumber:      lineNumber,
		}

		if trimmedLine == "" {
			line.Type = LineTypeBlank
		} else if matches := variableRegex.FindStringSubmatch(originalLine); len(matches) == 4 {
			// It's a variable line
			line.Type = LineTypeVariable
			line.IsCommentedOut = matches[1] == "#"
			line.Key = matches[2]
			// Trim potential quotes from the value, but keep internal whitespace
			line.Value = strings.Trim(matches[3], ` "'`) // Handle simple cases

			if _, ok := parsedData.VariableGroups[line.Key]; !ok {
				parsedData.VariableGroups[line.Key] = &VariableGroup{
					Key:               line.Key,
					Lines:             []*Line{},
					IsActive:          false, // Determined later
					ActiveLineIdx:     -1,    // Determined later
					LastActiveLineIdx: 0,     // Default to first option if initially inactive
				}
				parsedData.GroupOrder = append(parsedData.GroupOrder, line.Key)
			}
			group := parsedData.VariableGroups[line.Key]
			group.Lines = append(group.Lines, line)

		} else if strings.HasPrefix(trimmedLine, "#") {
			line.Type = LineTypeComment
		} else {
			// Treat other non-empty, non-comment, non-variable lines as comments for now
			// Or potentially log a warning about malformed lines
			// For simplicity, let's treat them as comments that should be preserved.
			line.Type = LineTypeComment
			// Consider adding a specific 'Malformed' type later if needed.
		}

		parsedData.Lines = append(parsedData.Lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	// Determine initial active state for each group
	determineInitialActiveStates(parsedData.VariableGroups)

	return parsedData, nil
}

// determineInitialActiveStates sets the initial IsActive, ActiveLineIdx, and LastActiveLineIdx.
// A group is active if exactly one of its lines is not commented out.
// If multiple are uncommented, the first uncommented one becomes active (MVP simplification).
// If none are uncommented, the group is inactive.
func determineInitialActiveStates(groups map[string]*VariableGroup) {
	for _, group := range groups {
		firstUncommentedIdx := -1
		firstVarIdx := -1
		uncommentedCount := 0

		for i, line := range group.Lines {
			if line.Type == LineTypeVariable {
				if firstVarIdx == -1 {
					firstVarIdx = i
				}
				if !line.IsCommentedOut {
					uncommentedCount++
					if firstUncommentedIdx == -1 {
						firstUncommentedIdx = i
					}
				}
			}
		}

		if uncommentedCount > 0 {
			group.IsActive = true
			group.ActiveLineIdx = firstUncommentedIdx
			group.LastActiveLineIdx = firstUncommentedIdx // Remember the initial active one
			if uncommentedCount > 1 {
				// Optional: Log a warning here if multiple lines for the same key are uncommented initially.
				// fmt.Printf("Warning: Multiple uncommented lines found for key '%s'. Activating the first one.\n", group.Key)
			}
		} else {
			group.IsActive = false
			group.ActiveLineIdx = -1
			// Default last active to the first variable line if any, otherwise -1
			if firstVarIdx != -1 {
				group.LastActiveLineIdx = firstVarIdx
			} else {
				group.LastActiveLineIdx = -1 // No variable lines in this group
			}
		}
	}
}

// Helper function (optional) to print parsed data for debugging
func (pd *ParsedData) PrintDebug() {
	fmt.Println("--- All Lines ---")
	for _, l := range pd.Lines {
		typeStr := ""
		switch l.Type {
		case LineTypeBlank:
			typeStr = "Blank"
		case LineTypeComment:
			typeStr = "Comment"
		case LineTypeVariable:
			typeStr = "Variable"
		}
		fmt.Printf("L%d [%s]: %s", l.LineNumber, typeStr, l.OriginalContent)
		if l.Type == LineTypeVariable {
			fmt.Printf(" (Key: %s, Val: %s, Commented: %t)", l.Key, l.Value, l.IsCommentedOut)
		}
		fmt.Println()
	}

	fmt.Println("\n--- Variable Groups (Order:", pd.GroupOrder, ") ---")
	for _, key := range pd.GroupOrder {
		g := pd.VariableGroups[key]
		fmt.Printf("Group: %s (Active: %t, ActiveIdx: %d, LastActiveIdx: %d)\n", g.Key, g.IsActive, g.ActiveLineIdx, g.LastActiveLineIdx)
		for i, l := range g.Lines {
			activeMarker := " "
			if g.IsActive && i == g.ActiveLineIdx {
				activeMarker = "*"
			}
			fmt.Printf("  %s [%d] L%d: %s\n", activeMarker, i, l.LineNumber, l.OriginalContent)
		}
	}
}
