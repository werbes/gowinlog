package main

import (
	"fmt"
	"sync"
	"time"

	winlog "github.com/ofcoursedude/gowinlog"
)

func main() {
	fmt.Println("Starting concurrent usage example...")
	
	// Create a WinLogWatcher
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Printf("Couldn't create watcher: %v\n", err)
		return
	}
	
	// Subscribe to the Application channel
	err = watcher.SubscribeFromNow("Application", "*")
	if err != nil {
		fmt.Printf("Couldn't subscribe to Application channel: %v\n", err)
		return
	}
	
	// Create a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	
	// Start multiple goroutines to process events concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Process events for 10 seconds
			timeout := time.After(10 * time.Second)
			for {
				select {
				case evt := <-watcher.Event():
					if evt != nil {
						fmt.Printf("Goroutine %d received event: %s: %s: %s\n", 
							id, evt.LevelText, evt.ProviderName, evt.Msg)
					}
				case err := <-watcher.Error():
					fmt.Printf("Goroutine %d received error: %v\n", id, err)
				case <-timeout:
					fmt.Printf("Goroutine %d timeout\n", id)
					return
				default:
					// If no event is waiting, need to wait or do something else
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to finish
	wg.Wait()
	
	// Shutdown the watcher
	watcher.Shutdown()
	fmt.Println("Concurrent usage example completed")
}