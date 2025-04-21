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
	err = watcher.SubscribeFromBeginning("Application", "*")
	if err != nil {
		fmt.Printf("Couldn't subscribe to Application: %v", err)
	}
	
	fmt.Println("Watching for events. Press Ctrl+C to exit.")
	
	count := 0
	for {
		select {
		case evt := <-watcher.Event():
			fmt.Printf("Event ID: %d\n", evt.EventId)
			fmt.Printf("Message: %s\n", evt.Msg)
			fmt.Printf("Level: %s\n", evt.LevelText)
			fmt.Printf("Provider: %s\n", evt.ProviderText)
			fmt.Printf("Channel: %s\n", evt.ChannelText)
			fmt.Printf("Bookmark: %v\n\n", evt.Bookmark)
			
			count++
			if count >= 10 {
				fmt.Println("Received 10 events. Exiting.")
				return
			}
		case err := <-watcher.Error():
			fmt.Printf("Error: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}