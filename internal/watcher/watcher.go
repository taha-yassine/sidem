package watcher

import (
	"context"
	"fmt"

	// "log" // Removed for TUI cleanliness
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// FileChangedMsg is sent when the watched file is modified.
type FileChangedMsg struct{}

// WatcherErrMsg is sent when the watcher encounters an error.
type WatcherErrMsg struct {
	err error
}

func (e WatcherErrMsg) Error() string {
	return e.err.Error()
}

// Watcher manages the file system watcher.
type Watcher struct {
	watcher *fsnotify.Watcher
	Events  chan tea.Msg // Channel to send messages back to Bubble Tea
	Errors  chan error   // Channel to send errors (raw errors)
}

// New creates a new Watcher.
func New() (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}
	return &Watcher{
		watcher: fsWatcher,
		Events:  make(chan tea.Msg),
		Errors:  make(chan error),
	}, nil
}

// Start begins watching the specified file.
// It runs in a goroutine and sends events/errors on the respective channels.
func (w *Watcher) Start(ctx context.Context, filePath string) {
	go func() {
		defer close(w.Events)
		defer close(w.Errors)
		defer w.watcher.Close()

		err := w.watcher.Add(filePath)
		if err != nil {
			// Send error directly, let main loop format if needed
			w.Errors <- fmt.Errorf("failed to add file %s to watcher: %w", filePath, err)
			return
		}

		var debounceTimer *time.Timer
		debounceDuration := 500 * time.Millisecond

		for {
			select {
			case <-ctx.Done():
				// log.Println("Watcher: Context done, stopping watcher.")
				return

			case event, ok := <-w.watcher.Events:
				if !ok {
					// log.Println("Watcher: Watcher events channel closed.")
					return
				}

				if event.Has(fsnotify.Write) && event.Name == filePath {
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(debounceDuration, func() {
						// log.Printf("Watcher: Detected write event for %s", event.Name)
						w.Events <- FileChangedMsg{}
					})
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					// log.Println("Watcher: Watcher errors channel closed.")
					return
				}
				// log.Printf("Watcher: Received error: %v", err)
				// Propagate the raw error
				w.Errors <- err
			}
		}
	}()
	// log.Printf("Watcher: Started watching %s", filePath)
}

// WatchFileCmd returns a command that listens for watcher events.
func (w *Watcher) WatchFileCmd() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-w.Events:
			if !ok {
				return nil // Channel closed
			}
			return msg
		case err, ok := <-w.Errors:
			if !ok {
				return nil // Channel closed
			}
			// Convert watcher error to a specific Bubble Tea message
			return WatcherErrMsg{err} // Use the raw error
		}
	}
}
