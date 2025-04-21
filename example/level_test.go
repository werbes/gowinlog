//go:build windows
// +build windows

package main

import (
	"fmt"
	"time"

	winlog "github.com/werbes/gowinlog"
)

func main() {
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Printf("Couldn't create watcher: %v\n", err)
		return
	}
	
	// Enable level rendering
	watcher.RenderLevel = true
	
	err = watcher.SubscribeFromBeginning("Application", "*")
	if err != nil {
		fmt.Printf("Couldn't subscribe to Application: %v", err)
		return
	}
	
	fmt.Println("Watching for events. Press Ctrl+C to exit.")
	fmt.Println("Level | LevelText | EventID | Message")
	fmt.Println("------------------------------------------")
	
	for {
		select {
		case evt := <-watcher.Event():
			// Print just the level information to verify our changes
			fmt.Printf("%d | %s | %d | %s\n", 
				evt.Level, 
				evt.LevelText, 
				evt.EventId,
				evt.Msg[:min(len(evt.Msg), 50)]) // Truncate message to 50 chars
		case err := <-watcher.Error():
			fmt.Printf("Error: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}