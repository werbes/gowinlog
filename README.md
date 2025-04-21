# gowinlog

[![Go Build](https://github.com/ofcoursedude/gowinlog/actions/workflows/go.yml/badge.svg)](https://github.com/ofcoursedude/gowinlog/actions/workflows/go.yml)
[![CodeQL](https://github.com/ofcoursedude/gowinlog/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/ofcoursedude/gowinlog/actions/workflows/codeql-analysis.yml)

Go library for subscribing to the Windows Event Log. This library now provides two different approaches for accessing Windows event logs:

1. **Windows Event Log API** (original approach): Uses the Windows Event Log API directly through CGO.
2. **WMI-based approach** (new, more stable approach): Uses Windows Management Instrumentation (WMI) to access event logs.

## Why Two Approaches?

The original Windows Event Log API approach can sometimes encounter issues such as:

- "The data area passed to a system call is too small" errors when rendering bookmarks
- Crashes with exception 0xc0000005
- Shared violations when accessing event handles

The new WMI-based approach provides a more stable alternative that avoids these issues.

Godocs
=======

[![PkgGoDev](https://pkg.go.dev/badge/github.com/ofcoursedude/gowinlog)](https://pkg.go.dev/github.com/ofcoursedude/gowinlog)

Installation
=======

```bash
go get github.com/werbes/gowinlog
go mod tidy
```

Features
========

### Windows Event Log API (Original Approach)
- Includes wrapper for wevtapi.dll, and a high level API
- Supports bookmarks for resuming consumption
- Filter events using XPath expressions 
- Thread-safe operations for concurrent usage

### WMI-based Approach (New)
- More stable, less prone to crashes
- No issues with "data area too small" errors
- No shared violations when accessing event handles
- Simpler implementation with fewer dependencies
- Uses the Windows Management Instrumentation (WMI) API

Usage
=======

## Windows Event Log API (Original Approach)

``` Go
package main

import (
	"fmt"
	"time"

	winlog "github.com/werbes/gowinlog"
)

func main() {
	fmt.Println("Starting...")
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Printf("Couldn't create watcher: %v\n", err)
		return
	}
	// Recieve any future messages on the Application channel
	// "*" doesn't filter by any fields of the event
	watcher.SubscribeFromNow("Application", "*")
	for {
		select {
		case evt := <-watcher.Event():
			// Print the event struct
			// fmt.Printf("\nEvent: %v\n", evt)
			// or print basic output
			fmt.Printf("\n%s: %s: %s\n", evt.LevelText, evt.ProviderName, evt.Msg)
		case err := <-watcher.Error():
			fmt.Printf("\nError: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}
```

## WMI-based Approach (New, More Stable Approach)

```Go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	winlog "github.com/werbes/gowinlog"
)

func main() {
	// Create a new WMI-based event log watcher
	watcher, err := winlog.NewWMIEventLogWatcher()
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

	fmt.Println("Watching for events. Press Ctrl+C to exit.")

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
			}
		}
	}()

	// Wait for signal
	<-sigChan
	fmt.Println("Shutting down...")
	watcher.Shutdown()
}
```

## Choosing Between Approaches

- Use the **WMI-based approach** if you're experiencing stability issues with the original approach.
- Use the **Windows Event Log API approach** if you need features not available in the WMI-based approach, such as bookmark support.

## WMI-based Approach Limitations

- Does not support bookmarks
- Polling-based rather than event-driven
- May have higher resource usage due to polling
- Limited filtering capabilities compared to XPath queries

Low-level API
------

`winevt.go` provides wrappers around the relevant functions in `wevtapi.dll`.
`wmi_eventlog.go` provides a WMI-based implementation for accessing Windows event logs.

Thread Safety
------

Both approaches are designed to be thread-safe, allowing multiple goroutines to interact with them concurrently:

- The `WinLogWatcher` and `WMIEventLogWatcher` structs use internal synchronization to protect their state.
- Bookmark operations (UpdateBookmark, RenderBookmark) are synchronized to prevent race conditions when multiple goroutines operate on the same bookmark handle.
- Event channels are safe for concurrent consumption.

This means you can safely use the library in multi-threaded applications without worrying about race conditions.

Dependencies
------

- github.com/go-ole/go-ole: Required for the WMI-based approach
- golang.org/x/sys/windows: Required for both approaches
