//go:build windows
// +build windows

package winlog

import (
	"fmt"
	"time"
	"sync"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// WMIEventLogWatcher is a watcher for Windows event logs using WMI
type WMIEventLogWatcher struct {
	errChan   chan error
	eventChan chan *WinLogEvent
	shutdown  chan interface{}
	watches   map[string]*wmiChannelWatcher
	watchMutex sync.Mutex

	// Optionally render localized fields
	RenderKeywords bool
	RenderMessage  bool
	RenderLevel    bool
	RenderTask     bool
	RenderProvider bool
	RenderOpcode   bool
	RenderChannel  bool
	RenderId       bool
}

type wmiChannelWatcher struct {
	channel    string
	query      string
	lastRecord uint64
	stopChan   chan interface{}
}

// NewWMIEventLogWatcher creates a new WMI-based event log watcher
func NewWMIEventLogWatcher() (*WMIEventLogWatcher, error) {
	return &WMIEventLogWatcher{
		shutdown:       make(chan interface{}),
		errChan:        make(chan error),
		eventChan:      make(chan *WinLogEvent),
		watches:        make(map[string]*wmiChannelWatcher),
		RenderKeywords: true,
		RenderMessage:  true,
		RenderLevel:    true,
		RenderTask:     true,
		RenderProvider: true,
		RenderOpcode:   true,
		RenderChannel:  true,
		RenderId:       true,
	}, nil
}

// Event returns the event channel
func (w *WMIEventLogWatcher) Event() <-chan *WinLogEvent {
	return w.eventChan
}

// Error returns the error channel
func (w *WMIEventLogWatcher) Error() <-chan error {
	return w.errChan
}

// SubscribeFromBeginning subscribes to a Windows Event Log channel, starting with the first event
func (w *WMIEventLogWatcher) SubscribeFromBeginning(channel, query string) error {
	return w.subscribe(channel, query, 0)
}

// SubscribeFromNow subscribes to a Windows Event Log channel, starting with the next event
func (w *WMIEventLogWatcher) SubscribeFromNow(channel, query string) error {
	// Get the current record number
	lastRecord, err := w.getLastRecordNumber(channel)
	if err != nil {
		return err
	}
	return w.subscribe(channel, query, lastRecord)
}

// subscribe subscribes to a Windows Event Log channel
func (w *WMIEventLogWatcher) subscribe(channel, query string, startRecord uint64) error {
	w.watchMutex.Lock()
	defer w.watchMutex.Unlock()

	if _, ok := w.watches[channel]; ok {
		return fmt.Errorf("a watcher for channel %q already exists", channel)
	}

	stopChan := make(chan interface{})
	w.watches[channel] = &wmiChannelWatcher{
		channel:    channel,
		query:      query,
		lastRecord: startRecord,
		stopChan:   stopChan,
	}

	go w.watchEvents(channel, query, startRecord, stopChan)
	return nil
}

// RemoveSubscription removes a subscription
func (w *WMIEventLogWatcher) RemoveSubscription(channel string) error {
	w.watchMutex.Lock()
	defer w.watchMutex.Unlock()

	if watch, ok := w.watches[channel]; ok {
		close(watch.stopChan)
		delete(w.watches, channel)
	}

	return nil
}

// Shutdown removes all subscriptions and shuts down the watcher
func (w *WMIEventLogWatcher) Shutdown() {
	close(w.shutdown)
	for channel := range w.watches {
		w.RemoveSubscription(channel)
	}
	close(w.errChan)
	close(w.eventChan)
}

// PublishError publishes an error to the error channel
func (w *WMIEventLogWatcher) PublishError(err error) {
	select {
	case w.errChan <- err:
	case <-w.shutdown:
	}
}

// getLastRecordNumber gets the last record number for a channel
func (w *WMIEventLogWatcher) getLastRecordNumber(channel string) (uint64, error) {
	// Initialize COM
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize COM: %v", err)
	}
	defer ole.CoUninitialize()

	// Connect to WMI
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return 0, fmt.Errorf("failed to create WMI locator: %v", err)
	}
	defer unknown.Release()

	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return 0, fmt.Errorf("failed to query WMI interface: %v", err)
	}
	defer wmi.Release()

	// Connect to the local server
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", ".", "root\\cimv2")
	if err != nil {
		return 0, fmt.Errorf("failed to connect to WMI server: %v", err)
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	// Query for the last event in the log
	wqlChannel := strings.Replace(channel, "Microsoft-Windows-", "Win32_NTLog", 1)
	query := fmt.Sprintf("SELECT * FROM Win32_NTLogEvent WHERE LogFile='%s' ORDER BY RecordNumber DESC", wqlChannel)
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", query)
	if err != nil {
		return 0, fmt.Errorf("failed to execute WMI query: %v", err)
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	// Get the first item (which is the last event)
	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return 0, fmt.Errorf("failed to get result count: %v", err)
	}
	count := int(countVar.Val)
	if count == 0 {
		return 0, nil
	}

	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
	if err != nil {
		return 0, fmt.Errorf("failed to get first item: %v", err)
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()

	// Get the record number
	recordNumberVar, err := oleutil.GetProperty(item, "RecordNumber")
	if err != nil {
		return 0, fmt.Errorf("failed to get record number: %v", err)
	}

	return uint64(recordNumberVar.Val), nil
}

// watchEvents watches for new events on a channel
func (w *WMIEventLogWatcher) watchEvents(channel, query string, startRecord uint64, stopChan chan interface{}) {
	// Initialize COM
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		w.PublishError(fmt.Errorf("failed to initialize COM: %v", err))
		return
	}
	defer ole.CoUninitialize()

	// Connect to WMI
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		w.PublishError(fmt.Errorf("failed to create WMI locator: %v", err))
		return
	}
	defer unknown.Release()

	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		w.PublishError(fmt.Errorf("failed to query WMI interface: %v", err))
		return
	}
	defer wmi.Release()

	// Connect to the local server
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", ".", "root\\cimv2")
	if err != nil {
		w.PublishError(fmt.Errorf("failed to connect to WMI server: %v", err))
		return
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	// Convert channel name to WMI format
	wqlChannel := strings.Replace(channel, "Microsoft-Windows-", "Win32_NTLog", 1)

	// Poll for new events
	lastRecord := startRecord
	for {
		select {
		case <-stopChan:
			return
		case <-w.shutdown:
			return
		default:
			// Query for new events
			wqlQuery := fmt.Sprintf("SELECT * FROM Win32_NTLogEvent WHERE LogFile='%s' AND RecordNumber > %d ORDER BY RecordNumber", wqlChannel, lastRecord)
			resultRaw, err := oleutil.CallMethod(service, "ExecQuery", wqlQuery)
			if err != nil {
				w.PublishError(fmt.Errorf("failed to execute WMI query: %v", err))
				time.Sleep(5 * time.Second)
				continue
			}
			result := resultRaw.ToIDispatch()

			// Process the results
			countVar, err := oleutil.GetProperty(result, "Count")
			if err != nil {
				w.PublishError(fmt.Errorf("failed to get result count: %v", err))
				result.Release()
				time.Sleep(5 * time.Second)
				continue
			}
			count := int(countVar.Val)

			// Process each event
			for i := 0; i < count; i++ {
				itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
				if err != nil {
					w.PublishError(fmt.Errorf("failed to get item %d: %v", i, err))
					continue
				}
				item := itemRaw.ToIDispatch()

				// Convert the WMI event to a WinLogEvent
				event, err := w.convertWMIEventToWinLogEvent(item, channel)
				if err != nil {
					w.PublishError(fmt.Errorf("failed to convert event: %v", err))
					item.Release()
					continue
				}

				// Update the last record number
				if event.RecordId > lastRecord {
					lastRecord = event.RecordId
				}

				// Publish the event
				select {
				case w.eventChan <- event:
				case <-stopChan:
					item.Release()
					result.Release()
					return
				case <-w.shutdown:
					item.Release()
					result.Release()
					return
				}

				item.Release()
			}

			result.Release()
			time.Sleep(5 * time.Second)
		}
	}
}

// convertWMIEventToWinLogEvent converts a WMI event to a WinLogEvent
func (w *WMIEventLogWatcher) convertWMIEventToWinLogEvent(item *ole.IDispatch, subscribedChannel string) (*WinLogEvent, error) {
	// Get event properties
	recordNumberVar, err := oleutil.GetProperty(item, "RecordNumber")
	if err != nil {
		return nil, fmt.Errorf("failed to get record number: %v", err)
	}
	recordNumber := uint64(recordNumberVar.Val)

	sourceNameVar, err := oleutil.GetProperty(item, "SourceName")
	if err != nil {
		return nil, fmt.Errorf("failed to get source name: %v", err)
	}
	sourceName := sourceNameVar.ToString()

	timeGeneratedVar, err := oleutil.GetProperty(item, "TimeGenerated")
	if err != nil {
		return nil, fmt.Errorf("failed to get time generated: %v", err)
	}
	timeGenerated := timeGeneratedVar.ToString()
	created, err := time.Parse("20060102150405.000000-070", timeGenerated)
	if err != nil {
		created = time.Now()
	}

	eventTypeVar, err := oleutil.GetProperty(item, "EventType")
	if err != nil {
		return nil, fmt.Errorf("failed to get event type: %v", err)
	}
	eventType := uint64(eventTypeVar.Val)

	eventCodeVar, err := oleutil.GetProperty(item, "EventCode")
	if err != nil {
		return nil, fmt.Errorf("failed to get event code: %v", err)
	}
	eventCode := uint64(eventCodeVar.Val)

	messageVar, err := oleutil.GetProperty(item, "Message")
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %v", err)
	}
	message := messageVar.ToString()

	computerNameVar, err := oleutil.GetProperty(item, "ComputerName")
	if err != nil {
		return nil, fmt.Errorf("failed to get computer name: %v", err)
	}
	computerName := computerNameVar.ToString()

	// Map event type to level
	var level uint64
	var levelText string
	switch eventType {
	case 1:
		level = 1
		levelText = "Error"
	case 2:
		level = 2
		levelText = "Warning"
	case 3:
		level = 4
		levelText = "Information"
	case 4:
		level = 0
		levelText = "Security Audit Success"
	case 5:
		level = 0
		levelText = "Security Audit Failure"
	default:
		level = 0
		levelText = fmt.Sprintf("Unknown (%d)", eventType)
	}

	// Create the event
	event := &WinLogEvent{
		ProviderName:      sourceName,
		EventId:           eventCode,
		Level:             level,
		Created:           created,
		RecordId:          recordNumber,
		ComputerName:      computerName,
		SubscribedChannel: subscribedChannel,
		Msg:               message,
		LevelText:         levelText,
		ProviderText:      sourceName,
		IdText:            fmt.Sprintf("%d", eventCode),
	}

	return event, nil
}