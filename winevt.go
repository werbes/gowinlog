//go:build windows
// +build windows

package winlog

import (
	"fmt"
	"sync"
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

	// Mutex to protect access to Windows API functions that might not be thread-safe
	winAPILock sync.Mutex
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
	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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

	// Additional checks for other parameters
	if BufferUsed == nil {
		return fmt.Errorf("BufferUsed pointer is nil")
	}

	if PropertyCount == nil {
		return fmt.Errorf("PropertyCount pointer is nil")
	}

	// If BufferSize > 0, Buffer must not be nil
	if BufferSize > 0 && Buffer == nil {
		return fmt.Errorf("Buffer is nil but BufferSize is %d", BufferSize)
	}

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

	// Create a safe pointer for Buffer and ensure BufferSize is 0 if Buffer is nil
	var bufferPtr uintptr = 0 // Initialize to 0 (NULL)
	var safeBufferSize uint32 = 0 // Default to 0

	// Only set bufferPtr and safeBufferSize if Buffer is not nil
	if Buffer != nil {
		bufferPtr = uintptr(unsafe.Pointer(Buffer))
		safeBufferSize = BufferSize
	}

	// Instead of using a goroutine, we'll use a simpler approach with direct error handling
	// This avoids potential issues with CGO and goroutines
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtRender: %v", r)
			}
		}()

		// Additional defensive check to ensure we don't pass invalid parameters to the Windows API
		if Fragment == 0 {
			callErr = fmt.Errorf("invalid fragment handle: %v", Fragment)
			return
		}

		// Additional defensive check for BufferUsed and PropertyCount
		if BufferUsed == nil || PropertyCount == nil {
			callErr = fmt.Errorf("BufferUsed or PropertyCount pointer is nil")
			return
		}

		// Make the API call with explicit nil checks for all parameters
		var contextPtr uintptr = uintptr(Context)
		var fragmentPtr uintptr = uintptr(Fragment)
		var bufferUsedPtr uintptr = 0
		var propertyCountPtr uintptr = 0

		if BufferUsed != nil {
			bufferUsedPtr = uintptr(unsafe.Pointer(BufferUsed))
		} else {
			callErr = fmt.Errorf("BufferUsed pointer is nil")
			return
		}

		if PropertyCount != nil {
			propertyCountPtr = uintptr(unsafe.Pointer(PropertyCount))
		} else {
			callErr = fmt.Errorf("PropertyCount pointer is nil")
			return
		}

		// Ensure we're not passing any invalid pointers to the Windows API
		// This is a belt-and-suspenders approach to prevent crashes
		if bufferPtr == 0 {
			safeBufferSize = 0 // Ensure BufferSize is 0 if Buffer is nil
		}

		// Ensure Context is valid (0 is allowed for some flags)
		if Context == 0 && Flags != EvtRenderEventXml && Flags != EvtRenderBookmark {
			callErr = fmt.Errorf("invalid context handle for flags: %v", Flags)
			return
		}

		// Ensure Fragment is valid
		if fragmentPtr == 0 {
			callErr = fmt.Errorf("invalid fragment handle: 0")
			return
		}

		// Ensure BufferUsed and PropertyCount are not nil before making the call
		if bufferUsedPtr == 0 {
			callErr = fmt.Errorf("BufferUsed pointer is nil")
			return
		}

		if propertyCountPtr == 0 {
			callErr = fmt.Errorf("PropertyCount pointer is nil")
			return
		}

		r1, _, err := evtRender.Call(
			contextPtr,
			fragmentPtr,
			uintptr(Flags),
			uintptr(safeBufferSize),
			bufferPtr,
			bufferUsedPtr,
			propertyCountPtr,
		)

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

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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
	if PublisherMetadata == 0 {
		return fmt.Errorf("invalid publisher metadata handle: 0")
	}
	if Event == 0 {
		return fmt.Errorf("invalid event handle: 0")
	}
	if BufferUsed == nil {
		return fmt.Errorf("BufferUsed pointer is nil")
	}

	// If BufferSize > 0, Buffer must not be nil
	if BufferSize > 0 && Buffer == nil {
		return fmt.Errorf("Buffer is nil but BufferSize is %d", BufferSize)
	}

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

	// Use defer/recover to catch any panics during the call
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic in EvtFormatMessage: %v", r)
			}
		}()

		// Create safe pointers for all parameters
		var bufferPtr uintptr = 0
		var valuesPtr uintptr = 0
		var safeBufferSize uint32 = 0
		var bufferUsedPtr uintptr = 0

		// Only set bufferPtr and safeBufferSize if Buffer is not nil
		if Buffer != nil {
			bufferPtr = uintptr(unsafe.Pointer(Buffer))
			safeBufferSize = BufferSize
		} else {
			// If Buffer is nil, ensure BufferSize is 0 to prevent crashes
			safeBufferSize = 0
		}

		// Only set valuesPtr if Values is not nil and ValueCount > 0
		// This is a critical check to prevent crashes when Values is nil but ValueCount is non-zero
		// or when Values is non-nil but ValueCount is 0
		if Values != nil && ValueCount > 0 {
			valuesPtr = uintptr(unsafe.Pointer(Values))
		} else {
			// If Values is nil or ValueCount is 0, ensure both are consistent
			// to prevent crashes in the Windows API
			valuesPtr = 0
			ValueCount = 0
		}

		// Set bufferUsedPtr only if BufferUsed is not nil (already checked above)
		bufferUsedPtr = uintptr(unsafe.Pointer(BufferUsed))

		// Additional defensive check before making the API call
		if PublisherMetadata == 0 || Event == 0 || bufferUsedPtr == 0 {
			callErr = fmt.Errorf("invalid parameters for EvtFormatMessage: PublisherMetadata=%v, Event=%v, BufferUsed=%v", 
				PublisherMetadata, Event, bufferUsedPtr)
			return
		}

		// Use a separate try/catch block for the actual API call
		func() {
			defer func() {
				if r := recover(); r != nil {
					callErr = fmt.Errorf("panic during API call in EvtFormatMessage: %v", r)
				}
			}()

			r1, _, err := evtFormatMessage.Call(
				uintptr(PublisherMetadata),
				uintptr(Event),
				uintptr(MessageId),
				uintptr(ValueCount),
				valuesPtr,
				uintptr(Flags),
				uintptr(safeBufferSize),
				bufferPtr,
				bufferUsedPtr,
			)

			if r1 == 0 {
				callErr = err
			}
		}()
	}()

	return callErr
}

func EvtCreateRenderContext(ValuePathsCount uint32, ValuePaths uintptr, Flags uint32) (syscall.Handle, error) {
	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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
	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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

	// Lock the mutex to prevent concurrent access to the Windows API
	winAPILock.Lock()
	defer winAPILock.Unlock()

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
