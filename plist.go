// Package plist implements serializing and deserializing of property list
// objects using CoreFoundation.
//
// Property list objects are any object of type:
//   - string
//   - []byte
//   - time.Time
//   - bool
//   - numeric type (except for 64-bit uint types)
//   - a slice of any property list object
//   - a map from a string to any property list object
//
// Note, a []byte (or []uint8) slice is always converted to a CFDataRef,
// but a slice of any other type is converted to a CFArrayRef
package plist

// #cgo LDFLAGS: -framework CoreFoundation
// #include <CoreFoundation/CoreFoundation.h>
import "C"
import "errors"
import "reflect"
import "strconv"

// CFPropertyListFormat
const (
	// OpenStep format (use of this format is discouraged)
	CFPropertyListOpenStepFormat = 1
	// XML format version 1.0
	CFPropertyListXMLFormat_v1_0 = 100
	// Binary format version 1.0
	CFPropertyListBinaryFormat_v1_0 = 200
)

// TODO: CFPropertyListWrite() for stream-based writing
// TODO: CFPropertyListCreateWithStream() for stream-based reading

// CFPropertyListCreateWithData decodes the given data into a property list object.
func CFPropertyListCreateWithData(data []byte) (plist interface{}, format int, err error) {
	cfData := convertBytesToCFData(data)
	defer C.CFRelease(C.CFTypeRef(cfData))
	var cfFormat C.CFPropertyListFormat
	var cfError C.CFErrorRef
	cfObj := C.CFPropertyListCreateWithData(nil, cfData, 0, &cfFormat, &cfError)
	if cfObj == nil {
		// an error occurred
		if cfError != nil {
			defer C.CFRelease(C.CFTypeRef(cfError))
			return nil, 0, NewCFError(cfError)
		}
		return nil, 0, errors.New("plist: unknown error in CFPropertyListCreateWithData")
	}
	defer C.CFRelease(C.CFTypeRef(cfObj))
	val, err := convertCFTypeToValue(cfTypeRef(cfObj))
	if err != nil {
		return nil, 0, err
	}
	return val, int(cfFormat), nil
}

// CFPropertyListCreateData returns a []byte containing a serialized representation
// of a given property list in a specified format.
func CFPropertyListCreateData(plist interface{}, format int) ([]byte, error) {
	cfObj, err := convertValueToCFType(plist)
	if err != nil {
		return nil, err
	}
	defer cfRelease(cfObj)
	return cfPropertyListCreateData(cfObj, format)
}

func cfPropertyListCreateData(plist cfTypeRef, format int) ([]byte, error) {
	var cfError C.CFErrorRef
	cfData := C.CFPropertyListCreateData(nil, C.CFPropertyListRef(plist), C.CFPropertyListFormat(format), 0, &cfError)
	if cfData == nil {
		// an error occurred
		if cfError != nil {
			defer cfRelease(cfTypeRef(cfError))
			return nil, NewCFError(cfError)
		}
		return nil, errors.New("plist: unknown error in CFPropertyListCreateData")
	}
	defer cfRelease(cfTypeRef(cfData))
	return convertCFDataToBytes(cfData), nil
}

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
	CFTypeID int
}

func (e *UnknownCFTypeError) Error() string {
	return "plist: unknown CFTypeID " + strconv.Itoa(e.CFTypeID)
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

type CFError struct {
	Domain      string
	Code        int
	UserInfo    map[string]interface{}
	Description string // comes from CFErrorCopyDescription()
}

func NewCFError(c C.CFErrorRef) *CFError {
	e := &CFError{
		Domain: convertCFStringToString(C.CFErrorGetDomain(c)),
		Code:   int(C.CFErrorGetCode(c)),
	}
	cfDict := C.CFErrorCopyUserInfo(c)
	defer C.CFRelease(C.CFTypeRef(cfDict))
	if userInfo, err := convertCFDictionaryToMap(cfDict); err == nil {
		// on error, skip user info
		e.UserInfo = userInfo
	}
	cfStr := C.CFErrorCopyDescription(c)
	defer C.CFRelease(C.CFTypeRef(cfStr))
	e.Description = convertCFStringToString(cfStr)
	return e
}

func (e *CFError) Error() string {
	return "plist: " + e.Description
}
