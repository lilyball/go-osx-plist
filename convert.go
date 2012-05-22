package plist

// #import <CoreFoundation/CoreFoundation.h>
// #import <CoreGraphics/CGBase.h> // for CGFloat
import "C"

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"time"
	"unsafe"
)

type cfTypeRef C.CFTypeRef

func cfRelease(cfObj cfTypeRef) {
	C.CFRelease(C.CFTypeRef(cfObj))
}

func convertValueToCFType(v reflect.Value) (cfTypeRef, error) {
	if !v.IsValid() {
		return nil, &UnsupportedValueError{v, "invalid value"}
	}
	switch v.Kind() {
	case reflect.Bool:
		return cfTypeRef(convertBoolToCFBoolean(v.Bool())), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cfTypeRef(convertInt64ToCFNumber(v.Int())), nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return cfTypeRef(convertUInt32ToCFNumber(uint32(v.Uint()))), nil
	case reflect.Uint, reflect.Uintptr:
		// don't try and convert if uint/uintptr is 64-bits
		if v.Type().Bits() < 64 {
			return cfTypeRef(convertUInt32ToCFNumber(uint32(v.Uint()))), nil
		}
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, &UnsupportedValueError{v, strconv.FormatFloat(f, 'g', -1, v.Type().Bits())}
		}
		return cfTypeRef(convertFloat64ToCFNumber(v.Float())), nil
	case reflect.String:
		cfStr := convertStringToCFString(v.String())
		if cfStr == nil {
			return nil, errors.New("plist: could not convert string to CFStringRef")
		}
		return cfTypeRef(cfStr), nil
	case reflect.Struct:
		// only struct type we support is time.Time
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return cfTypeRef(convertTimeToCFDate(v.Interface().(time.Time))), nil
		}
	case reflect.Array, reflect.Slice:
		// check for []byte first (byte is uint8)
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return cfTypeRef(convertBytesToCFData(v.Interface().([]byte))), nil
		}
		ary, err := convertSliceToCFArray(v)
		return cfTypeRef(ary), err
	case reflect.Map:
		dict, err := convertMapToCFDictionary(v)
		return cfTypeRef(dict), err
	case reflect.Interface:
		if v.IsNil() {
			return nil, &UnsupportedValueError{v, "nil interface"}
		}
		return convertValueToCFType(v.Elem())
	}
	return nil, &UnsupportedTypeError{v.Type()}
}

// we shouldn't ever get an error from this, but I'd rather not panic
func convertCFTypeToInterface(cfType cfTypeRef) (interface{}, error) {
	typeId := C.CFGetTypeID(C.CFTypeRef(cfType))
	switch typeId {
	case C.CFStringGetTypeID():
		return convertCFStringToString(C.CFStringRef(cfType)), nil
	case C.CFNumberGetTypeID():
		return convertCFNumberToInterface(C.CFNumberRef(cfType)), nil
	case C.CFBooleanGetTypeID():
		return convertCFBooleanToBool(C.CFBooleanRef(cfType)), nil
	case C.CFDataGetTypeID():
		return convertCFDataToBytes(C.CFDataRef(cfType)), nil
	case C.CFDateGetTypeID():
		return convertCFDateToTime(C.CFDateRef(cfType)), nil
	case C.CFArrayGetTypeID():
		ary, err := convertCFArrayToSlice(C.CFArrayRef(cfType))
		return ary, err
	case C.CFDictionaryGetTypeID():
		dict, err := convertCFDictionaryToMap(C.CFDictionaryRef(cfType))
		return dict, err
	}
	return nil, &UnknownCFTypeError{int(typeId)}
}

// ===== CFData =====
func convertBytesToCFData(data []byte) C.CFDataRef {
	var ptr *C.UInt8
	if len(data) > 0 {
		ptr = (*C.UInt8)((&data[0]))
	}
	return C.CFDataCreate(nil, ptr, C.CFIndex(len(data)))
}

func convertCFDataToBytes(cfData C.CFDataRef) []byte {
	bytes := C.CFDataGetBytePtr(cfData)
	return C.GoBytes(unsafe.Pointer(bytes), C.int(C.CFDataGetLength(cfData)))
}

// ===== CFString =====
// convertStringToCFString may return nil if the input string is not a valid UTF-8 string
func convertStringToCFString(str string) C.CFStringRef {
	// go through unsafe to get the string bytes directly without the copy
	header := (*reflect.StringHeader)(unsafe.Pointer(&str))
	bytes := (*C.UInt8)(unsafe.Pointer(header.Data))
	return C.CFStringCreateWithBytes(nil, bytes, C.CFIndex(header.Len), C.kCFStringEncodingUTF8, C.false)
}

func convertCFStringToString(cfStr C.CFStringRef) string {
	cstrPtr := C.CFStringGetCStringPtr(cfStr, C.kCFStringEncodingUTF8)
	if cstrPtr != nil {
		return C.GoString(cstrPtr)
	}
	// quick path doesn't work, so copy the bytes out to a buffer
	length := C.CFStringGetLength(cfStr)
	if length == 0 {
		// short-cut for empty strings
		return ""
	}
	cfRange := C.CFRange{0, length}
	enc := C.CFStringEncoding(C.kCFStringEncodingUTF8)
	// first find the buffer size necessary
	var usedBufLen C.CFIndex
	if C.CFStringGetBytes(cfStr, cfRange, enc, 0, C.false, nil, 0, &usedBufLen) > 0 {
		bytes := make([]byte, usedBufLen)
		buffer := (*C.UInt8)(unsafe.Pointer(&bytes[0]))
		if C.CFStringGetBytes(cfStr, cfRange, enc, 0, C.false, buffer, usedBufLen, nil) > 0 {
			// bytes is now filled up
			// convert it to a string
			header := (*reflect.SliceHeader)(unsafe.Pointer(&bytes))
			strHeader := &reflect.StringHeader{
				Data: header.Data,
				Len:  header.Len,
			}
			return *(*string)(unsafe.Pointer(strHeader))
		}
	}

	// we failed to convert, for some reason. Too bad there's no nil string
	return ""
}

// ===== CFDate =====
func convertTimeToCFDate(t time.Time) C.CFDateRef {
	// truncate to milliseconds, to get a more predictable conversion
	ms := int64(time.Duration(t.UnixNano()) / time.Millisecond * time.Millisecond)
	nano := C.double(ms) / C.double(time.Second)
	nano -= C.kCFAbsoluteTimeIntervalSince1970
	return C.CFDateCreate(nil, C.CFAbsoluteTime(nano))
}

func convertCFDateToTime(cfDate C.CFDateRef) time.Time {
	nano := C.double(C.CFDateGetAbsoluteTime(cfDate))
	nano += C.kCFAbsoluteTimeIntervalSince1970
	// pull out milliseconds, to get a more predictable conversion
	ms := int64(float64(C.round(nano * 1000)))
	sec := ms / 1000
	nsec := (ms % 1000) * int64(time.Millisecond)
	return time.Unix(sec, nsec)
}

// ===== CFBoolean =====
func convertBoolToCFBoolean(b bool) C.CFBooleanRef {
	// I don't think the CFBoolean constants have retain counts,
	// but just in case lets call CFRetain on them
	if b {
		return C.CFBooleanRef(C.CFRetain(C.CFTypeRef(C.kCFBooleanTrue)))
	}
	return C.CFBooleanRef(C.CFRetain(C.CFTypeRef(C.kCFBooleanFalse)))
}

func convertCFBooleanToBool(cfBoolean C.CFBooleanRef) bool {
	return C.CFBooleanGetValue(cfBoolean) != 0
}

// ===== CFNumber =====
// for simplicity's sake, only include the largest of any given numeric datatype
func convertInt64ToCFNumber(i int64) C.CFNumberRef {
	sint := C.SInt64(i)
	return C.CFNumberCreate(nil, C.kCFNumberSInt64Type, unsafe.Pointer(&sint))
}

func convertCFNumberToInt64(cfNumber C.CFNumberRef) int64 {
	var sint C.SInt64
	C.CFNumberGetValue(cfNumber, C.kCFNumberSInt64Type, unsafe.Pointer(&sint))
	return int64(sint)
}

// there is no uint64 CFNumber type, so we have to use the SInt64 one
func convertUInt32ToCFNumber(u uint32) C.CFNumberRef {
	sint := C.SInt64(u)
	return C.CFNumberCreate(nil, C.kCFNumberSInt64Type, unsafe.Pointer(&sint))
}

func convertCFNumberToUInt32(cfNumber C.CFNumberRef) uint32 {
	var sint C.SInt64
	C.CFNumberGetValue(cfNumber, C.kCFNumberSInt64Type, unsafe.Pointer(&sint))
	return uint32(sint)
}

func convertFloat64ToCFNumber(f float64) C.CFNumberRef {
	double := C.double(f)
	return C.CFNumberCreate(nil, C.kCFNumberDoubleType, unsafe.Pointer(&double))
}

func convertCFNumberToFloat64(cfNumber C.CFNumberRef) float64 {
	var double C.double
	C.CFNumberGetValue(cfNumber, C.kCFNumberDoubleType, unsafe.Pointer(&double))
	return float64(double)
}

// Converts the CFNumberRef to the most appropriate numeric type
func convertCFNumberToInterface(cfNumber C.CFNumberRef) interface{} {
	typ := C.CFNumberGetType(cfNumber)
	switch typ {
	case C.kCFNumberSInt8Type:
		var sint C.SInt8
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&sint))
		return int8(sint)
	case C.kCFNumberSInt16Type:
		var sint C.SInt16
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&sint))
		return int16(sint)
	case C.kCFNumberSInt32Type:
		var sint C.SInt32
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&sint))
		return int32(sint)
	case C.kCFNumberSInt64Type:
		var sint C.SInt64
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&sint))
		return int64(sint)
	case C.kCFNumberFloat32Type:
		var float C.Float32
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&float))
		return float32(float)
	case C.kCFNumberFloat64Type:
		var float C.Float64
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&float))
		return float64(float)
	case C.kCFNumberCharType:
		var char C.char
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&char))
		return byte(char)
	case C.kCFNumberShortType:
		var short C.short
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&short))
		return int16(short)
	case C.kCFNumberIntType:
		var i C.int
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&i))
		return int32(i)
	case C.kCFNumberLongType:
		var long C.long
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&long))
		return int(long)
	case C.kCFNumberLongLongType:
		// this is the only type that may actually overflow us
		var longlong C.longlong
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&longlong))
		return int64(longlong)
	case C.kCFNumberFloatType:
		var float C.float
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&float))
		return float32(float)
	case C.kCFNumberDoubleType:
		var double C.double
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&double))
		return float64(double)
	case C.kCFNumberCFIndexType:
		// CFIndex is a long
		var index C.CFIndex
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&index))
		return int(index)
	case C.kCFNumberNSIntegerType:
		// We don't have a definition of NSInteger, but we know it's either an int or a long
		var nsInt C.long
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&nsInt))
		return int(nsInt)
	case C.kCFNumberCGFloatType:
		// CGFloat is a float or double
		var float C.CGFloat
		C.CFNumberGetValue(cfNumber, typ, unsafe.Pointer(&float))
		if unsafe.Sizeof(float) == 8 {
			return float64(float)
		} else {
			return float32(float)
		}
	}
	panic("plist: unknown CFNumber type")
}

// ===== CFArray =====
// use reflect.Value to support slices of any type
func convertSliceToCFArray(slice reflect.Value) (C.CFArrayRef, error) {
	return convertSliceToCFArrayHelper(slice, convertValueToCFType)
}

func convertSliceToCFArrayHelper(slice reflect.Value, helper func(reflect.Value) (cfTypeRef, error)) (C.CFArrayRef, error) {
	if slice.Len() == 0 {
		// short-circuit 0, so we can assume plists[0] is valid later
		return C.CFArrayCreate(nil, nil, 0, nil), nil
	}
	// assume slice is a slice/array, because our caller already checked
	plists := make([]cfTypeRef, slice.Len())
	// defer the release
	defer func() {
		for _, cfObj := range plists {
			if cfObj != nil {
				cfRelease(cfObj)
			}
		}
	}()
	// convert the slice
	for i := 0; i < slice.Len(); i++ {
		cfType, err := helper(slice.Index(i))
		if err != nil {
			return nil, err
		}
		plists[i] = cfType
	}

	// create the array
	callbacks := (*C.CFArrayCallBacks)(&C.kCFTypeArrayCallBacks)
	return C.CFArrayCreate(nil, (*unsafe.Pointer)(&plists[0]), C.CFIndex(len(plists)), callbacks), nil
}

func convertCFArrayToSlice(cfArray C.CFArrayRef) ([]interface{}, error) {
	count := C.CFArrayGetCount(cfArray)
	if count == 0 {
		// short-circuit zero so we can assume cfTypes[0] is valid later
		return []interface{}{}, nil
	}
	cfTypes := make([]cfTypeRef, int(count))
	cfRange := C.CFRange{0, count}
	C.CFArrayGetValues(cfArray, cfRange, (*unsafe.Pointer)(&cfTypes[0]))
	result := make([]interface{}, int(count))
	for i, cfObj := range cfTypes {
		val, err := convertCFTypeToInterface(cfObj)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// ===== CFDictionary =====
// use reflect.Value to support maps of any type
func convertMapToCFDictionary(m reflect.Value) (C.CFDictionaryRef, error) {
	return convertMapToCFDictionaryHelper(m, convertValueToCFType)
}

func convertMapToCFDictionaryHelper(m reflect.Value, helper func(reflect.Value) (cfTypeRef, error)) (C.CFDictionaryRef, error) {
	// assume m is a map, because our caller already checked
	if m.Type().Key().Kind() != reflect.String {
		// the map keys aren't strings
		return nil, &UnsupportedTypeError{m.Type()}
	}
	mapKeys := m.MapKeys()
	keys := make([]cfTypeRef, len(mapKeys))
	values := make([]cfTypeRef, len(mapKeys))
	// defer the release
	defer func() {
		for _, cfKey := range keys {
			if cfKey != nil {
				cfRelease(cfTypeRef(cfKey))
			}
		}
		for _, cfVal := range values {
			if cfVal != nil {
				cfRelease(cfTypeRef(cfVal))
			}
		}
	}()
	// create the keys and values slices
	for i, keyVal := range mapKeys {
		// keyVal is a Value representing a string
		cfStr := convertStringToCFString(keyVal.String())
		if cfStr == nil {
			return nil, errors.New("plist: could not convert string to CFStringRef")
		}
		keys[i] = cfTypeRef(cfStr)
		cfObj, err := helper(m.MapIndex(keyVal))
		if err != nil {
			return nil, err
		}
		values[i] = cfObj
	}
	return createCFDictionary(keys, values), nil
}

// wrapper for C.CFDictionaryCreate, since referencing the callbacks in 2 separate files
// seems to be triggering some sort of "redefinition" error in cgo
func createCFDictionary(keys, values []cfTypeRef) C.CFDictionaryRef {
	if len(keys) != len(values) {
		panic("plist: unexpected length difference between keys and values")
	}
	var keyPtr, valPtr *unsafe.Pointer
	if len(keys) > 0 {
		keyPtr = (*unsafe.Pointer)(&keys[0])
		valPtr = (*unsafe.Pointer)(&values[0])
	}
	keyCallbacks := (*C.CFDictionaryKeyCallBacks)(&C.kCFTypeDictionaryKeyCallBacks)
	valCallbacks := (*C.CFDictionaryValueCallBacks)(&C.kCFTypeDictionaryValueCallBacks)
	return C.CFDictionaryCreate(nil, keyPtr, valPtr, C.CFIndex(len(keys)), keyCallbacks, valCallbacks)
}

func convertCFDictionaryToMap(cfDict C.CFDictionaryRef) (map[string]interface{}, error) {
	count := int(C.CFDictionaryGetCount(cfDict))
	cfKeys := make([]cfTypeRef, count)
	cfVals := make([]cfTypeRef, count)
	C.CFDictionaryGetKeysAndValues(cfDict, (*unsafe.Pointer)(&cfKeys[0]), (*unsafe.Pointer)(&cfVals[0]))
	m := make(map[string]interface{}, count)
	for i := 0; i < count; i++ {
		cfKey := cfKeys[i]
		typeId := C.CFGetTypeID(C.CFTypeRef(cfKey))
		if typeId != C.CFStringGetTypeID() {
			return nil, &UnsupportedKeyTypeError{int(typeId)}
		}
		key := convertCFStringToString(C.CFStringRef(cfKey))
		val, err := convertCFTypeToInterface(cfVals[i])
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
	return m, nil
}
