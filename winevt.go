//go:build windows
// +build windows

package winlog

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

/* Interop code for wevtapi.dll */

var (
	evtCreateBookmark        *windows.LazyProc
	evtUpdateBookmark        *windows.LazyProc
	evtRender                *windows.LazyProc
	evtClose                 *windows.LazyProc
	evtCancel                *windows.LazyProc
	evtFormatMessage         *windows.LazyProc
	evtCreateRenderContext   *windows.LazyProc
	evtSubscribe             *windows.LazyProc
	evtQuery                 *windows.LazyProc
	evtOpenPublisherMetadata *windows.LazyProc
	evtNext                  *windows.LazyProc
)

func mustFindProc(mod *windows.LazyDLL, functionName string) *windows.LazyProc {
	if mod.Load() != nil {
		panic(fmt.Sprintf("error loading %v", mod.Name))
	}
	proc := mod.NewProc(functionName)
	if proc == nil || proc.Find() != nil {
		panic(fmt.Sprintf("missing %v from %v", functionName, mod.Name))
	}
	return proc
}

func init() {
	winevtDll := windows.NewLazySystemDLL("wevtapi.dll")
	evtCreateBookmark = mustFindProc(winevtDll, "EvtCreateBookmark")
	evtUpdateBookmark = mustFindProc(winevtDll, "EvtUpdateBookmark")
	evtRender = mustFindProc(winevtDll, "EvtRender")
	evtClose = mustFindProc(winevtDll, "EvtClose")
	evtCancel = mustFindProc(winevtDll, "EvtCancel")
	evtFormatMessage = mustFindProc(winevtDll, "EvtFormatMessage")
	evtCreateRenderContext = mustFindProc(winevtDll, "EvtCreateRenderContext")
	evtSubscribe = mustFindProc(winevtDll, "EvtSubscribe")
	evtQuery = mustFindProc(winevtDll, "EvtQuery")
	evtOpenPublisherMetadata = mustFindProc(winevtDll, "EvtOpenPublisherMetadata")
	evtNext = mustFindProc(winevtDll, "EvtNext")
}

type EVT_SUBSCRIBE_FLAGS int

const (
	_ = iota
	EvtSubscribeToFutureEvents
	EvtSubscribeStartAtOldestRecord
	EvtSubscribeStartAfterBookmark
)

/* Fields that can be rendered with GetRendered*Value */
type EVT_SYSTEM_PROPERTY_ID int

const (
	EvtSystemProviderName = iota
	EvtSystemProviderGuid
	EvtSystemEventID
	EvtSystemQualifiers
	EvtSystemLevel
	EvtSystemTask
	EvtSystemOpcode
	EvtSystemKeywords
	EvtSystemTimeCreated
	EvtSystemEventRecordId
	EvtSystemActivityID
	EvtSystemRelatedActivityID
	EvtSystemProcessID
	EvtSystemThreadID
	EvtSystemChannel
	EvtSystemComputer
	EvtSystemUserID
	EvtSystemVersion
)

/* Formatting modes for GetFormattedMessage */
type EVT_FORMAT_MESSAGE_FLAGS int

const (
	_ = iota
	EvtFormatMessageEvent
	EvtFormatMessageLevel
	EvtFormatMessageTask
	EvtFormatMessageOpcode
	EvtFormatMessageKeyword
	EvtFormatMessageChannel
	EvtFormatMessageProvider
	EvtFormatMessageId
	EvtFormatMessageXml
)

type EVT_RENDER_FLAGS uint32

const (
	EvtRenderEventValues = iota
	EvtRenderEventXml
	EvtRenderBookmark
)

type EVT_RENDER_CONTEXT_FLAGS uint32

const (
	EvtRenderContextValues = iota
	EvtRenderContextSystem
	EvtRenderContextUser
)

type EVT_QUERY_FLAGS uint32

const (
	EvtQueryChannelPath         = 0x1
	EvtQueryFilePath            = 0x2
	EvtQueryForwardDirection    = 0x100
	EvtQueryReverseDirection    = 0x200
	EvtQueryTolerateQueryErrors = 0x1000
)

func EvtCreateBookmark(BookmarkXml *uint16) (syscall.Handle, error) {
	// Use defer/recover to catch any panics during the call
	var handle syscall.Handle
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtCreateBookmark: %v", r)
			}
		}()

		r1, _, err := evtCreateBookmark.Call(uintptr(unsafe.Pointer(BookmarkXml)))
		if r1 == 0 {
			callErr = err
		} else {
			handle = syscall.Handle(r1)
		}
	}()

	if callErr != nil {
		return 0, callErr
	}
	return handle, nil
}

func EvtUpdateBookmark(Bookmark, Event syscall.Handle) error {
	// Add defensive checks to prevent crashes
	if Bookmark == 0 {
		return fmt.Errorf("invalid bookmark handle: 0")
	}
	if Event == 0 {
		return fmt.Errorf("invalid event handle: 0")
	}

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtUpdateBookmark: %v", r)
			}
		}()

		r1, _, err := evtUpdateBookmark.Call(uintptr(Bookmark), uintptr(Event))
		if r1 == 0 {
			callErr = err
		}
	}()

	return callErr
}

func EvtRender(Context, Fragment syscall.Handle, Flags, BufferSize uint32, Buffer *uint16, BufferUsed, PropertyCount *uint32) error {
	// Add defensive checks to prevent crashes
	if Fragment == 0 {
		return fmt.Errorf("invalid fragment handle: %v", Fragment)
	}

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtRender: %v", r)
			}
		}()

		r1, _, err := evtRender.Call(uintptr(Context), uintptr(Fragment), uintptr(Flags), uintptr(BufferSize), uintptr(unsafe.Pointer(Buffer)), uintptr(unsafe.Pointer(BufferUsed)), uintptr(unsafe.Pointer(PropertyCount)))
		if r1 == 0 {
			callErr = err
		}
	}()

	return callErr
}

func EvtClose(Object syscall.Handle) error {
	// Add defensive checks to prevent crashes
	if Object == 0 {
		return fmt.Errorf("invalid object handle: 0")
	}

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtClose: %v", r)
			}
		}()

		r1, _, err := evtClose.Call(uintptr(Object))
		if r1 == 0 {
			callErr = err
		}
	}()

	return callErr
}

func EvtFormatMessage(PublisherMetadata, Event syscall.Handle, MessageId, ValueCount uint32, Values *byte, Flags, BufferSize uint32, Buffer *uint16, BufferUsed *uint32) error {
	// Add defensive checks to prevent crashes
	if PublisherMetadata == 0 || Event == 0 {
		return fmt.Errorf("invalid handle: PublisherMetadata=%v, Event=%v", PublisherMetadata, Event)
	}

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtFormatMessage: %v", r)
			}
		}()

		r1, _, err := evtFormatMessage.Call(uintptr(PublisherMetadata), uintptr(Event), uintptr(MessageId), uintptr(ValueCount), uintptr(unsafe.Pointer(Values)), uintptr(Flags), uintptr(BufferSize), uintptr(unsafe.Pointer(Buffer)), uintptr(unsafe.Pointer(BufferUsed)))
		if r1 == 0 {
			callErr = err
		}
	}()

	return callErr
}

func EvtCreateRenderContext(ValuePathsCount uint32, ValuePaths uintptr, Flags uint32) (syscall.Handle, error) {
	// Use defer/recover to catch any panics during the call
	var handle syscall.Handle
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtCreateRenderContext: %v", r)
			}
		}()

		r1, _, err := evtCreateRenderContext.Call(uintptr(ValuePathsCount), ValuePaths, uintptr(Flags))
		if r1 == 0 {
			callErr = err
		} else {
			handle = syscall.Handle(r1)
		}
	}()

	if callErr != nil {
		return 0, callErr
	}
	return handle, nil
}

func EvtSubscribe(Session, SignalEvent syscall.Handle, ChannelPath, Query *uint16, Bookmark syscall.Handle, context uintptr, Callback uintptr, Flags uint32) (syscall.Handle, error) {
	// Add defensive checks to prevent crashes
	if ChannelPath == nil {
		return 0, fmt.Errorf("channel path is nil")
	}

	// Use defer/recover to catch any panics during the call
	var handle syscall.Handle
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtSubscribe: %v", r)
			}
		}()

		r1, _, err := evtSubscribe.Call(uintptr(Session), uintptr(SignalEvent), uintptr(unsafe.Pointer(ChannelPath)), uintptr(unsafe.Pointer(Query)), uintptr(Bookmark), context, Callback, uintptr(Flags))
		if r1 == 0 {
			callErr = err
		} else {
			handle = syscall.Handle(r1)
		}
	}()

	if callErr != nil {
		return 0, callErr
	}
	return handle, nil
}

func EvtQuery(Session syscall.Handle, Path, Query *uint16, Flags uint32) (syscall.Handle, error) {
	// Use defer/recover to catch any panics during the call
	var handle syscall.Handle
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtQuery: %v", r)
			}
		}()

		r1, _, err := evtQuery.Call(uintptr(Session), uintptr(unsafe.Pointer(Path)), uintptr(unsafe.Pointer(Query)), uintptr(Flags))
		if r1 == 0 {
			callErr = err
		} else {
			handle = syscall.Handle(r1)
		}
	}()

	if callErr != nil {
		return 0, callErr
	}
	return handle, nil
}

func EvtOpenPublisherMetadata(Session syscall.Handle, PublisherIdentity, LogFilePath *uint16, Locale, Flags uint32) (syscall.Handle, error) {
	// Add defensive checks to prevent crashes
	if PublisherIdentity == nil {
		return 0, fmt.Errorf("invalid publisher identity: nil")
	}

	// Use defer/recover to catch any panics during the call
	var handle syscall.Handle
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtOpenPublisherMetadata: %v", r)
			}
		}()

		r1, _, err := evtOpenPublisherMetadata.Call(uintptr(Session), uintptr(unsafe.Pointer(PublisherIdentity)), uintptr(unsafe.Pointer(LogFilePath)), uintptr(Locale), uintptr(Flags))
		if r1 == 0 {
			callErr = err
		} else {
			handle = syscall.Handle(r1)
		}
	}()

	if callErr != nil {
		return 0, callErr
	}
	return handle, nil
}

func EvtCancel(handle syscall.Handle) error {
	// Add defensive checks to prevent crashes
	if handle == 0 {
		return fmt.Errorf("invalid handle: 0")
	}

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtCancel: %v", r)
			}
		}()

		r1, _, err := evtCancel.Call(uintptr(handle))
		if r1 == 0 {
			callErr = err
		}
	}()

	return callErr
}

func EvtNext(ResultSet syscall.Handle, EventArraySize uint32, EventArray *syscall.Handle, Timeout, Flags uint32, Returned *uint32) error {
	// Add defensive checks to prevent crashes
	if ResultSet == 0 {
		return fmt.Errorf("invalid result set handle: 0")
	}
	if EventArray == nil {
		return fmt.Errorf("event array is nil")
	}
	if Returned == nil {
		return fmt.Errorf("returned pointer is nil")
	}

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtNext: %v", r)
			}
		}()

		r1, _, err := evtNext.Call(uintptr(ResultSet), uintptr(EventArraySize), uintptr(unsafe.Pointer(EventArray)), uintptr(Timeout), uintptr(Flags), uintptr(unsafe.Pointer(Returned)))
		if r1 == 0 {
			callErr = err
		}
	}()

	return callErr
}
