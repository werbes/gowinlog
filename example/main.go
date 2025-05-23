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
	for {
		select {
		case evt := <-watcher.Event():
			fmt.Printf("Event ID: %d\n", evt.EventId)
			fmt.Printf("Message: %s\n", evt.Msg)
			fmt.Printf("Level: %s\n", evt.LevelText)
			fmt.Printf("Provider: %s\n", evt.ProviderText)
			fmt.Printf("Channel: %s\n", evt.ChannelText)
			bookmark := evt.Bookmark
			fmt.Printf("Bookmark: %v\n\n", bookmark)
		case err := <-watcher.Error():
			fmt.Printf("Error: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}
