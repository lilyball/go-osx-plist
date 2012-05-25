package plist

// #include <CoreFoundation/CoreFoundation.h>
import "C"

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Format represents the format of the property list
type Format struct {
	cfFormat C.CFPropertyListFormat // don't export this, we want control over all valid values
}

var (
	// OpenStep format (use of this format is discouraged)
	OpenStepFormat = Format{1}
	// XML format version 1.0
	XMLFormat = Format{100}
	// Binary format version 1.0
	BinaryFormat = Format{200}
)

func (f Format) String() string {
	switch f.cfFormat {
	case 1:
		return "OpenStep format"
	case 100:
		return "XML format version 1.0"
	case 200:
		return "Binary format version 1.0"
	}
	return "Unknown format"
}

// Marshal returns the property list encoding of v.
//
// The Marshall interface is very heavily based off of encoding/json.Marshal.
//
// Marshal traverses the value v recursively. If an encountered value implements
// the Marshaler interface and is not a nil pointer, Marshal calls its
// MarshalPlist method to produce a property list object (as defined by
// CFPropertyListCreateData()). If the method returns any other object, that is
// considered an error.
//
// Otherwise, Marshal uses the following type-dependent default encodings:
//
// Boolean values encode as CFBooleans.
//
// Floating point and integer values encode as CFNumbers, except for 64-bit
// unsigned integers which cause Marshal to return an UnsupportedValueError.
//
// String values encode as CFStrings, with each invalid UTF-8 sequence replaced
// by the encoding of the Unicode replacement character U+FFFD.
//
// Time values encode as CFDate, with millisecond precision. Far-future or
// far-past dates may have less than millisecond precision.
//
// Array and slice values encode as CFArrays, except that []byte encodes as a
// CFData.
//
// Struct values encode as CFDictionaries. Each exported struct field becomes a
// member of the object unless
//
//     - the field's tag is "-"
//     - the field is empty and its tag specifies the "omitempty" option.
//
// The empty values are false, 0, any nil pointer or interface value, and any
// array, slice, map, or string of length zero. The object's default key string
// is the struct field name but can be specified in the struct field's tag
// value. The "plist" key in the struct field's tag value is the key name,
// followed by an optional comma and options. Examples:
//
//     // Field is ignored by this package.
//     Field int `plist:"-"`
//     // Field appears in plist as key "myName".
//     Field int `plist:"myName"`
//     // Field appears in plist as key "myName" and
//     // the field is omitted from the object if its value is empty,
//     // as defined above.
//     Field int `plist:"myName,omitempty"`
//     // Field appears in plist as key "Field" (the default), but
//     // the field is skipped if empty.
//     // Note the leading comma.
//     Field int `plist:",omitempty"`
//
// The key name will be used if it's a non-empty string consisting of only
// Unicode letters, digits, dollar signs, percent signs, hyphens, underscores
// and slashes.
//
// Map values encode as CFDictionaries. The map's key type must be string.
//
// Pointer values encode as the value pointed to. A nil pointer causes Marshal
// to return an UnsupportedValueError.
//
// Interface values encode as the value contained in the interface. A nil
// interface value causes Marshal to return an UnsupportedValueError.
//
// Channel, complex, and function values cannot be encoded in a plist.
// Attempting to encode such a value causes Marshal to return an
// UnsupportedTypeError.
//
// Property lists cannot represent cyclic data structures and Marshal does not
// handle them. Passing cyclic structures to Marshal will result in an infinite
// recursion.
func Marshal(v interface{}, format Format) ([]byte, error) {
	cfObj, err := marshalValue(reflect.ValueOf(v))
	if err != nil {
		return nil, err
	}
	defer cfRelease(cfObj)
	return cfPropertyListCreateData(cfObj, format)
}

var timeType = reflect.TypeOf(time.Time{})
var byteSliceType = reflect.TypeOf([]byte(nil))
var stringType = reflect.TypeOf("")

func marshalValue(v reflect.Value) (cfTypeRef, error) {
	if !v.IsValid() {
		return nil, &UnsupportedValueError{v, "invalid value"}
	}
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil, &UnsupportedValueError{v, "nil pointer"}
	}
	if v.Kind() == reflect.Interface && v.IsNil() {
		return nil, &UnsupportedValueError{v, "nil interface"}
	}

	m, ok := v.Interface().(Marshaler)
	if !ok {
		if v.Kind() != reflect.Ptr && v.CanAddr() {
			m, ok = v.Addr().Interface().(Marshaler)
			if ok {
				v = v.Addr()
			}
		}
	}
	if ok {
		obj, err := m.MarshalPlist()
		if err != nil {
			return nil, err
		}
		return convertValueToCFType(reflect.ValueOf(obj))
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Type() == byteSliceType {
			// this is a []byte
			return cfTypeRef(convertBytesToCFData(v.Interface().([]byte))), nil
		}
		cfAry, err := convertSliceToCFArrayHelper(v, marshalValue)
		return cfTypeRef(cfAry), err
	case reflect.Map:
		cfDict, err := convertMapToCFDictionaryHelper(v, marshalValue)
		return cfTypeRef(cfDict), err
	case reflect.Struct:
		if v.Type() == timeType {
			// this is a time.Time
			return cfTypeRef(convertTimeToCFDate(v.Interface().(time.Time))), nil
		}
		cfDict, err := marshalStruct(v)
		return cfTypeRef(cfDict), err
	case reflect.Ptr, reflect.Interface:
		return marshalValue(v.Elem())
	}
	// everything else can be covered by the dumb conversion routine
	return convertValueToCFType(v)
}

func marshalStruct(v reflect.Value) (C.CFDictionaryRef, error) {
	// assume v is a struct
	// we could translate the struct to a map[string]interface{}, but that would
	// be wasteful. Just replicate the relevant logic here
	fields := encodeFields(v.Type())
	keys := make([]cfTypeRef, 0, len(fields))
	values := make([]cfTypeRef, 0, len(fields))
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
	for _, ef := range fields {
		fieldValue := v.Field(ef.i)
		if ef.omitEmpty && isEmptyValue(fieldValue) {
			continue
		}
		cfStr := convertStringToCFString(ef.name)
		if cfStr == nil {
			return nil, errors.New("plist: could not convert string to CFStringRef")
		}
		keys = append(keys, cfTypeRef(cfStr))
		cfObj, err := marshalValue(fieldValue)
		if err != nil {
			return nil, err
		}
		values = append(values, cfObj)
	}
	return createCFDictionary(keys, values), nil
}

// isEmptyValue determines if the value should be skipped for omitempty fields.
// This is lifted from encoding/json so as to match behavior.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// Take a cue from encoding/json and pre-parse the rules for encoding struct
// fields.

// encodeField contains information about how to encode a field of a struct.
type encodeField struct {
	i         int // field index in struct
	name      string
	omitEmpty bool
}

var (
	typeCacheLock     sync.RWMutex
	encodeFieldsCache = make(map[reflect.Type][]encodeField)
)

// encodeFields returns a slice of encodeField for a given struct type.
func encodeFields(t reflect.Type) []encodeField {
	typeCacheLock.RLock()
	fs, ok := encodeFieldsCache[t]
	typeCacheLock.RUnlock()
	if ok {
		return fs
	}

	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	fs, ok = encodeFieldsCache[t]
	if ok {
		return fs
	}

	v := reflect.Zero(t)
	n := v.NumField()
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			// this is a non-exported field
			continue
		}
		if f.Anonymous {
			// encoding/json currently skips anonymous struct fields,
			// so we will too.
			continue
		}
		var ef encodeField
		ef.i = i
		ef.name = f.Name

		tv := f.Tag.Get("plist")
		if tv != "" {
			if tv == "-" {
				continue
			}
			name, opts := parseTag(tv)
			if isValidName(name) {
				ef.name = name
			}
			ef.omitEmpty = opts.Contains("omitempty")
		}
		fs = append(fs, ef)
	}
	encodeFieldsCache[t] = fs
	return fs
}

// isValidName determines if the name matches the naming rules for valid names.
// This is lifted from encoding/json
func isValidName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
			// default:
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				return false
			}
		}
	}
	return true
}

// Unmarshal parses the plist-encoded data and stores the result in the value
// pointed to by v.
//
// Unmarshal uses the inverse of the encodings that Marshal uses, allocating
// maps, slices, and pointers as necessary, with the following additional rules:
//
// To unmarshal a plist into a pointer, Unmarshal unmarshals the plist into the
// value pointed at by the pointer. If the pointer is nil, Unmarshal allocates a
// new value for it to point to.
//
// To unmarshal a plist into an interface value, Unmarshal unmarshals the plist
// into the concrete value contained in the interface value. If the interface
// value is nil, that is, has no concrete value stored in it, Unmarshal stores
// one of these in the interface value:
//
//     bool, for CFBooleans
//     int8, int16, int32, int64, float32, or float64 for CFNumbers
//     string, for CFStrings
//     []byte, for CFDatas
//     time.Time, for CFDates
//     []interface{}, for CFArrays
//     map[string]interface{}, for CFDictionaries
//
// If a plist value is not appropriate for a given target type, or if a plist
// number overflows the target type, Unmarshal skips that field and completes
// the unmarshalling as best it can. If no more serious errors are encountered,
// Unmarshal returns an UnmarshalTypeError describing the earliest such error.
func Unmarshal(data []byte, v interface{}) (format Format, err error) {
	cfObj, format, err := cfPropertyListCreateWithData(data)
	if err != nil {
		return format, err
	}
	defer cfRelease(cfObj)
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return format, &InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	state := &unmarshalState{}
	if err := state.unmarshalValue(cfObj, rv); err != nil {
		return format, err
	}
	return format, state.err
}

type unmarshalState struct {
	err error
}

var (
	cfArrayTypeID      = C.CFArrayGetTypeID()
	cfBooleanTypeID    = C.CFBooleanGetTypeID()
	cfDataTypeID       = C.CFDataGetTypeID()
	cfDateTypeID       = C.CFDateGetTypeID()
	cfDictionaryTypeID = C.CFDictionaryGetTypeID()
	cfNumberTypeID     = C.CFNumberGetTypeID()
	cfStringTypeID     = C.CFStringGetTypeID()
)

var cfTypeMap = map[C.CFTypeID]reflect.Type{
	cfArrayTypeID:      reflect.TypeOf([]interface{}(nil)),
	cfBooleanTypeID:    reflect.TypeOf(false),
	cfDataTypeID:       reflect.TypeOf([]byte(nil)),
	cfDateTypeID:       reflect.TypeOf(time.Time{}),
	cfDictionaryTypeID: reflect.TypeOf(map[string]interface{}(nil)),
	cfNumberTypeID:     reflect.TypeOf(int64(0)),
	cfStringTypeID:     reflect.TypeOf(""),
}

var cfTypeNames = map[C.CFTypeID]string{
	cfArrayTypeID:      "CFArray",
	cfBooleanTypeID:    "CFBoolean",
	cfDataTypeID:       "CFData",
	cfDateTypeID:       "CFDate",
	cfDictionaryTypeID: "CFDictionary",
	cfNumberTypeID:     "CFNumber",
	cfStringTypeID:     "CFString",
}

func (state *unmarshalState) unmarshalValue(cfObj cfTypeRef, v reflect.Value) error {
	vType := v.Type()
	var unmarshaler Unmarshaler
	if u, ok := v.Interface().(Unmarshaler); ok {
		unmarshaler = u
	} else if vType.Kind() != reflect.Ptr && vType.Name() != "" && v.CanAddr() {
		// matching the encoding/json behavior here
		// If v is a named type and is addressable, check its address for Unmarshaler.
		vA := v.Addr()
		if u, ok := vA.Interface().(Unmarshaler); ok {
			unmarshaler = u
		}
	}
	if unmarshaler != nil {
		// flip over to the dumb conversion routine so we have something to give UnmarshalPlist()
		plist, err := convertCFTypeToInterface(cfObj)
		if err != nil {
			return err
		}
		if vType.Kind() == reflect.Ptr && v.IsNil() {
			v.Set(reflect.New(vType.Elem()))
			unmarshaler = v.Interface().(Unmarshaler)
		}
		return unmarshaler.UnmarshalPlist(plist)
	}
	if vType.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(vType.Elem()))
		}
		return state.unmarshalValue(cfObj, v.Elem())
	}
	typeID := C.CFGetTypeID(C.CFTypeRef(cfObj))
	vSetter := v      // receiver of any Set* calls
	vAddr := v.Addr() // used for re-setting v for maps/slices
	if vType.Kind() == reflect.Interface {
		if v.IsNil() {
			// pick an appropriate type based on the cfobj
			typ, ok := cfTypeMap[typeID]
			if !ok {
				return &UnknownCFTypeError{typeID}
			}
			if !typ.AssignableTo(vType) {
				// v must be some interface that our object doesn't conform to
				state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
				return nil
			}
			vSetter.Set(reflect.Zero(typ))
		}
		vAddr = v
		v = v.Elem()
		vType = v.Type()
	}
	switch typeID {
	case cfArrayTypeID:
		if vType.Kind() != reflect.Slice && vType.Kind() != reflect.Array {
			state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
			return nil
		}
		return convertCFArrayToSliceHelper(C.CFArrayRef(cfObj), func(elem cfTypeRef, idx, count int) (bool, error) {
			if idx == 0 && vType.Kind() == reflect.Slice {
				vSetter.Set(reflect.MakeSlice(vType, count, count))
				v = vAddr.Elem()
			} else if vType.Kind() == reflect.Array && idx >= v.Len() {
				return false, nil
			}
			if err := state.unmarshalValue(elem, v.Index(idx)); err != nil {
				return false, err
			}
			return true, nil
		})
	case cfBooleanTypeID:
		if vType.Kind() != reflect.Bool {
			state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
			return nil
		}
		vSetter.Set(reflect.ValueOf(C.CFBooleanGetValue(C.CFBooleanRef(cfObj)) != C.false))
		return nil
	case cfDataTypeID:
		if !byteSliceType.AssignableTo(vType) {
			state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
			return nil
		}
		vSetter.Set(reflect.ValueOf(convertCFDataToBytes(C.CFDataRef(cfObj))))
		return nil
	case cfDateTypeID:
		if !timeType.AssignableTo(vType) {
			state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
			return nil
		}
		vSetter.Set(reflect.ValueOf(convertCFDateToTime(C.CFDateRef(cfObj))))
	case cfDictionaryTypeID:
		if vType.Kind() == reflect.Map {
			// it's a map. Check its key type first
			if !stringType.AssignableTo(vType.Key()) {
				state.recordError(&UnmarshalTypeError{cfTypeNames[cfStringTypeID], vType.Key()})
				return nil
			}
			if v.IsNil() {
				vSetter.Set(reflect.MakeMap(vType))
				v = vAddr.Elem()
			}
			return convertCFDictionaryToMapHelper(C.CFDictionaryRef(cfObj), func(key string, value cfTypeRef, count int) error {
				keyVal := reflect.ValueOf(key)
				val := reflect.New(vType.Elem())
				if err := state.unmarshalValue(value, val); err != nil {
					return err
				}
				v.SetMapIndex(keyVal, val.Elem())
				return nil
			})
		} else if vType.Kind() == reflect.Struct {
			return convertCFDictionaryToMapHelper(C.CFDictionaryRef(cfObj), func(key string, value cfTypeRef, count int) error {
				// we need to iterate the fields because the tag might rename the key
				var f reflect.StructField
				var ok bool
				for i := 0; i < vType.NumField(); i++ {
					sf := vType.Field(i)
					tag := sf.Tag.Get("plist")
					if tag == "-" {
						// Pretend this field doesn't exist
						continue
					}
					if sf.Anonymous {
						// Match encoding/json's behavior here and pretend it doesn't exist
						continue
					}
					name, _ := parseTag(tag)
					if name == key {
						f = sf
						ok = true
						// This is unambiguously the right match
						break
					}
					if sf.Name == key {
						f = sf
						ok = true
					}
					// encoding/json does a case-insensitive match. Lets do that too
					if !ok && strings.EqualFold(sf.Name, key) {
						f = sf
						ok = true
					}
				}
				if ok {
					if f.PkgPath != "" {
						// this is an unexported field
						return &UnmarshalFieldError{key, vType, f}
					}
					vElem := v.FieldByIndex(f.Index)
					if err := state.unmarshalValue(value, vElem); err != nil {
						return err
					}
				}
				return nil
			})
		}
		state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
		return nil
	case cfNumberTypeID:
		switch vType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := convertCFNumberToInt64(C.CFNumberRef(cfObj))
			if v.OverflowInt(i) {
				state.recordError(&UnmarshalTypeError{cfTypeNames[typeID] + " " + strconv.FormatInt(i, 10), vType})
				return nil
			}
			if vSetter.Kind() == reflect.Interface {
				vSetter.Set(reflect.ValueOf(i))
			} else {
				vSetter.SetInt(i)
			}
			return nil
		case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u := uint64(convertCFNumberToUInt32(C.CFNumberRef(cfObj)))
			if v.OverflowUint(u) {
				state.recordError(&UnmarshalTypeError{cfTypeNames[typeID] + " " + strconv.FormatUint(u, 10), vType})
				return nil
			}
			if vSetter.Kind() == reflect.Interface {
				vSetter.Set(reflect.ValueOf(u))
			} else {
				vSetter.SetUint(u)
			}
			return nil
		case reflect.Float32, reflect.Float64:
			f := convertCFNumberToFloat64(C.CFNumberRef(cfObj))
			if v.OverflowFloat(f) {
				state.recordError(&UnmarshalTypeError{cfTypeNames[typeID] + " " + strconv.FormatFloat(f, 'f', -1, 64), vType})
				return nil
			}
			if vSetter.Kind() == reflect.Interface {
				vSetter.Set(reflect.ValueOf(f))
			} else {
				vSetter.SetFloat(f)
			}
			return nil
		}
		state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
		return nil
	case cfStringTypeID:
		if vType.Kind() != reflect.String {
			state.recordError(&UnmarshalTypeError{cfTypeNames[typeID], vType})
			return nil
		}
		vSetter.Set(reflect.ValueOf(convertCFStringToString(C.CFStringRef(cfObj))))
		return nil
	}
	return &UnknownCFTypeError{typeID}
}

func (state *unmarshalState) recordError(err error) {
	if state.err == nil {
		state.err = err
	}
}

// Marshaler is the interface implemented by objects that can marshal themselves
// into a property list.}
type Marshaler interface {
	MarshalPlist() (interface{}, error)
}

// Unmarshaler is the interface implemented by objects that can unmarshal a
// plist representation of themselves. The input can be assumed to be a valid
// basic property list object.
type Unmarshaler interface {
	UnmarshalPlist(interface{}) error
}

// An UnmarshalTypeError describes a plist value that was not appropriate for a
// value of a specific Go type.
type UnmarshalTypeError struct {
	Value string       // description of plist value - "CFBoolean, "CFArray", "CFNumber -5"
	Type  reflect.Type // type of Go value it could not be assigned to
}

func (e *UnmarshalTypeError) Error() string {
	return "plist: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

// An UnmarshalFieldError describes a plist dictionary key that led to an
// unexported (and therefore unwritable) struct field.
type UnmarshalFieldError struct {
	Key   string
	Type  reflect.Type
	Field reflect.StructField
}

func (e *UnmarshalFieldError) Error() string {
	return "plist: cannot unmarshal dictionary key " + strconv.Quote(e.Key) + " into unexported field " + e.Field.Name + " of type " + e.Type.String()
}

// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "plist: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "plist: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "plist: Unmarshal(nil " + e.Type.String() + ")"
}

// BUG(kballard): This package ignores anonymous (embedded) struct fields during
// encoding and decoding. This is done to maintain parity with the encoding/json
// package. At such time that encoding/json changes behavior, this package may
// also change. To force an anonymous field to be ignored in all future versions
// of this package, use an explicit `plist:"-"` tag in the struct definition.
