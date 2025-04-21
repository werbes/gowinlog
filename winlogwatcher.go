//go:build windows
// +build windows

// Winlog hooks into the Windows Event Log and streams events through channels
package winlog

import (
	"fmt"
	"time"
)

// Winlog hooks into the Windows Event Log and streams events through channels

/* WinLogWatcher encompasses the overall functionality, eventlog subscriptions etc. */

// Event Channel for receiving events
func (wlw *WinLogWatcher) Event() <-chan *WinLogEvent {
	return wlw.eventChan
}

// Channel for receiving errors (not "error" events)
func (wlw *WinLogWatcher) Error() <-chan error {
	return wlw.errChan
}

// NewWinLogWatcher creates a new watcher
func NewWinLogWatcher() (*WinLogWatcher, error) {
	// Create a WMI-based watcher that will be used for all operations
	wmiWatcher, err := NewWMIEventLogWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create WMI watcher: %v", err)
	}

	// Still create a render context for backward compatibility
	cHandle, err := GetSystemRenderContext()
	if err != nil {
		return nil, err
	}

	// Create channels for events and errors
	eventChan := make(chan *WinLogEvent)
	errChan := make(chan error)
	shutdown := make(chan interface{})

	// Create the watcher
	watcher := &WinLogWatcher{
		shutdown:       shutdown,
		errChan:        errChan,
		eventChan:      eventChan,
		renderContext:  cHandle,
		watches:        make(map[string]*channelWatcher),
		wmiWatcher:     wmiWatcher,
		RenderKeywords: true,
		RenderMessage:  true,
		RenderLevel:    true,
		RenderTask:     true,
		RenderProvider: true,
		RenderOpcode:   true,
		RenderChannel:  true,
		RenderId:       true,
	}

	// Forward events from the WMI watcher to the original event channel
	go func() {
		for {
			select {
			case event := <-wmiWatcher.Event():
				select {
				case eventChan <- event:
				case <-shutdown:
					return
				}
			case <-shutdown:
				return
			}
		}
	}()

	// Forward errors from the WMI watcher to the original error channel
	go func() {
		for {
			select {
			case err := <-wmiWatcher.Error():
				select {
				case errChan <- err:
				case <-shutdown:
					return
				}
			case <-shutdown:
				return
			}
		}
	}()

	return watcher, nil
}

// Subscribe to a Windows Event Log channel, starting with the first event
// in the log. `query` is an XPath expression for filtering events: to recieve
// all events on the channel, use "*" as the query.
func (self *WinLogWatcher) SubscribeFromBeginning(channel, query string) error {
	// Delegate to the WMI-based implementation
	return self.wmiWatcher.SubscribeFromBeginning(channel, query)
}

// Subscribe to a Windows Event Log channel, starting with the next event
// that arrives. `query` is an XPath expression for filtering events: to recieve
// all events on the channel, use "*" as the query.
func (self *WinLogWatcher) SubscribeFromNow(channel, query string) error {
	// Delegate to the WMI-based implementation
	return self.wmiWatcher.SubscribeFromNow(channel, query)
}

func (self *WinLogWatcher) subscribeWithoutBookmark(channel, query string, flags EVT_SUBSCRIBE_FLAGS) error {
	self.watchMutex.Lock()
	defer self.watchMutex.Unlock()
	if _, ok := self.watches[channel]; ok {
		return fmt.Errorf("A watcher for channel %q already exists", channel)
	}
	newBookmark, err := CreateBookmark()
	if err != nil {
		return fmt.Errorf("Failed to create new bookmark handle: %v", err)
	}
	callback := &LogEventCallbackWrapper{callback: self, subscribedChannel: channel}
	subscription, err := CreateListener(channel, query, flags, callback)
	if err != nil {
		CloseEventHandle(uint64(newBookmark))
		return err
	}
	self.watches[channel] = &channelWatcher{
		bookmark:     newBookmark,
		subscription: subscription,
		callback:     callback,
	}
	return nil
}

// Subscribe to a Windows Event Log channel, starting with the first event in the log
// after the bookmarked event. There may be a gap if events have been purged. `query`
// is an XPath expression for filtering events: to recieve all events on the channel,
// use "*" as the query
//
// Note: When using the WMI-based implementation, bookmarks are not supported.
// This method will fall back to SubscribeFromBeginning and log a warning.
func (self *WinLogWatcher) SubscribeFromBookmark(channel, query string, xmlString string) error {
	// The WMI implementation doesn't support bookmarks, so we'll fall back to SubscribeFromBeginning
	self.PublishError(fmt.Errorf("WMI implementation does not support bookmarks, falling back to SubscribeFromBeginning for channel %q", channel))

	// Delegate to SubscribeFromBeginning
	return self.SubscribeFromBeginning(channel, query)
}

/* Remove subscription from channel */
func (self *WinLogWatcher) RemoveSubscription(channel string) error {
	// Delegate to the WMI-based implementation
	return self.wmiWatcher.RemoveSubscription(channel)
}

// Remove all subscriptions from this watcher and shut down.
func (self *WinLogWatcher) Shutdown() {
	// Shutdown the WMI-based watcher
	self.wmiWatcher.Shutdown()

	// Also perform the original shutdown for backward compatibility
	close(self.shutdown)
	for channel := range self.watches {
		// Don't call RemoveSubscription here as it now delegates to WMI
		self.watchMutex.Lock()
		if watch, ok := self.watches[channel]; ok {
			CancelEventHandle(uint64(watch.subscription))
			CloseEventHandle(uint64(watch.subscription))
			CloseEventHandle(uint64(watch.bookmark))
		}
		delete(self.watches, channel)
		self.watchMutex.Unlock()
	}
	CloseEventHandle(uint64(self.renderContext))
	close(self.errChan)
	close(self.eventChan)
}

/* Publish the received error to the errChan, but discard if shutdown is in progress */
func (self *WinLogWatcher) PublishError(err error) {
	// Publish to both the WMI watcher and the original error channel
	self.wmiWatcher.PublishError(err)

	// Also publish to the original error channel for backward compatibility
	select {
	case self.errChan <- err:
	case <-self.shutdown:
	}
}

// This function is no longer used. All event conversion is now done directly in PublishEvent
// to avoid potential shared violations with event handles.
func (self *WinLogWatcher) convertEvent(handle EventHandle, subscribedChannel string) (*WinLogEvent, error) {
	// Add a defer/recover block to catch any panics during event conversion
	defer func() {
		if r := recover(); r != nil {
			self.PublishError(fmt.Errorf("Recovered from panic in convertEvent: %v", r))
		}
	}()

	// Check if handle is valid
	if handle == 0 {
		return nil, fmt.Errorf("convertEvent received invalid handle: 0")
	}

	// This function is deprecated and should not be used.
	// All event conversion is now done directly in PublishEvent to avoid potential shared violations.
	self.PublishError(fmt.Errorf("convertEvent is deprecated, use PublishEvent directly"))
	return nil, fmt.Errorf("convertEvent is deprecated, use PublishEvent directly")
}

/* Publish a new event */
func (self *WinLogWatcher) PublishEvent(handle EventHandle, subscribedChannel string) {
	// This method is no longer used directly since we're using the WMI-based implementation
	// However, we'll keep it for backward compatibility

	// Add a defer/recover block to catch any panics during event publishing
	defer func() {
		if r := recover(); r != nil {
			self.PublishError(fmt.Errorf("Recovered from panic in PublishEvent: %v", r))
		}
	}()

	// Log a warning that this method is deprecated
	self.PublishError(fmt.Errorf("PublishEvent is deprecated when using WMI-based implementation"))

	// Check if handle is valid
	if handle == 0 {
		self.PublishError(fmt.Errorf("PublishEvent received invalid handle: 0"))
		return
	}

	// Check if subscribedChannel is valid
	if subscribedChannel == "" {
		self.PublishError(fmt.Errorf("PublishEvent received empty subscribedChannel"))
		return
	}

	// First, render the event as XML to make a copy of its data
	// This ensures we don't keep the original handle open for too long
	var eventXML string
	var xmlErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				xmlErr = fmt.Errorf("panic in RenderEventXML: %v", r)
			}
		}()
		eventXML, xmlErr = RenderEventXML(handle)
	}()

	// Get the bookmark for the channel - wrapped in a try/catch block
	var watch *channelWatcher
	var ok bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				self.PublishError(fmt.Errorf("Recovered from panic in bookmark lookup: %v", r))
				ok = false
			}
		}()
		self.watchMutex.Lock()
		defer self.watchMutex.Unlock()
		watch, ok = self.watches[subscribedChannel]
	}()

	if !ok || watch == nil {
		self.PublishError(fmt.Errorf("No handle for channel bookmark %q", subscribedChannel))
		// Close the handle before returning
		CloseEventHandle(uint64(handle))
		return
	}

	// Check if bookmark is valid
	if watch.bookmark == 0 {
		self.PublishError(fmt.Errorf("Invalid bookmark handle for channel %q", subscribedChannel))
		// Close the handle before returning
		CloseEventHandle(uint64(handle))
		return
	}

	// Update the bookmark with the current event - wrapped in a try/catch block
	var bookmarkErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				bookmarkErr = fmt.Errorf("panic in UpdateBookmark: %v", r)
			}
		}()
		bookmarkErr = UpdateBookmark(watch.bookmark, handle)
	}()

	if bookmarkErr != nil {
		self.PublishError(fmt.Errorf("Error updating bookmark: %v", bookmarkErr))
		// Continue processing the event even if bookmark update fails
	}

	// Serialize the bookmark as XML and include it in the event - wrapped in a try/catch block
	var bookmarkXml string
	if watch.bookmark != 0 {
		var bookmarkErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					bookmarkErr = fmt.Errorf("panic in RenderBookmark: %v", r)
				}
			}()
			bookmarkXml, bookmarkErr = RenderBookmark(watch.bookmark)
		}()

		if bookmarkErr != nil {
			self.PublishError(fmt.Errorf("Error rendering bookmark: %v", bookmarkErr))
			// Continue processing the event even if bookmark rendering fails
		}
	}

	// Render all the event values we need before closing the handle
	// This ensures we don't access the handle after it's closed
	var renderedFields EvtVariant
	var renderedFieldsErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				renderedFieldsErr = fmt.Errorf("panic in RenderEventValues: %v", r)
			}
		}()
		renderedFields, renderedFieldsErr = RenderEventValues(self.renderContext, handle)
	}()

	// Get the publisher handle for message formatting
	var publisherHandle PublisherHandle
	var publisherHandleErr error
	if renderedFieldsErr == nil && renderedFields != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					publisherHandleErr = fmt.Errorf("panic in GetEventPublisherHandle: %v", r)
				}
			}()
			publisherHandle, publisherHandleErr = GetEventPublisherHandle(renderedFields)
		}()
	}

	// Now create the event from the rendered data
	var event *WinLogEvent
	if renderedFieldsErr == nil && renderedFields != nil {
		// Extract fields from the rendered values
		var computerName, providerName, channel string
		var level, task, opcode, recordId, qualifiers, eventId, processId, threadId, version uint64
		var created time.Time
		var keywordsText, msgText, lvlText, taskText, providerText, opcodeText, channelText, idText string

		// Extract system fields
		func() {
			defer func() {
				if r := recover(); r != nil {
					self.PublishError(fmt.Errorf("Recovered from panic in field extraction: %v", r))
				}
			}()

			computerName, _ = renderedFields.String(EvtSystemComputer)
			providerName, _ = renderedFields.String(EvtSystemProviderName)
			channel, _ = renderedFields.String(EvtSystemChannel)
			level, _ = renderedFields.Uint(EvtSystemLevel)
			task, _ = renderedFields.Uint(EvtSystemTask)
			opcode, _ = renderedFields.Uint(EvtSystemOpcode)
			recordId, _ = renderedFields.Uint(EvtSystemEventRecordId)
			qualifiers, _ = renderedFields.Uint(EvtSystemQualifiers)
			eventId, _ = renderedFields.Uint(EvtSystemEventID)
			processId, _ = renderedFields.Uint(EvtSystemProcessID)
			threadId, _ = renderedFields.Uint(EvtSystemThreadID)
			version, _ = renderedFields.Uint(EvtSystemVersion)
			created, _ = renderedFields.FileTime(EvtSystemTimeCreated)
		}()

		// Format localized messages if we have a valid publisher handle and event handle
		if publisherHandleErr == nil && publisherHandle != 0 && handle != 0 {
			// Format messages using the publisher handle and event handle
			func() {
				defer func() {
					if r := recover(); r != nil {
						self.PublishError(fmt.Errorf("Recovered from panic in message formatting: %v", r))
					}
				}()

				// Format messages using the Windows API
				if self.RenderKeywords && publisherHandle != 0 {
					var err error
					keywordsText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageKeyword)
					if err != nil {
						// Provide a fallback message with the event ID
						keywordsText = fmt.Sprintf("[Keywords for Event ID: %d]", eventId)
					}
				}

				if self.RenderMessage && publisherHandle != 0 {
					var err error
					msgText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageEvent)
					if err != nil {
						// Provide a fallback message with the event ID
						msgText = fmt.Sprintf("[Event ID: %d]", eventId)
					}
				}

				if self.RenderLevel && publisherHandle != 0 {
					var err error
					lvlText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageLevel)
					if err != nil {
						// Provide a fallback message with the level
						lvlText = fmt.Sprintf("[Level: %d]", level)
					}
				}

				if self.RenderTask && publisherHandle != 0 {
					var err error
					taskText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageTask)
					if err != nil {
						// Provide a fallback message with the task
						taskText = fmt.Sprintf("[Task: %d]", task)
					}
				}

				if self.RenderProvider && publisherHandle != 0 {
					var err error
					providerText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageProvider)
					if err != nil {
						// Provide a fallback with the provider name
						providerText = providerName
					}
				}

				if self.RenderOpcode && publisherHandle != 0 {
					var err error
					opcodeText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageOpcode)
					if err != nil {
						// Provide a fallback message with the opcode
						opcodeText = fmt.Sprintf("[Opcode: %d]", opcode)
					}
				}

				if self.RenderChannel && publisherHandle != 0 {
					var err error
					channelText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageChannel)
					if err != nil {
						// Provide a fallback with the channel
						channelText = channel
					}
				}

				if self.RenderId && publisherHandle != 0 {
					var err error
					idText, err = FormatMessage(publisherHandle, handle, EvtFormatMessageId)
					if err != nil {
						// Provide a fallback with the ID
						idText = fmt.Sprintf("%d", eventId)
					}
				}
			}()

			// Close the publisher handle when we're done with it
			func() {
				defer func() {
					if r := recover(); r != nil {
						self.PublishError(fmt.Errorf("Recovered from panic in CloseEventHandle (publisher): %v", r))
					}
				}()
				CloseEventHandle(uint64(publisherHandle))
			}()
		}

		// Now that we've formatted all the messages, we can close the event handle
		// This ensures we don't keep the handle open while processing the event
		func() {
			defer func() {
				if r := recover(); r != nil {
					self.PublishError(fmt.Errorf("Recovered from panic in CloseEventHandle: %v", r))
				}
			}()
			CloseEventHandle(uint64(handle))
		}()

		// Create the event object
		func() {
			defer func() {
				if r := recover(); r != nil {
					self.PublishError(fmt.Errorf("Recovered from panic in event creation: %v", r))
				}
			}()

			event = &WinLogEvent{
				Xml:               eventXML,
				XmlErr:            xmlErr,
				ProviderName:      providerName,
				EventId:           eventId,
				Qualifiers:        qualifiers,
				Level:             level,
				Task:              task,
				Opcode:            opcode,
				Created:           created,
				RecordId:          recordId,
				ProcessId:         processId,
				ThreadId:          threadId,
				Channel:           channel,
				ComputerName:      computerName,
				Version:           version,
				RenderedFieldsErr: renderedFieldsErr,

				Keywords:           keywordsText,
				Msg:                msgText,
				LevelText:          lvlText,
				TaskText:           taskText,
				OpcodeText:         opcodeText,
				ChannelText:        channelText,
				ProviderText:       providerText,
				IdText:             idText,
				PublisherHandleErr: publisherHandleErr,

				SubscribedChannel: subscribedChannel,
				Bookmark:          bookmarkXml,
			}
		}()
	}

	// If we couldn't create the event but we have XML, create a basic event with the XML data
	if (event == nil) && eventXML != "" {
		event = &WinLogEvent{
			Xml:               eventXML,
			XmlErr:            xmlErr,
			SubscribedChannel: subscribedChannel,
			Bookmark:          bookmarkXml,
		}
	}

	// Check if event is valid
	if event == nil {
		self.PublishError(fmt.Errorf("Failed to create event from XML"))
		return
	}

	// Don't block when shutting down if the consumer has gone away - wrapped in a try/catch block
	func() {
		defer func() {
			if r := recover(); r != nil {
				self.PublishError(fmt.Errorf("Recovered from panic in event channel send: %v", r))
			}
		}()
		select {
		case self.eventChan <- event:
		case <-self.shutdown:
			return
		}
	}()
}
