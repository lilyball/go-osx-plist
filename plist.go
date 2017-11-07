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

// TODO: CFPropertyListWrite() for stream-based writing
// TODO: CFPropertyListCreateWithStream() for stream-based reading

func cfPropertyListCreateWithData(data []byte) (cfObj cfTypeRef, format Format, err error) {
	cfData := convertBytesToCFData(data)
	defer C.CFRelease(C.CFTypeRef(cfData))
	var cfFormat C.CFPropertyListFormat
	var cfError C.CFErrorRef
	cfPlist := C.CFPropertyListCreateWithData(nil, cfData, 0, &cfFormat, &cfError)
	if cfPlist == nil {
		// an error occurred
		if cfError != nil {
			defer cfRelease(cfTypeRef(cfError))
			return nil, Format{cfFormat}, NewCFError(cfError)
		}
		return nil, Format{}, errors.New("plist: unknown error in CFPropertyListCreateWithData")
	}
	return cfTypeRef(cfPlist), Format{cfFormat}, nil
}

func cfPropertyListCreateData(plist cfTypeRef, format Format) ([]byte, error) {
	var cfError C.CFErrorRef
	cfData := C.CFPropertyListCreateData(nil, C.CFPropertyListRef(plist), format.cfFormat, 0, &cfError)
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

type CFError struct {
	Domain      string
	Code        int
	UserInfo    map[string]interface{}
	Description string // comes from CFErrorCopyDescription()
}

func NewCFError(c C.CFErrorRef) *CFError {
	e := &CFError{
		Domain: convertCFStringToString(C.CFStringRef(C.CFErrorGetDomain(c))),
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
