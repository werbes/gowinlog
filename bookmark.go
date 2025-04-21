// +build windows

package winlog

import (
	"fmt"
	"sync"
	"syscall"
)

// bookmarkMutex protects access to bookmark handles
var bookmarkMutex sync.Mutex

/*Bookmarks allow you to remember a specific event and restore subscription at that point of time*/

/* Create a new, empty bookmark. Bookmark handles must be closed with CloseEventHandle. */
func CreateBookmark() (BookmarkHandle, error) {
	bookmark, err := EvtCreateBookmark(nil)
	if err != nil {
		return 0, err
	}
	return BookmarkHandle(bookmark), nil
}

/* Create a bookmark from a XML-serialized bookmark. Bookmark handles must be closed with CloseEventHandle. */
func CreateBookmarkFromXml(xmlString string) (BookmarkHandle, error) {
	wideXmlString, err := syscall.UTF16PtrFromString(xmlString)
	if err != nil {
		return 0, err
	}
	bookmark, err := EvtCreateBookmark(wideXmlString)
	if bookmark == 0 {
		return 0, err
	}
	return BookmarkHandle(bookmark), nil
}

/* Update a bookmark to store the channel and ID of the given event */
func UpdateBookmark(bookmarkHandle BookmarkHandle, eventHandle EventHandle) error {
	bookmarkMutex.Lock()
	defer bookmarkMutex.Unlock()
	return EvtUpdateBookmark(syscall.Handle(bookmarkHandle), syscall.Handle(eventHandle))
}

/* Serialize the bookmark as XML */
func RenderBookmark(bookmarkHandle BookmarkHandle) (string, error) {
	bookmarkMutex.Lock()
	defer bookmarkMutex.Unlock()

	// Check if handle is valid
	if bookmarkHandle == 0 {
		return "", fmt.Errorf("invalid bookmark handle")
	}

	// Use defer/recover to catch any panics that might occur
	var result string
	var resultErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				result = "[Recovered from panic in RenderBookmark]"
				resultErr = fmt.Errorf("panic in RenderBookmark: %v", r)
			}
		}()

		var dwUsed uint32
		var dwProps uint32
		err := EvtRender(0, syscall.Handle(bookmarkHandle), EvtRenderBookmark, 0, nil, &dwUsed, &dwProps)
		if err != nil {
			resultErr = err
			return
		}

		// Ensure we don't allocate an excessively large buffer
		if dwUsed > 1024*1024 { // 1MB limit
			resultErr = fmt.Errorf("bookmark too large to render: %d bytes", dwUsed)
			return
		}

		buf := make([]uint16, dwUsed)
		err = EvtRender(0, syscall.Handle(bookmarkHandle), EvtRenderBookmark, uint32(len(buf)), &buf[0], &dwUsed, &dwProps)
		if err != nil {
			resultErr = err
			return
		}
		result = syscall.UTF16ToString(buf)
	}()

	return result, resultErr
}
