package plist

// #include <CoreFoundation/CoreFoundation.h>
import "C"
import "reflect"
import "strconv"

// An UnsupportedTypeError is returned by Marshal when attempting to encode an
// unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "plist: unsupported type: " + e.Type.String()
}

type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "json: unsupported value: " + e.Str
}

type UnknownCFTypeError struct {
	CFTypeID C.CFTypeID
}

func (e *UnknownCFTypeError) Error() string {
	cfStr := C.CFCopyTypeIDDescription(e.CFTypeID)
	str := convertCFStringToString(cfStr)
	cfRelease(cfTypeRef(cfStr))
	return "plist: unknown CFTypeID " + strconv.Itoa(int(e.CFTypeID)) + " (" + str + ")"
}

// UnsupportedKeyTypeError represents the case where a CFDictionary is being converted
// back into a map[string]interface{} but its key type is not a CFString.
//
// This should never occur in practice, because the only CFDictionaries that
// should be handled are coming from property lists, which require the keys to
// be strings.
type UnsupportedKeyTypeError struct {
	CFTypeID int
}

func (e *UnsupportedKeyTypeError) Error() string {
	return "plist: unexpected dictionary key CFTypeID " + strconv.Itoa(e.CFTypeID)
}
