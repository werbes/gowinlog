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
	
	err = watcher.SubscribeFromBeginning("Application", "*")
	if err != nil {
		fmt.Printf("Couldn't subscribe to Application: %v", err)
		return
	}
	
	fmt.Println("Watching for events. Press Ctrl+C to exit.")
	fmt.Println("EventID | Provider | Message")
	fmt.Println("------------------------------------------")
	
	for {
		select {
		case evt := <-watcher.Event():
			// Print just the message to verify our changes
			fmt.Printf("%d | %s | %s\n", 
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