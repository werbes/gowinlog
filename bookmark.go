// +build windows

package winlog

import (
	"fmt"
	"syscall"
)

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
	// Create a copy of the event handle to prevent access issues if the event is modified by Windows
	eventCopy, err := EvtCreateEventCopy(syscall.Handle(eventHandle))
	if err != nil {
		return fmt.Errorf("Failed to create event copy for bookmark update: %v", err)
	}
	defer CloseEventHandle(uint64(eventCopy))

	// Use the copied event handle for the update
	return EvtUpdateBookmark(syscall.Handle(bookmarkHandle), eventCopy)
}

/* Serialize the bookmark as XML */
func RenderBookmark(bookmarkHandle BookmarkHandle) (string, error) {
	var dwUsed uint32
	var dwProps uint32
	EvtRender(0, syscall.Handle(bookmarkHandle), EvtRenderBookmark, 0, nil, &dwUsed, &dwProps)
	buf := make([]uint16, dwUsed)
	err := EvtRender(0, syscall.Handle(bookmarkHandle), EvtRenderBookmark, uint32(len(buf)), &buf[0], &dwUsed, &dwProps)
	if err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buf), nil
}
