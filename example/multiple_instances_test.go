//go:build windows
// +build windows

package main

import (
	"fmt"
	"sync"
	"time"

	winlog "github.com/ofcoursedude/gowinlog"
)

// This test creates multiple WinLogWatcher instances and runs them in parallel
// to verify that the library is stable when running multiple parallel instances
// in the same application.
func main() {
	fmt.Println("Starting multiple instances test...")

	// Number of parallel instances to create
	numInstances := 5

	// Create a WaitGroup to wait for all instances to finish
	var wg sync.WaitGroup
	wg.Add(numInstances)

	// Create and run multiple instances in parallel
	for i := 0; i < numInstances; i++ {
		go func(instanceID int) {
			defer wg.Done()

			// Create a new watcher instance
			watcher, err := winlog.NewWinLogWatcher()
			if err != nil {
				fmt.Printf("Instance %d: Couldn't create watcher: %v\n", instanceID, err)
				return
			}

			// Subscribe to different channels based on instance ID
			var channel string
			switch instanceID % 3 {
			case 0:
				channel = "Application"
			case 1:
				channel = "System"
			case 2:
				channel = "Security"
			}

			// Subscribe to the channel
			err = watcher.SubscribeFromNow(channel, "*")
			if err != nil {
				fmt.Printf("Instance %d: Couldn't subscribe to %s channel: %v\n", instanceID, channel, err)
				watcher.Shutdown()
				return
			}

			fmt.Printf("Instance %d: Subscribed to %s channel\n", instanceID, channel)

			// Process events for 30 seconds
			timeout := time.After(30 * time.Second)
			eventCount := 0
			errorCount := 0

			for {
				select {
				case evt := <-watcher.Event():
					if evt != nil {
						eventCount++
						if eventCount % 10 == 0 {
							fmt.Printf("Instance %d: Received %d events from %s channel\n", 
								instanceID, eventCount, channel)
						}
					}
				case err := <-watcher.Error():
					errorCount++
					fmt.Printf("Instance %d: Received error: %v\n", instanceID, err)
				case <-timeout:
					fmt.Printf("Instance %d: Timeout after processing %d events and %d errors\n", 
						instanceID, eventCount, errorCount)
					
					// Shutdown the watcher
					watcher.Shutdown()
					return
				default:
					// If no event is waiting, need to wait or do something else
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}

	// Wait for all instances to finish
	wg.Wait()

	fmt.Println("Multiple instances test completed successfully")
}