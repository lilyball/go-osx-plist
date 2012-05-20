// Package plist implements serializing and deserializing of property list
// objects using CoreFoundation.
//
// Property list objects are any object of type:
// - string
// - []byte
// - time.Time
// - bool
// - numeric type
// - a slice of any property list object
// - a map from a string to any property list object
//
// Note, a []byte (or []uint8) slice is always converted to a CFDataRef,
// but a slice of any other type is converted to a CFArrayRef
package plist

// #cgo LDFLAGS: -framework CoreFoundation
// #include <CoreFoundation/CoreFoundation.h>
import "C"
import "reflect"
import "strconv"

type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "plist: unsupported type: " + e.Type.String()
}

type UnknownCFTypeError struct {
	CFTypeID int
}

func (e *UnknownCFTypeError) Error() string {
	return "plist: unknown CFTypeID " + strconv.Itoa(e.CFTypeID)
}

type UnexpectedKeyTypeError struct {
	CFTypeID int
}

func (e *UnexpectedKeyTypeError) Error() string {
	return "plist: unexpected dictionary key CFTypeID " + strconv.Itoa(e.CFTypeID)
}
