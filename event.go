//go:build windows
// +build windows

package winlog

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

/*Functionality related to events and listening to the event log*/

// Get a handle to a render context which will render properties from the System element.
//
//	Wraps EvtCreateRenderContext() with Flags = EvtRenderContextSystem. The resulting
//	handle must be closed with CloseEventHandle.
func GetSystemRenderContext() (SysRenderContext, error) {
	context, err := EvtCreateRenderContext(0, 0, EvtRenderContextSystem)
	if err != nil {
		return 0, err
	}
	return SysRenderContext(context), nil
}

/*
Get a handle for a event log subscription on the given channel.

	`query` is an XPath expression to filter the events on the channel - "*" allows all events.
	The resulting handle must be closed with CloseEventHandle.
*/
func CreateListener(channel, query string, startpos EVT_SUBSCRIBE_FLAGS, watcher *LogEventCallbackWrapper) (ListenerHandle, error) {
	wideChan, err := syscall.UTF16PtrFromString(channel)
	if err != nil {
		return 0, err
	}
	wideQuery, err := syscall.UTF16PtrFromString(query)
	if err != nil {
		return 0, err
	}
	listenerHandle, err := EvtSubscribe(0, 0, wideChan, wideQuery, 0, uintptr(unsafe.Pointer(watcher)), uintptr(syscall.NewCallback(eventCallback)), uint32(startpos))
	if err != nil {
		return 0, err
	}
	return ListenerHandle(listenerHandle), nil
}

/*
Get a handle for an event log subscription on the given channel. Will begin at the

	bookmarked event, or the closest possible event if the log has been truncated.
	`query` is an XPath expression to filter the events on the channel - "*" allows all events.
	The resulting handle must be closed with CloseEventHandle.
*/
func CreateListenerFromBookmark(channel, query string, watcher *LogEventCallbackWrapper, bookmarkHandle BookmarkHandle) (ListenerHandle, error) {
	wideChan, err := syscall.UTF16PtrFromString(channel)
	if err != nil {
		return 0, err
	}
	wideQuery, err := syscall.UTF16PtrFromString(query)
	if err != nil {
		return 0, err
	}
	listenerHandle, err := EvtSubscribe(0, 0, wideChan, wideQuery, syscall.Handle(bookmarkHandle), uintptr(unsafe.Pointer(watcher)), syscall.NewCallback(eventCallback), uint32(EvtSubscribeStartAfterBookmark))
	if err != nil {
		return 0, err
	}
	return ListenerHandle(listenerHandle), nil
}

/*
Get the formatted string that represents this message. This method wraps EvtFormatMessage.

	For specific error cases like missing locale resources (ERROR_MR_MID_NOT_FOUND) or
	incorrect parameters (ERROR_INVALID_PARAMETER), it returns a descriptive placeholder
	message instead of an error to ensure events are still processed.
*/
/* Get the formatted string that represents this message. This method wraps EvtFormatMessage. */
func FormatMessage(eventPublisherHandle PublisherHandle, eventHandle EventHandle, format EVT_FORMAT_MESSAGE_FLAGS) (string, error) {
	// Add defensive checks for invalid handles
	if eventPublisherHandle == 0 {
		return "[Invalid publisher handle]", fmt.Errorf("invalid publisher handle: 0")
	}
	if eventHandle == 0 {
		return "[Invalid event handle]", fmt.Errorf("invalid event handle: 0")
	}

	// Use defer/recover to catch any panics during the call
	var result string
	var resultErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				result = "[Recovered from panic in FormatMessage]"
				resultErr = fmt.Errorf("panic in FormatMessage: %v", r)
			}
		}()

		// First call to get the required buffer size - wrapped in its own try/catch block
		var size uint32 = 0
		func() {
			defer func() {
				if r := recover(); r != nil {
					result = "[Recovered from panic in buffer size calculation]"
					resultErr = fmt.Errorf("panic in buffer size calculation: %v", r)
				}
			}()

			// Explicitly set ValueCount to 0 when Values is nil to prevent crashes
			err := EvtFormatMessage(syscall.Handle(eventPublisherHandle), syscall.Handle(eventHandle), 0, 0, nil, uint32(format), 0, nil, &size)
			if err != nil {
				if errno, ok := err.(syscall.Errno); !ok || errno != 122 { // ERROR_INSUFFICIENT_BUFFER
					resultErr = err
					return
				}
			}
		}()

		// If we already have an error or result from the first call, return early
		if resultErr != nil || result != "" {
			return
		}

		// Ensure we don't allocate an excessively large buffer
		if size == 0 {
			result = "[Failed to get buffer size]"
			return
		}

		if size > 1024*1024 { // 1MB limit
			result = "[Message too large to render]"
			return
		}

		// Allocate buffer of the required size
		buf := make([]uint16, size)
		if len(buf) == 0 || size == 0 {
			result = "[Failed to allocate buffer]"
			resultErr = fmt.Errorf("failed to allocate buffer of size %d", size)
			return
		}

		// Second call to get the actual message - wrapped in its own try/catch block
		func() {
			defer func() {
				if r := recover(); r != nil {
					result = "[Recovered from panic in message rendering]"
					resultErr = fmt.Errorf("panic in message rendering: %v", r)
				}
			}()

			// Explicitly set ValueCount to 0 when Values is nil to prevent crashes
			err := EvtFormatMessage(syscall.Handle(eventPublisherHandle), syscall.Handle(eventHandle), 0, 0, nil, uint32(format), uint32(size), &buf[0], &size)
			if err != nil {
				// Handle specific error codes that might occur but shouldn't prevent processing
				if errno, ok := err.(syscall.Errno); ok {
					// ERROR_EVT_MESSAGE_NOT_FOUND or ERROR_MR_MID_NOT_FOUND - missing message resource
					if errno == 15027 || errno == 317 {
						result = "[Message resource not found for event ID]"
						return
					}
					// ERROR_EVT_MESSAGE_ID_NOT_FOUND - message ID not found
					if errno == 15028 {
						result = "[Message ID not found]"
						return
					}
					// ERROR_EVT_UNRESOLVED_VALUE_INSERT - unresolved insert
					if errno == 15029 {
						result = "[Message contains unresolved parameter]"
						return
					}
				}
				resultErr = err
				return
			}
		}()

		// If we already have an error or result from the second call, return early
		if resultErr != nil || result != "" {
			return
		}

		// Convert the buffer to a string
		if len(buf) > 0 {
			result = syscall.UTF16ToString(buf)
		} else {
			result = "[Empty message buffer]"
		}
	}()

	if resultErr != nil {
		return result, resultErr
	}
	return result, nil
}

/* Get the formatted string for the last error which occurred. Wraps GetLastError and FormatMessage. */
func GetLastError() error {
	return syscall.GetLastError()
}

/*
Render the system properties from the event and returns an array of properties.

	Properties can be accessed using RenderStringField, RenderIntField, RenderFileTimeField,
	or RenderUIntField depending on type. This buffer must be freed after use.
*/
func RenderEventValues(renderContext SysRenderContext, eventHandle EventHandle) (EvtVariant, error) {
	// Check if handles are valid
	if renderContext == 0 {
		return nil, fmt.Errorf("invalid render context")
	}
	if eventHandle == 0 {
		return nil, fmt.Errorf("invalid event handle")
	}

	// Use defer/recover to catch any panics that might occur
	var result EvtVariant
	var resultErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				resultErr = fmt.Errorf("panic in RenderEventValues: %v", r)
			}
		}()

		var bufferUsed uint32 = 0
		var propertyCount uint32 = 0
		err := EvtRender(syscall.Handle(renderContext), syscall.Handle(eventHandle), EvtRenderEventValues, 0, nil, &bufferUsed, &propertyCount)
		if bufferUsed == 0 {
			resultErr = err
			return
		}

		// Ensure we don't allocate an excessively large buffer
		if bufferUsed > 5*1024*1024 { // 5MB limit
			resultErr = fmt.Errorf("event values too large to render: %d bytes", bufferUsed)
			return
		}

		buffer := make([]byte, bufferUsed)
		bufSize := bufferUsed
		err = EvtRender(syscall.Handle(renderContext), syscall.Handle(eventHandle), EvtRenderEventValues, bufSize, (*uint16)(unsafe.Pointer(&buffer[0])), &bufferUsed, &propertyCount)
		if err != nil {
			resultErr = err
			return
		}
		result = NewEvtVariant(buffer)
	}()

	return result, resultErr
}

// Render the event as XML.
func RenderEventXML(eventHandle EventHandle) (string, error) {
	// Check if handle is valid
	if eventHandle == 0 {
		return "", fmt.Errorf("invalid event handle")
	}

	// Use defer/recover to catch any panics that might occur
	var result string
	var resultErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				result = "[Recovered from panic in RenderEventXML]"
				resultErr = fmt.Errorf("panic in RenderEventXML: %v", r)
			}
		}()

		var bufferUsed, propertyCount uint32

		err := EvtRender(0, syscall.Handle(eventHandle), EvtRenderEventXml, 0, nil, &bufferUsed, &propertyCount)

		if bufferUsed == 0 {
			resultErr = err
			return
		}

		// Ensure we don't allocate an excessively large buffer
		if bufferUsed > 10*1024*1024 { // 10MB limit
			result = "[XML too large to render]"
			return
		}

		buffer := make([]byte, bufferUsed)
		bufSize := bufferUsed

		err = EvtRender(0, syscall.Handle(eventHandle), EvtRenderEventXml, bufSize, (*uint16)(unsafe.Pointer(&buffer[0])), &bufferUsed, &propertyCount)
		if err != nil {
			result = err.Error()
			resultErr = err
			return
		}

		// Remove null bytes
		xml := bytes.Replace(buffer, []byte("\x00"), []byte{}, -1)

		result = string(xml)
	}()

	return result, resultErr
}

/* Get a handle that represents the publisher of the event, given the rendered event values. */
func GetEventPublisherHandle(renderedFields EvtVariant) (PublisherHandle, error) {
	// Add defensive checks to prevent crashes
	if renderedFields == nil || len(renderedFields) == 0 {
		return 0, fmt.Errorf("rendered fields is nil or empty")
	}

	// Use defer/recover to catch any panics that might occur
	var publisherHandle PublisherHandle
	var resultErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				resultErr = fmt.Errorf("panic in GetEventPublisherHandle: %v", r)
			}
		}()

		publisher, err := renderedFields.String(EvtSystemProviderName)
		if err != nil {
			resultErr = fmt.Errorf("error getting provider name: %v", err)
			return
		}

		// Check if publisher name is empty
		if publisher == "" {
			resultErr = fmt.Errorf("provider name is empty")
			return
		}

		widePublisher, err := syscall.UTF16PtrFromString(publisher)
		if err != nil {
			resultErr = fmt.Errorf("error converting publisher name to UTF16: %v", err)
			return
		}

		handle, err := EvtOpenPublisherMetadata(0, widePublisher, nil, 0, 0)
		if err != nil {
			resultErr = fmt.Errorf("error opening publisher metadata: %v", err)
			return
		}

		// Check if handle is valid
		if handle == 0 {
			resultErr = fmt.Errorf("publisher handle is 0")
			return
		}

		publisherHandle = PublisherHandle(handle)
	}()

	if resultErr != nil {
		return 0, resultErr
	}

	return publisherHandle, nil
}

/* Close an event handle. */
func CloseEventHandle(handle uint64) error {
	// Check if handle is valid
	if handle == 0 {
		return fmt.Errorf("invalid handle")
	}
	return EvtClose(syscall.Handle(handle))
}

/* Cancel pending actions on the event handle. */
func CancelEventHandle(handle uint64) error {
	// Check if handle is valid
	if handle == 0 {
		return fmt.Errorf("invalid handle")
	}
	err := EvtCancel(syscall.Handle(handle))
	if err != nil {
		return err
	}
	return nil
}

/* Get the first event in the log, for testing */
func getTestEventHandle() (EventHandle, error) {
	wideQuery, _ := syscall.UTF16PtrFromString("*")
	wideChannel, _ := syscall.UTF16PtrFromString("Application")
	handle, err := EvtQuery(0, wideChannel, wideQuery, EvtQueryChannelPath)
	if err != nil {
		return 0, err
	}

	var record syscall.Handle
	var recordsReturned uint32
	err = EvtNext(handle, 1, &record, 500, 0, &recordsReturned)
	if err != nil {
		EvtClose(handle)
		return 0, nil
	}
	EvtClose(handle)
	return EventHandle(record), nil
}

func eventCallback(Action uint32, Context unsafe.Pointer, handle syscall.Handle) uintptr {
	// Add defensive checks to prevent crashes
	if Context == nil {
		// Can't call PublishError because we don't have a callback
		// Just log to stderr and return
		fmt.Fprintf(os.Stderr, "Event log callback got nil Context\n")
		return 0
	}

	// Recover from any panics that might occur during callback processing
	defer func() {
		if r := recover(); r != nil {
			// Try to log the error, but if that fails, just log to stderr
			cbWrap := (*LogEventCallbackWrapper)(Context)
			if cbWrap != nil && cbWrap.callback != nil {
				cbWrap.callback.PublishError(fmt.Errorf("Recovered from panic in eventCallback: %v", r))
			} else {
				fmt.Fprintf(os.Stderr, "Recovered from panic in eventCallback: %v\n", r)
			}
		}
	}()

	cbWrap := (*LogEventCallbackWrapper)(Context)
	if cbWrap.callback == nil {
		fmt.Fprintf(os.Stderr, "Event log callback got nil callback in wrapper\n")
		return 0
	}

	if Action == 0 {
		cbWrap.callback.PublishError(fmt.Errorf("Event log callback got error: %v", GetLastError()))
	} else {
		// Check if handle is valid
		if handle == 0 {
			cbWrap.callback.PublishError(fmt.Errorf("Event log callback got invalid handle"))
			return 0
		}

		// Pass the event handle to PublishEvent
		// The handle will be closed by PublishEvent after it's done with it
		cbWrap.callback.PublishEvent(EventHandle(handle), cbWrap.subscribedChannel)
	}
	return 0
}

// CreateMap converts the WinLogEvent to a map[string]interface{}
func (ev *WinLogEvent) CreateMap() map[string]interface{} {
	toReturn := make(map[string]interface{})
	toReturn["ProviderName"] = ev.ProviderName
	toReturn["EventId"] = ev.EventId
	toReturn["Qualifiers"] = ev.Qualifiers
	toReturn["Level"] = ev.Level
	toReturn["Task"] = ev.Task
	toReturn["Opcode"] = ev.Opcode
	toReturn["Created"] = ev.Created
	toReturn["RecordId"] = ev.RecordId
	toReturn["ProcessId"] = ev.ProcessId
	toReturn["ThreadId"] = ev.ThreadId
	toReturn["Channel"] = ev.Channel
	toReturn["ComputerName"] = ev.ComputerName
	toReturn["Version"] = ev.Version
	toReturn["Msg"] = ev.Msg
	toReturn["LevelText"] = ev.LevelText
	toReturn["TaskText"] = ev.TaskText
	toReturn["OpcodeText"] = ev.OpcodeText
	toReturn["Keywords"] = ev.Keywords
	toReturn["ChannelText"] = ev.ChannelText
	toReturn["ProviderText"] = ev.ProviderText
	toReturn["IdText"] = ev.IdText
	toReturn["Bookmark"] = ev.Bookmark
	toReturn["SubscribedChannel"] = ev.SubscribedChannel
	toReturn["Bookmark"] = ev.Bookmark
	return toReturn
}
