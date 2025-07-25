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

	// Enable message rendering
	watcher.RenderMessage = true

	err = watcher.SubscribeFromBeginning("Microsoft-Windows-Sysmon/Operational", "*")
	if err != nil {
		fmt.Printf("Couldn't subscribe to Sysmon: %v", err)
		// Try Application log as fallback
		err = watcher.SubscribeFromBeginning("Application", "*")
		if err != nil {
			fmt.Printf("Couldn't subscribe to Application: %v", err)
			return
		}
	}

	fmt.Println("Watching for events. Press Ctrl+C to exit.")
	fmt.Println("EventID | Provider | Message")
	fmt.Println("------------------------------------------")
	fmt.Println("Testing standard event message handling for all event types")

	for {
		select {
		case evt := <-watcher.Event():
			// Print the full message to verify our formatting changes
			fmt.Printf("%d | %s | %s\n\n", 
				evt.EventId, 
				evt.ProviderName,
				evt.Msg)
		case err := <-watcher.Error():
			fmt.Printf("Error: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}
