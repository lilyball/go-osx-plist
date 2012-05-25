package plist

import (
	"fmt"
	"reflect"
	"testing"
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
