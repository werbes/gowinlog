//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/werbes/gowinlog"
)

func main() {
	// Create a new event log watcher using the standard API
	// This should now use the WMI-based implementation internally
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Println("Error creating watcher:", err)
		return
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Subscribe to the Application event log
	err = watcher.SubscribeFromNow("Application", "*")
	if err != nil {
		fmt.Println("Error subscribing to Application log:", err)
		return
	}

	// Subscribe to the System event log
	err = watcher.SubscribeFromNow("System", "*")
	if err != nil {
		fmt.Println("Error subscribing to System log:", err)
		return
	}

	// Try to subscribe using a bookmark (should fall back to SubscribeFromBeginning)
	err = watcher.SubscribeFromBookmark("Security", "*", "<Bookmark><BookmarkList/></Bookmark>")
	if err != nil {
		fmt.Println("Error subscribing to Security log with bookmark:", err)
		// Continue anyway, as this is expected to fail but fall back
	}

	fmt.Println("Watching for events. Press Ctrl+C to exit.")
	fmt.Println("This test verifies that the standard API now uses WMI internally.")

	// Process events and errors
	go func() {
		for {
			select {
			case event := <-watcher.Event():
				if event != nil {
					fmt.Printf("[%s] %s: %s (ID: %s, Level: %s)\n",
						event.Created.Format(time.RFC3339),
						event.ProviderName,
						event.Msg,
						event.IdText,
						event.LevelText)
				}
			case err := <-watcher.Error():
				if err != nil {
					fmt.Println("Error:", err)
				}
			case <-time.After(100 * time.Millisecond):
				// Prevent tight loop
			}
		}
	}()

	// Wait for signal
	<-sigChan
	fmt.Println("Shutting down...")
	watcher.Shutdown()
}