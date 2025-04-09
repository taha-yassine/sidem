package main

import (
	"fmt"
	"log"
	"os"

	"dotenv-manager/internal/parser"
	"dotenv-manager/internal/tui"
	"dotenv-manager/internal/watcher"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dotenv-manager [dotenv-file]",
	Short: "A TUI application to manage .env files",
	Long: `dotenv-manager provides a terminal user interface
for viewing, editing, and managing variables within a .env file.

If [dotenv-file] is not provided, it defaults to '.env' in the current directory.`,
	Args:                  cobra.MaximumNArgs(1), // Allow 0 or 1 argument
	Run:                   runApplication,
	DisableFlagsInUseLine: true,
}

func runApplication(cmd *cobra.Command, args []string) {
	// 1. Determine the target .env file path
	filePath := ".env" // Default
	if len(args) > 0 {
		filePath = args[0] // Use the provided argument
	}

	// Configure logging (optional, useful for watcher debugging)
	// log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 2. Check if the file exists before parsing
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File not found at %s\n", filePath)
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	// 3. Parse the .env file
	parsedData, err := parser.ParseFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	// Optional: Print debug info if needed
	// parsedData.PrintDebug()

	// 4. Create the watcher
	w, err := watcher.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file watcher: %v\n", err)
		os.Exit(1)
	}
	// Defer closing resources isn't straightforward with Bubble Tea managing the loop.
	// The watcher context will be cancelled in the TUI model's quit handling.

	// 5. Initialize the Bubble Tea model
	initialModel := tui.InitialModel(filePath, parsedData, w)

	// 6. Create and run the Bubble Tea program
	p := tea.NewProgram(initialModel, tea.WithAltScreen(), tea.WithMouseCellMotion()) // Enable AltScreen and mouse

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	// Exit successfully
	fmt.Println("dotenv-manager exited.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}
