package plist

import (
	"reflect"
	"testing"
	"testing/quick"
)

// These tests rely on the Arbitrary type from convert_test.go

func TestArbitraryPlist(t *testing.T) {
	for _, inFormat := range []int{CFPropertyListXMLFormat_v1_0, CFPropertyListBinaryFormat_v1_0} {
		f := func(arb Arbitrary) (interface{}, int) { a, _ := standardize(arb.Value); return a, inFormat }
		g := func(arb Arbitrary) (interface{}, int) {
			if data, err := CFPropertyListCreateData(arb.Value, inFormat); err != nil {
				t.Error(err)
			} else {
				if plist, format, err := CFPropertyListCreateWithData(data); err != nil {
					t.Error(err)
				} else {
					a, _ := standardize(plist)
					return a, format
				}
			}
			return nil, 0
		}
		if err := quick.CheckEqual(f, g, nil); err != nil {
			out1 := err.(*quick.CheckEqualError).Out1[0]
			out2 := err.(*quick.CheckEqualError).Out2[0]
			if out1 != nil && out2 != nil {
				findDifferences(t, out1, out2)
			}
			t.Error(err)
		}
	}
}

func findDifferences(t *testing.T, obj1, obj2 interface{}) {
	var loop func(reflect.Value, reflect.Value) (interface{}, interface{})
	loop = func(val1, val2 reflect.Value) (a, b interface{}) {
		typ1 := val1.Type()
		typ2 := val2.Type()
		if typ1.Kind() != typ2.Kind() {
			return obj1, obj2
		}
		switch typ1.Kind() {
		case reflect.Slice, reflect.Array:
			len1 := val1.Len()
			len2 := val2.Len()
			if len1 != len2 {
				return obj1, obj2
			}
			for i := 0; i < len1; i++ {
				elem1 := val1.Index(i)
				elem2 := val2.Index(i)
				if a, b := loop(elem1, elem2); a != nil {
					return a, b
				}
			}
		case reflect.Map:
			len1 := val1.Len()
			len2 := val2.Len()
			if len1 != len2 {
				return obj1, obj2
			}
			keys := val1.MapKeys()
			for _, key := range keys {
				elem1 := val1.MapIndex(key)
				elem2 := val2.MapIndex(key)
				if !elem2.IsValid() {
					// missing key
					return obj1, obj2
				}
				if a, b := loop(elem1, elem2); a != nil {
					return a, b
				}
			}
		case reflect.Interface:
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
		t.Logf("Output difference: (%T) %#v != (%T) %#v", a, a, b, b)
	}
}
