// +build windows

package winlog

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

/*Functionality related to events and listening to the event log*/

// Get a handle to a render context which will render properties from the System element.
//    Wraps EvtCreateRenderContext() with Flags = EvtRenderContextSystem. The resulting
//    handle must be closed with CloseEventHandle.
func GetSystemRenderContext() (SysRenderContext, error) {
	context, err := EvtCreateRenderContext(0, 0, EvtRenderContextSystem)
	if err != nil {
		return 0, err
	}
	return SysRenderContext(context), nil
}

/* Get a handle for a event log subscription on the given channel.
   `query` is an XPath expression to filter the events on the channel - "*" allows all events.
   The resulting handle must be closed with CloseEventHandle. */
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

/* Get a handle for an event log subscription on the given channel. Will begin at the
   bookmarked event, or the closest possible event if the log has been truncated.
   `query` is an XPath expression to filter the events on the channel - "*" allows all events.
   The resulting handle must be closed with CloseEventHandle. */
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

// EventXML represents the structure of a Windows Event Log XML
type EventXML struct {
	System struct {
		Provider struct {
			Name        string `xml:"Name,attr"`
			Guid        string `xml:"Guid,attr"`
			EventSourceName string `xml:"EventSourceName,attr"`
		} `xml:"Provider"`
		EventID     struct {
			Value       int    `xml:",chardata"`
			Qualifiers  string `xml:"Qualifiers,attr"`
		} `xml:"EventID"`
		Version     int    `xml:"Version"`
		Level       int    `xml:"Level"`
		Task        int    `xml:"Task"`
		Opcode      int    `xml:"Opcode"`
		Keywords    string `xml:"Keywords"`
		TimeCreated struct {
			SystemTime string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
		EventRecordID int    `xml:"EventRecordID"`
		Correlation   struct {
			ActivityID    string `xml:"ActivityID,attr"`
			RelatedActivityID string `xml:"RelatedActivityID,attr"`
		} `xml:"Correlation"`
		Execution    struct {
			ProcessID   int    `xml:"ProcessID,attr"`
			ThreadID    int    `xml:"ThreadID,attr"`
		} `xml:"Execution"`
		Channel     string `xml:"Channel"`
		Computer    string `xml:"Computer"`
		Security    struct {
			UserID      string `xml:"UserID,attr"`
		} `xml:"Security"`
	} `xml:"System"`
	EventData struct {
		Data []string `xml:"Data"`
		// Some events use binary data
		Binary []struct {
			Value string `xml:",chardata"`
		} `xml:"Binary"`
	} `xml:"EventData"`
	UserData struct {
		// UserData can contain any XML structure, so we'll use a map to store it
		// This is important for handling various event types
		XMLName xml.Name
		Attrs   []xml.Attr `xml:",any,attr"`
		Content []byte     `xml:",innerxml"`
	} `xml:"UserData"`
	// EventRecordData is used by some event types
	EventRecordData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventRecordData"`
	RenderingInfo struct {
		Message     string `xml:"Message"`
		Level       string `xml:"Level"`
		Task        string `xml:"Task"`
		Opcode      string `xml:"Opcode"`
		Channel     string `xml:"Channel"`
		Provider    string `xml:"Provider"`
		Keywords    struct {
			Keyword []string `xml:"Keyword"`
		} `xml:"Keywords"`
	} `xml:"RenderingInfo"`
}

// ExtractFromXML parses the XML representation of an event and extracts the specified information
func ExtractFromXML(xmlData string, format EVT_FORMAT_MESSAGE_FLAGS) (string, error) {
	// If XML is empty, return empty string
	if xmlData == "" {
		return "", nil
	}

	var event EventXML
	err := xml.Unmarshal([]byte(xmlData), &event)
	if err != nil {
		return "", fmt.Errorf("Failed to parse event XML: %v", err)
	}

	switch format {
	case EvtFormatMessageEvent:
		// Try to get message from RenderingInfo first
		if event.RenderingInfo.Message != "" {
			return event.RenderingInfo.Message, nil
		}

		// Check for EventRecordData
		if len(event.EventRecordData.Data) > 0 {
			var msg strings.Builder
			msg.WriteString(fmt.Sprintf("Event ID: %d", event.System.EventID.Value))
			msg.WriteString(" - EventRecordData: ")

			// Include EventRecordData elements
			for i, data := range event.EventRecordData.Data {
				if i > 0 {
					msg.WriteString(", ")
				}
				msg.WriteString(fmt.Sprintf("%s: %s", data.Name, data.Value))
			}

			return msg.String(), nil
		}

		// If no EventRecordData, construct a basic message from event data
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("Event ID: %d", event.System.EventID.Value))

		// Include regular data elements if available
		if len(event.EventData.Data) > 0 {
			msg.WriteString(" - Data: ")
			for i, data := range event.EventData.Data {
				if i > 0 {
					msg.WriteString(", ")
				}
				msg.WriteString(data)
			}
		}

		// Include binary data if available
		if len(event.EventData.Binary) > 0 {
			if len(event.EventData.Data) > 0 {
				msg.WriteString("; ")
			} else {
				msg.WriteString(" - ")
			}
			msg.WriteString("Binary: ")
			for i, binary := range event.EventData.Binary {
				if i > 0 {
					msg.WriteString(", ")
				}
				// Limit the binary data length to avoid very long messages
				value := binary.Value
				if len(value) > 50 {
					value = value[:50] + "..."
				}
				msg.WriteString(value)
			}
		}

		// If we still don't have any data, check UserData as a last resort
		if len(event.EventData.Data) == 0 && len(event.EventData.Binary) == 0 && len(event.UserData.Content) > 0 {
			msg.WriteString(" - UserData: ")
			// Limit the content length to avoid very long messages
			content := string(event.UserData.Content)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			msg.WriteString(content)
		}

		return msg.String(), nil
	case EvtFormatMessageLevel:
		if event.RenderingInfo.Level != "" {
			return event.RenderingInfo.Level, nil
		}
		// Map level numbers to common names
		switch event.System.Level {
		case 0:
			return "LogAlways", nil
		case 1:
			return "Critical", nil
		case 2:
			return "Error", nil
		case 3:
			return "Warning", nil
		case 4:
			return "Information", nil
		case 5:
			return "Verbose", nil
		default:
			return fmt.Sprintf("Level %d", event.System.Level), nil
		}
	case EvtFormatMessageTask:
		if event.RenderingInfo.Task != "" {
			return event.RenderingInfo.Task, nil
		}
		return fmt.Sprintf("Task %d", event.System.Task), nil
	case EvtFormatMessageOpcode:
		if event.RenderingInfo.Opcode != "" {
			return event.RenderingInfo.Opcode, nil
		}
		// Map common opcodes
		switch event.System.Opcode {
		case 0:
			return "Info", nil
		case 1:
			return "Start", nil
		case 2:
			return "Stop", nil
		default:
			return fmt.Sprintf("Opcode %d", event.System.Opcode), nil
		}
	case EvtFormatMessageKeyword:
		if len(event.RenderingInfo.Keywords.Keyword) > 0 {
			return strings.Join(event.RenderingInfo.Keywords.Keyword, ", "), nil
		}
		return event.System.Keywords, nil
	case EvtFormatMessageChannel:
		if event.RenderingInfo.Channel != "" {
			return event.RenderingInfo.Channel, nil
		}
		return event.System.Channel, nil
	case EvtFormatMessageProvider:
		if event.RenderingInfo.Provider != "" {
			return event.RenderingInfo.Provider, nil
		}
		return event.System.Provider.Name, nil
	case EvtFormatMessageId:
		return fmt.Sprintf("%d", event.System.EventID.Value), nil
	default:
		return "", fmt.Errorf("Unsupported format flag: %d", format)
	}
}

/* Get the formatted string that represents this message. This method uses XML parsing instead of EvtFormatMessage. */
func FormatMessage(eventPublisherHandle PublisherHandle, eventHandle EventHandle, format EVT_FORMAT_MESSAGE_FLAGS) (string, error) {
	// Get the XML representation of the event
	xml, err := RenderEventXML(eventHandle)
	if err != nil {
		return "", fmt.Errorf("Failed to render event XML: %v", err)
	}

	// Extract the requested information from the XML
	return ExtractFromXML(xml, format)
}

/* Get the formatted string for the last error which occurred. Wraps GetLastError and FormatMessage. */
func GetLastError() error {
	return syscall.GetLastError()
}

/* Render the system properties from the event and returns an array of properties.
   Properties can be accessed using RenderStringField, RenderIntField, RenderFileTimeField,
   or RenderUIntField depending on type. This buffer must be freed after use. */
func RenderEventValues(renderContext SysRenderContext, eventHandle EventHandle) (EvtVariant, error) {
	// Create a copy of the event handle to prevent access issues if the event is modified by Windows
	eventCopy, err := EvtCreateEventCopy(syscall.Handle(eventHandle))
	if err != nil {
		return nil, fmt.Errorf("Failed to create event copy for rendering values: %v", err)
	}
	defer CloseEventHandle(uint64(eventCopy))

	var bufferUsed uint32 = 0
	var propertyCount uint32 = 0
	err = EvtRender(syscall.Handle(renderContext), eventCopy, EvtRenderEventValues, 0, nil, &bufferUsed, &propertyCount)
	if bufferUsed == 0 {
		return nil, err
	}
	buffer := make([]byte, bufferUsed)
	bufSize := bufferUsed
	err = EvtRender(syscall.Handle(renderContext), eventCopy, EvtRenderEventValues, bufSize, (*uint16)(unsafe.Pointer(&buffer[0])), &bufferUsed, &propertyCount)
	if err != nil {
		return nil, err
	}
	return NewEvtVariant(buffer), nil
}

// Render the event as XML.
func RenderEventXML(eventHandle EventHandle) (string, error) {
	// Create a copy of the event handle to prevent access issues if the event is modified by Windows
	eventCopy, err := EvtCreateEventCopy(syscall.Handle(eventHandle))
	if err != nil {
		return "", fmt.Errorf("Failed to create event copy for XML rendering: %v", err)
	}
	defer CloseEventHandle(uint64(eventCopy))

	var bufferUsed, propertyCount uint32

	err = EvtRender(0, eventCopy, EvtRenderEventXml, 0, nil, &bufferUsed, &propertyCount)

	if bufferUsed == 0 {
		return "", err
	}

	buffer := make([]byte, bufferUsed)
	bufSize := bufferUsed

	err = EvtRender(0, eventCopy, EvtRenderEventXml, bufSize, (*uint16)(unsafe.Pointer(&buffer[0])), &bufferUsed, &propertyCount)
	if err != nil {
		return err.Error(), err
	}

	// Remove null bytes
	xml := bytes.Replace(buffer, []byte("\x00"), []byte{}, -1)

	return string(xml), nil
}

/* Get a handle that represents the publisher of the event, given the rendered event values. */
func GetEventPublisherHandle(renderedFields EvtVariant) (PublisherHandle, error) {
	publisher, err := renderedFields.String(EvtSystemProviderName)
	if err != nil {
		return 0, err
	}
	widePublisher, err := syscall.UTF16PtrFromString(publisher)
	if err != nil {
		return 0, err
	}
	handle, err := EvtOpenPublisherMetadata(0, widePublisher, nil, 0, 0)
	if err != nil {
		return 0, err
	}
	return PublisherHandle(handle), nil
}

/* Close an event handle. */
func CloseEventHandle(handle uint64) error {
	return EvtClose(syscall.Handle(handle))
}

/* Cancel pending actions on the event handle. */
func CancelEventHandle(handle uint64) error {
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
	cbWrap := (*LogEventCallbackWrapper)(Context)
	if Action == 0 {
		cbWrap.callback.PublishError(fmt.Errorf("Event log callback got error: %v", GetLastError()))
	} else {
		// Create a copy of the event handle to prevent access issues if the event is modified by Windows
		eventCopy, err := EvtCreateEventCopy(handle)
		if err != nil {
			cbWrap.callback.PublishError(fmt.Errorf("Failed to create event copy in callback: %v", err))
			return 0
		}
		// Use the copied event handle for all operations
		cbWrap.callback.PublishEvent(EventHandle(eventCopy), cbWrap.subscribedChannel)
	}
	return 0
}

// CreateMap converts the WinLogEvent to a map[string]interface{}
func (ev *WinLogEvent) CreateMap() map[string]interface{} {
	toReturn := make(map[string]interface{})
	toReturn["Xml"] = ev.Xml
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
