package parser

import (
	"bufio"
	"errors"
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
// TODO: Refactor into a tree structure for simplicity
type VariableGroup struct {
	Key             string  // The variable name.
	Lines           []*Line // Pointers to the original Line objects in ParsedData.Lines.
	IsSelected      bool    // Represents group selection state (checkbox). Group IsSelected equivalent.
	SelectedLineIdx int     // Index within Lines pointing to the currently selected value. Holds last selection if IsSelected is false.
}

// ParsedData holds the complete parsed information from the .env file.
type ParsedData struct {
	Lines          []*Line                   // All lines in their original order.
	VariableGroups map[string]*VariableGroup // Variables grouped by key.
	GroupOrder     []string                  // Order in which variable groups should be displayed.
}

// variableRegex matches potential variable lines (commented or uncommented).
// It captures:
// 1: Optional comment marker (#)
// 2: Key (either 'quoted' or unquoted)
// 3: The rest of the line after the '=' (value + optional inline comment)
// It handles optional 'export' prefix and spaces around '=', '#'.
var variableRegex = regexp.MustCompile(`^\s*(#)?\s*(?:export\s+)?('?[A-Za-z_][A-Za-z0-9_]*'?)\s*=\s*(.*)$`)

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
		// Keep trimmedLine for blank/comment checks, but parse originalLine for variables
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

			// Process Key (remove optional single quotes)
			keyRaw := matches[2]
			if len(keyRaw) >= 2 && keyRaw[0] == '\'' && keyRaw[len(keyRaw)-1] == '\'' {
				line.Key = keyRaw[1 : len(keyRaw)-1]
				// Basic validation: ensure key name is valid after removing quotes
				if !isValidKey(line.Key) {
					// Treat as a comment if the key is invalid after de-quoting
					// Or return an error, depending on desired strictness
					line.Type = LineTypeComment
					line.Key = "" // Clear invalid key
					parsedData.Lines = append(parsedData.Lines, line)
					continue // Skip variable processing
				}
			} else {
				line.Key = keyRaw
				if !isValidKey(line.Key) {
					// Treat as a comment if the key is invalid
					line.Type = LineTypeComment
					line.Key = "" // Clear invalid key
					parsedData.Lines = append(parsedData.Lines, line)
					continue // Skip variable processing
				}
			}

			// Process Value (handle quotes, escapes, inline comments)
			valueRaw, err := parseValueAndComment(matches[3])
			if err != nil {
				// Handle potential parsing errors (e.g., unterminated quotes)
				// Option 1: Treat as comment
				// line.Type = LineTypeComment
				// line.Key = "" // Clear key if value is bad? Or keep it?
				// Option 2: Return error
				return nil, fmt.Errorf("error parsing line %d: %w", lineNumber, err)
				// Option 3: Log warning and treat as comment (simplest for now)
				// fmt.Printf("Warning: Line %d parsing error: %v. Treating as comment.\n", lineNumber, err)
				// line.Type = LineTypeComment
				// line.Key = ""
			} else {
				line.Value = valueRaw
			}

			// If parsing resulted in treating it as a comment, skip group logic
			if line.Type == LineTypeComment {
				parsedData.Lines = append(parsedData.Lines, line)
				continue
			}

			// Add to VariableGroup
			if _, ok := parsedData.VariableGroups[line.Key]; !ok {
				parsedData.VariableGroups[line.Key] = &VariableGroup{
					Key:             line.Key,
					Lines:           []*Line{},
					IsSelected:      false, // Determined later
					SelectedLineIdx: -1,    // Determined later
				}
				parsedData.GroupOrder = append(parsedData.GroupOrder, line.Key)
			}
			group := parsedData.VariableGroups[line.Key]
			group.Lines = append(group.Lines, line)

		} else if strings.HasPrefix(trimmedLine, "#") {
			line.Type = LineTypeComment
		} else {
			// Treat other non-empty, non-comment, non-variable lines as comments
			line.Type = LineTypeComment
		}

		parsedData.Lines = append(parsedData.Lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	// Determine initial active state for each group
	determineInitialSelectedStates(parsedData.VariableGroups)

	return parsedData, nil
}

// isValidKey checks if a string is a valid unquoted key name.
var keyValidationRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isValidKey(key string) bool {
	return keyValidationRegex.MatchString(key)
}

// parseValueAndComment extracts the value from the rest of the line,
// handling quotes, escapes, and inline comments.
func parseValueAndComment(input string) (string, error) {
	input = strings.TrimLeft(input, " \t") // Trim leading space only

	if input == "" {
		return "", nil // Empty value
	}

	var valueRaw string
	var quoteType rune = 0 // 0 = unquoted, '\'' = single, '"' = double

	switch input[0] {
	case '\'':
		quoteType = '\''
		endQuoteIdx := -1
		escaped := false
		for i := 1; i < len(input); i++ {
			if input[i] == '\'' && !escaped {
				endQuoteIdx = i
				break
			}
			escaped = input[i] == '\\' && !escaped
		}
		if endQuoteIdx == -1 {
			return "", errors.New("unterminated single-quoted value")
		}
		valueRaw = input[1:endQuoteIdx]
		// Check for inline comment after closing quote
		// commentPart := strings.TrimSpace(input[endQuoteIdx+1:])
		// if len(commentPart) > 0 && !strings.HasPrefix(commentPart, "#") {
		// 	 return "", fmt.Errorf("unexpected characters after closing single quote: %s", commentPart)
		// }
	case '"':
		quoteType = '"'
		endQuoteIdx := -1
		escaped := false
		for i := 1; i < len(input); i++ {
			if input[i] == '"' && !escaped {
				endQuoteIdx = i
				break
			}
			escaped = input[i] == '\\' && !escaped
		}
		if endQuoteIdx == -1 {
			return "", errors.New("unterminated double-quoted value")
		}
		valueRaw = input[1:endQuoteIdx]
		// Check for inline comment after closing quote
		// commentPart := strings.TrimSpace(input[endQuoteIdx+1:])
		// if len(commentPart) > 0 && !strings.HasPrefix(commentPart, "#") {
		// 	return "", fmt.Errorf("unexpected characters after closing double quote: %s", commentPart)
		// }
	default:
		// Unquoted value: find the first " #"
		commentIdx := -1
		for i := 0; i < len(input); i++ {
			if input[i] == '#' && i > 0 && (input[i-1] == ' ' || input[i-1] == '\t') {
				// Found start of inline comment if # is preceded by whitespace
				commentIdx = i - 1 // Point to the space before #
				break
			}
		}

		if commentIdx != -1 {
			valueRaw = input[:commentIdx]
		} else {
			valueRaw = input
		}
		// Trim trailing whitespace from unquoted value *before* unescaping
		valueRaw = strings.TrimRight(valueRaw, " \t")
	}

	// return unescapeValue(valueRaw, quoteType)
	_ = quoteType // TODO: Remove in future
	return valueRaw, nil
}

// unescapeValue processes escape sequences based on the quoting style.
// func unescapeValue(raw string, quoteType rune) (string, error) {
// 	var sb strings.Builder
// 	sb.Grow(len(raw)) // Pre-allocate capacity
// 	escaped := false

// 	for _, r := range raw {
// 		if escaped {
// 			switch quoteType {
// 			case '\'': // Single quotes: \\ and \'
// 				switch r {
// 				case '\\', '\'':
// 					sb.WriteRune(r)
// 				default:
// 					// Invalid escape sequence for single quotes, keep literal backslash and char
// 					// Or return error? Let's keep literal for robustness.
// 					sb.WriteRune('\\')
// 					sb.WriteRune(r)
// 					// return "", fmt.Errorf("invalid escape sequence in single-quoted string: \\%c", r)
// 				}
// 			case '"': // Double quotes: \\, \', \"
// 				switch r {
// 				case '\\', '\'', '"':
// 					sb.WriteRune(r)
// 				// Add other common escapes if needed later (e.g., \n, \t), but not per user spec yet
// 				// case 'n': sb.WriteRune('\n')
// 				// case 'r': sb.WriteRune('\r')
// 				// case 't': sb.WriteRune('\t')
// 				default:
// 					// Invalid escape sequence for double quotes, keep literal backslash and char
// 					sb.WriteRune('\\')
// 					sb.WriteRune(r)
// 					// return "", fmt.Errorf("invalid escape sequence in double-quoted string: \\%c", r)
// 				}
// 			default: // Unquoted or error case - should not happen if called correctly
// 				// No escapes defined for unquoted values in the spec
// 				sb.WriteRune('\\')
// 				sb.WriteRune(r) // Treat as literal
// 			}
// 			escaped = false
// 		} else if r == '\\' && (quoteType == '\'' || quoteType == '"') {
// 			escaped = true // Potential start of an escape sequence only if quoted
// 		} else {
// 			sb.WriteRune(r)
// 			escaped = false // Ensure escaped is reset if it wasn't a backslash
// 		}
// 	}

// 	if escaped {
// 		// Trailing backslash - treat as error or literal?
// 		// Let's treat as literal for robustness (append the dangling backslash)
// 		sb.WriteRune('\\')
// 		// Alternatively, return an error:
// 		// return "", errors.New("value ends with a dangling escape character ('\\')")
// 	}

// 	return sb.String(), nil
// }

// determineInitialSelectedStates sets the initial IsSelected, SelectedLineIdx.
// A group is selected if exactly one of its lines is not commented out.
// If multiple are uncommented, the first uncommented one becomes selected (MVP simplification).
// If none are uncommented, the group is inactive, but SelectedLineIdx remembers the first var.
func determineInitialSelectedStates(groups map[string]*VariableGroup) {
	for _, group := range groups {
		firstUncommentedIdx := -1
		firstVarIdx := -1
		uncommentedCount := 0

		for i, line := range group.Lines {
			if line.Type == LineTypeVariable {
				if uncommentedCount > 1 {
					// All required info was gathered
					// Break early
					break
				}
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
			group.IsSelected = true
			group.SelectedLineIdx = firstUncommentedIdx
			if uncommentedCount > 1 {
				// Optional: Log a warning here if multiple lines for the same key are uncommented initially.
				fmt.Printf("Warning: Multiple uncommented lines found for key '%s'. Selecting the first one.\n", group.Key)
			}
		} else {
			group.IsSelected = false
			// Default active index to the first variable line if any, otherwise -1
			// This serves as the "memory" for when the group is reactivated.
			group.SelectedLineIdx = firstVarIdx
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
		fmt.Printf("Group: %s (Selected: %t, SelectedIdx: %d)\n", g.Key, g.IsSelected, g.SelectedLineIdx)
		for i, l := range g.Lines {
			activeMarker := " "
			if i == g.SelectedLineIdx { // Show marker even if inactive
				activeMarker = "*"
			}
			fmt.Printf("  %s [%d] L%d: %s\n", activeMarker, i, l.LineNumber, l.OriginalContent)
		}
	}
}
