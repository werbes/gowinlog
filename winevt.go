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
	r1, _, err := evtCreateBookmark.Call(uintptr(unsafe.Pointer(BookmarkXml)))
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func EvtUpdateBookmark(Bookmark, Event syscall.Handle) error {
	r1, _, err := evtUpdateBookmark.Call(uintptr(Bookmark), uintptr(Event))
	if r1 == 0 {
		return err
	}
	return nil
}

func EvtRender(Context, Fragment syscall.Handle, Flags, BufferSize uint32, Buffer *uint16, BufferUsed, PropertyCount *uint32) error {
	r1, _, err := evtRender.Call(uintptr(Context), uintptr(Fragment), uintptr(Flags), uintptr(BufferSize), uintptr(unsafe.Pointer(Buffer)), uintptr(unsafe.Pointer(BufferUsed)), uintptr(unsafe.Pointer(PropertyCount)))
	if r1 == 0 {
		return err
	}
	return nil
}

func EvtClose(Object syscall.Handle) error {
	r1, _, err := evtClose.Call(uintptr(Object))
	if r1 == 0 {
		return err
	}
	return nil
}

// EvtFormatMessage safely wraps the Windows API function
func EvtFormatMessage(
	publisherMetadata syscall.Handle,
	event syscall.Handle,
	messageId uint32,
	valueCount uint32,
	values *uint16,
	flags EVT_FORMAT_MESSAGE_FLAGS,
	bufferSize uint32,
	buffer *uint16,
	bufferUsed *uint32,
) error {
	r1, _, err := evtFormatMessage.Call(
		uintptr(publisherMetadata),
		uintptr(event),
		uintptr(messageId),
		uintptr(valueCount),
		uintptr(unsafe.Pointer(values)),
		uintptr(flags),
		uintptr(bufferSize),
		uintptr(unsafe.Pointer(buffer)),
		uintptr(unsafe.Pointer(bufferUsed)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

func EvtCreateRenderContext(ValuePathsCount uint32, ValuePaths uintptr, Flags uint32) (syscall.Handle, error) {
	r1, _, err := evtCreateRenderContext.Call(uintptr(ValuePathsCount), ValuePaths, uintptr(Flags))
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func EvtSubscribe(Session, SignalEvent syscall.Handle, ChannelPath, Query *uint16, Bookmark syscall.Handle, context uintptr, Callback uintptr, Flags uint32) (syscall.Handle, error) {
	r1, _, err := evtSubscribe.Call(uintptr(Session), uintptr(SignalEvent), uintptr(unsafe.Pointer(ChannelPath)), uintptr(unsafe.Pointer(Query)), uintptr(Bookmark), context, Callback, uintptr(Flags))
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func EvtQuery(Session syscall.Handle, Path, Query *uint16, Flags uint32) (syscall.Handle, error) {
	r1, _, err := evtQuery.Call(uintptr(Session), uintptr(unsafe.Pointer(Path)), uintptr(unsafe.Pointer(Query)), uintptr(Flags))
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func EvtOpenPublisherMetadata(Session syscall.Handle, PublisherIdentity, LogFilePath *uint16, Locale, Flags uint32) (syscall.Handle, error) {
	r1, _, err := evtOpenPublisherMetadata.Call(uintptr(Session), uintptr(unsafe.Pointer(PublisherIdentity)), uintptr(unsafe.Pointer(LogFilePath)), uintptr(Locale), uintptr(Flags))
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func EvtCancel(handle syscall.Handle) error {
	r1, _, err := evtCancel.Call(uintptr(handle))
	if r1 == 0 {
		return err
	}
	return nil
}

func EvtNext(ResultSet syscall.Handle, EventArraySize uint32, EventArray *syscall.Handle, Timeout, Flags uint32, Returned *uint32) error {
	r1, _, err := evtNext.Call(uintptr(ResultSet), uintptr(EventArraySize), uintptr(unsafe.Pointer(EventArray)), uintptr(Timeout), uintptr(Flags), uintptr(unsafe.Pointer(Returned)))
	if r1 == 0 {
		return err
	}
	return nil
}
