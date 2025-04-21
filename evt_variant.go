// +build windows

package winlog

import (
	"fmt"
	"time"
	"unicode/utf16"
	"unsafe"
)

/* Convenience functions to get values out of
   an array of EvtVariant structures */

const (
	EvtVarTypeNull = iota
	EvtVarTypeString
	EvtVarTypeAnsiString
	EvtVarTypeSByte
	EvtVarTypeByte
	EvtVarTypeInt16
	EvtVarTypeUInt16
	EvtVarTypeInt32
	EvtVarTypeUInt32
	EvtVarTypeInt64
	EvtVarTypeUInt64
	EvtVarTypeSingle
	EvtVarTypeDouble
	EvtVarTypeBoolean
	EvtVarTypeBinary
	EvtVarTypeGuid
	EvtVarTypeSizeT
	EvtVarTypeFileTime
	EvtVarTypeSysTime
	EvtVarTypeSid
	EvtVarTypeHexInt32
	EvtVarTypeHexInt64
	EvtVarTypeEvtHandle
	EvtVarTypeEvtXml
)

type evtVariant struct {
	Data  uint64
	Count uint32
	Type  uint32
}

type fileTime struct {
	lowDateTime  uint32
	highDateTime uint32
}

type EvtVariant []byte

/* Given a byte array from EvtRender, make an EvtVariant.
   EvtVariant wraps an array of variables. */
func NewEvtVariant(buffer []byte) EvtVariant {
	return EvtVariant(buffer)
}

func (e EvtVariant) elemAt(index uint32) *evtVariant {
	// Add defensive checks to prevent crashes
	if len(e) == 0 {
		return nil
	}

	// Check if index is within bounds (assuming each evtVariant is 16 bytes)
	if int(index*16) >= len(e) {
		return nil
	}

	return (*evtVariant)(unsafe.Pointer(uintptr(16*index) + uintptr(unsafe.Pointer(&e[0]))))
}

func UTF16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
	}
	return string(utf16.Decode(s))
}

/* Return the string value of the variable at `index`. If the
   variable isn't a string, an error is returned */
func (e EvtVariant) String(index uint32) (string, error) {
	// Add defensive checks to prevent crashes
	if len(e) == 0 {
		return "", fmt.Errorf("EvtVariant is empty")
	}

	elem := e.elemAt(index)
	if elem == nil {
		return "", fmt.Errorf("EvtVariant index %v is out of bounds", index)
	}

	if elem.Type != EvtVarTypeString {
		return "", fmt.Errorf("EvtVariant at index %v was not of type string, type was %v", index, elem.Type)
	}

	// Check if Data is a valid pointer
	if elem.Data == 0 {
		return "", fmt.Errorf("EvtVariant at index %v has nil data", index)
	}

	// Check if Count is reasonable
	if elem.Count > 1<<20 { // 1 million characters should be enough
		return "", fmt.Errorf("EvtVariant at index %v has unreasonable count: %v", index, elem.Count)
	}

	// Use defer/recover to catch any panics during string conversion
	var result string
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in EvtVariant.String: %v", r)
			}
		}()

		wideString := (*[1 << 29]uint16)(unsafe.Pointer(uintptr(elem.Data)))
		result = UTF16ToString(wideString[0 : elem.Count+1])
	}()

	if err != nil {
		return "", err
	}

	return result, nil
}

/* Return the unsigned integer value at `index`. If the variable
   isn't a Byte, UInt16, UInt32 or UInt64 an error is returned. */
func (e EvtVariant) Uint(index uint32) (uint64, error) {
	// Add defensive checks to prevent crashes
	if len(e) == 0 {
		return 0, fmt.Errorf("EvtVariant is empty")
	}

	elem := e.elemAt(index)
	if elem == nil {
		return 0, fmt.Errorf("EvtVariant index %v is out of bounds", index)
	}

	// Use defer/recover to catch any panics during conversion
	var result uint64
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in EvtVariant.Uint: %v", r)
			}
		}()

		switch elem.Type {
		case EvtVarTypeByte:
			result = uint64(byte(elem.Data))
		case EvtVarTypeUInt16:
			result = uint64(uint16(elem.Data))
		case EvtVarTypeUInt32:
			result = uint64(uint32(elem.Data))
		case EvtVarTypeUInt64:
			result = uint64(elem.Data)
		default:
			err = fmt.Errorf("EvtVariant at index %v was not an unsigned integer, type is %v", index, elem.Type)
		}
	}()

	if err != nil {
		return 0, err
	}

	return result, nil
}

/* Return the integer value at `index`. If the variable
   isn't a SByte, Int16, Int32 or Int64 an error is returned. */
func (e EvtVariant) Int(index uint32) (int64, error) {
	// Add defensive checks to prevent crashes
	if len(e) == 0 {
		return 0, fmt.Errorf("EvtVariant is empty")
	}

	elem := e.elemAt(index)
	if elem == nil {
		return 0, fmt.Errorf("EvtVariant index %v is out of bounds", index)
	}

	// Use defer/recover to catch any panics during conversion
	var result int64
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in EvtVariant.Int: %v", r)
			}
		}()

		switch elem.Type {
		case EvtVarTypeSByte:
			result = int64(byte(elem.Data))
		case EvtVarTypeInt16:
			result = int64(int16(elem.Data))
		case EvtVarTypeInt32:
			result = int64(int32(elem.Data))
		case EvtVarTypeInt64:
			result = int64(elem.Data)
		default:
			err = fmt.Errorf("EvtVariant at index %v was not an integer, type is %v", index, elem.Type)
		}
	}()

	if err != nil {
		return 0, err
	}

	return result, nil
}

/* Return the FileTime at `index`, converted to Time.time. If the
   variable isn't a FileTime an error is returned */
func (e EvtVariant) FileTime(index uint32) (time.Time, error) {
	// Add defensive checks to prevent crashes
	if len(e) == 0 {
		return time.Time{}, fmt.Errorf("EvtVariant is empty")
	}

	elem := e.elemAt(index)
	if elem == nil {
		return time.Time{}, fmt.Errorf("EvtVariant index %v is out of bounds", index)
	}

	if elem.Type != EvtVarTypeFileTime {
		return time.Time{}, fmt.Errorf("EvtVariant at index %v was not of type FileTime, type was %v", index, elem.Type)
	}

	// Use defer/recover to catch any panics during conversion
	var result time.Time
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in EvtVariant.FileTime: %v", r)
			}
		}()

		var t = (*fileTime)(unsafe.Pointer(&elem.Data))
		timeSecs := (((int64(t.highDateTime) << 32) | int64(t.lowDateTime)) / 10000000) - int64(11644473600)
		timeNano := (((int64(t.highDateTime) << 32) | int64(t.lowDateTime)) % 10000000) * 100
		result = time.Unix(timeSecs, timeNano)
	}()

	if err != nil {
		return time.Time{}, err
	}

	return result, nil
}

/* Return whether the variable was actually set, or whether it
   has null type */
func (e EvtVariant) IsNull(index uint32) bool {
	// Add defensive checks to prevent crashes
	if len(e) == 0 {
		return true // Consider empty variants as null
	}

	elem := e.elemAt(index)
	if elem == nil {
		return true // Consider out-of-bounds elements as null
	}

	// Use defer/recover to catch any panics
	var result bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				// If we panic, consider it null
				result = true
			}
		}()

		result = elem.Type == EvtVarTypeNull
	}()

	return result
}
