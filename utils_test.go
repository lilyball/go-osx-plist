package plist

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"
)

// random utilities for tests

// findDifferences finds the first difference between two interface{}s and
// prints that object. This is useful with large Arbitrary values.
func findDifferences(t *testing.T, obj1, obj2 interface{}) {
	var loop func(reflect.Value, reflect.Value) (interface{}, interface{})
	loop = func(val1, val2 reflect.Value) (a, b interface{}) {
		if !val1.IsValid() || !val2.IsValid() {
			if val1.IsValid() == val2.IsValid() {
				return nil, nil
			}
			return val1.Interface(), val2.Interface()
		}
		typ1 := val1.Type()
		typ2 := val2.Type()
		if typ1 != typ2 {
			return val1.Interface(), val2.Interface()
		}
		switch typ1.Kind() {
		case reflect.Array:
			if val1.Len() != val2.Len() {
				return val1.Interface(), val2.Interface()
			}
			for i := 0; i < val1.Len(); i++ {
				if a, b := loop(val1.Index(i), val2.Index(i)); a != nil {
					return a, b
				}
			}
		case reflect.Slice:
			if val1.IsNil() != val2.IsNil() {
				return val1.Interface(), val2.Interface()
			}
			if val1.Len() != val2.Len() {
				return val1.Interface(), val2.Interface()
			}
			for i := 0; i < val1.Len(); i++ {
				if a, b := loop(val1.Index(i), val2.Index(i)); a != nil {
					return a, b
				}
			}
		case reflect.Map:
			if val1.IsNil() != val2.IsNil() {
				return val1.Interface(), val2.Interface()
			}
			if val1.Len() != val2.Len() {
				return val1.Interface(), val2.Interface()
			}
			for _, key := range val1.MapKeys() {
				elem1 := val1.MapIndex(key)
				elem2 := val2.MapIndex(key)
				if !elem2.IsValid() {
					// missing key
					return val1.Interface(), val2.Interface()
				}
				if a, b := loop(elem1, elem2); a != nil {
					return a, b
				}
			}
		case reflect.Interface:
			if val1.IsNil() != val2.IsNil() {
				if val1.IsNil() == val2.IsNil() {
					return nil, nil
				}
				return val1.Interface(), val2.Interface()
			}
			return loop(val1.Elem(), val2.Elem())
		default:
			// use reflect.DeepEqual to do the actual comparison
			// because it can dissect the interface objects
			if !reflect.DeepEqual(val1.Interface(), val2.Interface()) {
				return val1.Interface(), val2.Interface()
			}
		}
		return nil, nil
	}
	val1 := reflect.ValueOf(obj1)
	val2 := reflect.ValueOf(obj2)
	a, b := loop(val1, val2)
	if a != nil {
		la := ""
		va := reflect.ValueOf(a)
		if va.Kind() == reflect.Slice || va.Kind() == reflect.Map {
			if va.IsNil() {
				la = ",nil"
			} else {
				la = fmt.Sprintf(",%d", va.Len())
			}
		}
		lb := ""
		vb := reflect.ValueOf(b)
		if vb.Kind() == reflect.Slice || vb.Kind() == reflect.Map {
			if vb.IsNil() {
				lb = ",nil"
			} else {
				lb = fmt.Sprintf(",%d", vb.Len())
			}
		}
		t.Logf("Output difference: (%T%s) %v != (%T%s) %v", a, la, a, b, lb, b)
	} else if !reflect.DeepEqual(obj1, obj2) {
		t.Logf("No difference, but reflect.DeepEqual disagrees")
	} else {
		t.Logf("No difference")
	}
}

// Arbitrary is used with testing/quick to generate arbitrary plist values
type Arbitrary struct {
	Value interface{}
	Depth int
}

// generates valid utf-8-encoded strings
func generateString(rand *rand.Rand) (string, bool) {
	typ := reflect.TypeOf([]rune{})
	if val, ok := quick.Value(typ, rand); ok {
		return string(val.Interface().([]rune)), true
	}
	return "", false
}

func (a Arbitrary) Generate(rand *rand.Rand, size int) reflect.Value {
	n := int(math.Pow(5, float64(a.Depth+1)))
	r := rand.Intn(n + 2)
	if r >= n {
		// Containers
		switch r {
		case n: // Array
			num := rand.Intn(size)
			s := make([]interface{}, num)
			azero := Arbitrary{Depth: a.Depth + 1}
			for i := 0; i < num; i++ {
				s[i] = azero.Generate(rand, size).Interface().(Arbitrary).Value
			}
			return reflect.ValueOf(Arbitrary{Value: s})
		case n + 1: // Dictionary
			num := rand.Intn(size)
			m := make(map[string]interface{}, num)
			azero := Arbitrary{Depth: a.Depth + 1}
			for i := 0; i < num; i++ {
				key, ok := generateString(rand)
				if !ok {
					panic("Couldn't generate string")
					return reflect.Value{}
				}
				value := azero.Generate(rand, size).Interface().(Arbitrary).Value
				m[key] = value
			}
			return reflect.ValueOf(Arbitrary{Value: m})
		}
	} else {
		// Shallow values
		var typ reflect.Type
		switch r % 5 {
		case 0: // Boolean
			typ = reflect.TypeOf(false)
		case 1: // Data
			typ = reflect.TypeOf([]byte{})
		case 2: // Date
			// There is no built-in generator for time.Time. Generate nanoseconds instead
			typ = reflect.TypeOf(int64(0))
			if nanoVal, ok := quick.Value(typ, rand); ok {
				nano := nanoVal.Int()
				// trim to millisecond precision
				nano = nano / int64(time.Millisecond) * int64(time.Millisecond)
				return reflect.ValueOf(Arbitrary{Value: time.Unix(0, nano)})
			}
			panic("Couldn't generate date")
			return reflect.Value{}
		case 3: // Number
			switch rand.Intn(3) {
			case 0: // int64
				typ = reflect.TypeOf(int64(0))
			case 1: // uint32
				typ = reflect.TypeOf(uint32(0))
			case 2: // float64
				typ = reflect.TypeOf(float64(0))
			}
		case 4: // String
			// strings are special, since we need to ensure valid utf-8 encoding
			if str, ok := generateString(rand); ok {
				return reflect.ValueOf(Arbitrary{Value: str})
			}
			// conversion failed
			panic("Couldn't generate string")
			return reflect.Value{}
		}
		if val, ok := quick.Value(typ, rand); ok {
			return reflect.ValueOf(Arbitrary{Value: val.Interface()})
		}
	}
	panic("Can't generate value")
	return reflect.Value{}
}

// standardize converts any integer values that fit within an int64 into an int64.
// It also truncates any floating values that have no fractional part into an int64
// It also replaces empty slices with nil ones.
// It returns the new value, and a boolean indicating if any conversion took place
func standardize(obj interface{}) (newObj interface{}, changed bool) {
	val := reflect.ValueOf(obj)
	typ := val.Type()
	switch typ.Kind() {
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := val.Uint()
		if u <= math.MaxInt64 {
			return int64(u), true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return val.Int(), true
	case reflect.Float32, reflect.Float64:
		f := val.Float()
		_, rem := math.Modf(f)
		if rem == 0 {
			return int64(f), true
		}
	case reflect.Struct:
		// We truncate to millisecond precision in the conversion, but we can't even rely on that
		// for testing purposes because far-future timestamps lose even that much.
		// Truncate times to the nearest second
		if typ == reflect.TypeOf(time.Time{}) {
			t := obj.(time.Time)
			newT := time.Unix(t.Unix(), 0)
			return newT, !t.Equal(newT)
		}
	case reflect.Slice:
		if !val.IsNil() && val.Len() == 0 {
			return reflect.Zero(typ).Interface(), true
		}
		canChange := typ.Elem().Kind() == reflect.Interface
		if canChange {
			numElem := val.Len()
			for i := 0; i < numElem; i++ {
				elem := val.Index(i)
				if newElem, ok := standardize(elem.Interface()); ok {
					changed = true
					elem.Set(reflect.ValueOf(newElem))
				}
			}
		}
	case reflect.Map:
		canChangeKey := typ.Key().Kind() == reflect.Interface
		canChangeVal := typ.Elem().Kind() == reflect.Interface
		if canChangeKey || canChangeVal {
			keys := val.MapKeys()
			for _, key := range keys {
				elem := val.MapIndex(key)
				if canChangeKey {
					if newKey, ok := standardize(key); ok {
						changed = true
						val.SetMapIndex(key, reflect.Value{})
						key = reflect.ValueOf(newKey)
						val.SetMapIndex(key, elem)
					}
				}
				if canChangeVal {
					if newElem, ok := standardize(elem.Interface()); ok {
						changed = true
						val.SetMapIndex(key, reflect.ValueOf(newElem))
					}
				}
			}
		}
	}
	return obj, changed
}
