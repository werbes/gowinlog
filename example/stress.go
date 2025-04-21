//go:build windows
// +build windows

package main

import (
	"fmt"
	"sync"
	"time"

	winlog "github.com/werbes/gowinlog"
)

// This test script attempts to reproduce the race condition by subscribing to
// multiple event logs in parallel and processing events concurrently.
func main() {
	fmt.Println("Starting stress test...")

	// Create a watcher
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Printf("Couldn't create watcher: %v\n", err)
		return
	}

	// Define channels to subscribe to
	channels := []string{
		"Application",
		"System",
		"Security",
		"Setup",
		"Windows PowerShell",
	}

	// Subscribe to all channels
	for _, channel := range channels {
		err := watcher.SubscribeFromBeginning(channel, "*")
		if err != nil {
			fmt.Printf("Couldn't subscribe to %s: %v\n", channel, err)
		} else {
			fmt.Printf("Subscribed to %s\n", channel)
		}
	}

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	wg.Add(2) // One for events, one for errors

	// Process events in a separate goroutine
	go func() {
		defer wg.Done()
		count := 0
		maxEvents := 1000 // Process up to 1000 events
		
		for count < maxEvents {
			select {
			case evt := <-watcher.Event():
				if evt == nil {
					fmt.Println("Received nil event")
					continue
				}
				
				fmt.Printf("Event ID: %d, Channel: %s, Provider: %s\n", 
					evt.EventId, evt.Channel, evt.ProviderName)
				
				// Print the message to verify it's working correctly
				if evt.Msg != "" {
					fmt.Printf("Message: %s\n\n", evt.Msg)
				} else {
					fmt.Printf("No message available\n\n")
				}
				
				count++
				
				// Introduce a small delay to allow other goroutines to run
				time.Sleep(1 * time.Millisecond)
				
			case <-time.After(100 * time.Millisecond):
				// If no event is waiting, need to wait or do something else
			}
		}
		
		fmt.Printf("Processed %d events. Test completed successfully.\n", count)
	}()

	// Process errors in a separate goroutine
	go func() {
		defer wg.Done()
		for {
			select {
			case err := <-watcher.Error():
				fmt.Printf("Error: %v\n", err)
			case <-time.After(5 * time.Second):
				// Exit after 5 seconds of no errors
				return
			}
		}
	}()

	// Wait for both goroutines to finish
	wg.Wait()

	// Shutdown the watcher
	watcher.Shutdown()
	fmt.Println("Stress test completed.")
}