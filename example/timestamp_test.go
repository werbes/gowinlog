//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	winlog "github.com/ofcoursedude/gowinlog"
)

func main() {
	fmt.Println("Starting timestamp test...")

	// Create a new event log watcher
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Println("Error creating watcher:", err)
		return
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Subscribe to the Application event log from 10 minutes ago
	startTime := time.Now().Add(-10 * time.Minute)
	fmt.Printf("Subscribing from timestamp: %s\n", startTime.Format(time.RFC3339))
	
	err = watcher.SubscribeFromTimestamp("Application", "*", startTime)
	if err != nil {
		fmt.Println("Error subscribing to Application log:", err)
		return
	}

	fmt.Println("Subscribed to Application log using timestamp-based filtering")
	fmt.Println("Watching for events. Press Ctrl+C to exit.")

	// Process events for 30 seconds
	timeout := time.After(30 * time.Second)
	eventCount := 0

	// Process events and errors
	for {
		select {
		case event := <-watcher.Event():
			if event != nil {
				eventCount++
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
		case <-timeout:
			fmt.Printf("Timeout after processing %d events\n", eventCount)
			
			// Get the last event timestamp
			lastTime, err := watcher.GetLastEventTime("Application")
			if err != nil {
				fmt.Println("Error getting last event time:", err)
			} else {
				fmt.Printf("Last event time: %s\n", lastTime.Format(time.RFC3339))
			}
			
			// Shutdown the watcher
			watcher.Shutdown()
			return
		case <-sigChan:
			fmt.Println("Received signal, shutting down...")
			
			// Get the last event timestamp
			lastTime, err := watcher.GetLastEventTime("Application")
			if err != nil {
				fmt.Println("Error getting last event time:", err)
			} else {
				fmt.Printf("Last event time: %s\n", lastTime.Format(time.RFC3339))
			}
			
			// Shutdown the watcher
			watcher.Shutdown()
			return
		default:
			// If no event is waiting, need to wait or do something else
			time.Sleep(100 * time.Millisecond)
		}
	}
}