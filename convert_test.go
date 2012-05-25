package plist

import (
	"reflect"
	"testing"
	"testing/quick"
	"time"
)

func TestCFData(t *testing.T) {
	f := func(data []byte) []byte { return data }
	g := func(data []byte) []byte {
		cfData := convertBytesToCFData(data)
		if cfData == nil {
			t.Fatal("CFDataRef is NULL (%#v)", data)
		}
		defer cfRelease(cfTypeRef(cfData))
		return convertCFDataToBytes(cfData)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestCFString(t *testing.T) {
	// because the generator for string produces invalid strings,
	// lets generate []runes instead and convert those to strings in the function
	f := func(runes []rune) string { return string(runes) }
	g := func(runes []rune) string {
		cfStr := convertStringToCFString(string(runes))
		if cfStr == nil {
			t.Fatal("CFStringRef is NULL (%#v)", runes)
		}
		defer cfRelease(cfTypeRef(cfStr))
		return convertCFStringToString(cfStr)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestCFString_Invalid(t *testing.T) {
	// go ahead and generate random strings and see if we actually get objects back.
	// This is testing the unicode replacement functionality.
	// Just to be safe in case testing/quick ever fixes their string generation to
	// only generate valid strings, lets generate []bytes instead and then convert that.
	f := func(bytes []byte) bool {
		s := string(bytes)
		cfStr := convertStringToCFString(s)
		defer cfRelease(cfTypeRef(cfStr))
		return cfStr != nil
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}

	// Test some manually-crafted strings
	g := func(input, expected string) {
		cfStr := convertStringToCFString(input)
		defer cfRelease(cfTypeRef(cfStr))
		if cfStr == nil {
			t.Errorf("failed on input %#v", input)
			return
		}
		output := convertCFStringToString(cfStr)
		if output != expected {
			t.Errorf("failed on input: %#v. Output: %#v. Expected: %#v", input, output, expected)
		}
	}
	g("hello world", "hello world")
	g("hello\x00world", "hello\x00world")
	g("hello\uFFFDworld", "hello\uFFFDworld")
	g("hello\uFEFFworld\x00", "hello\uFEFFworld\x00")
	g("hello\x80world", "hello\uFFFDworld")
	g("hello\xFE\xFFworld", "hello\uFFFD\uFFFDworld")
}

func TestCFNumber_Int64(t *testing.T) {
	f := func(i int64) int64 { return i }
	g := func(i int64) int64 {
		cfNum := convertInt64ToCFNumber(i)
		if cfNum == nil {
			t.Fatal("CFNumberRef is NULL (%#v)", i)
		}
		defer cfRelease(cfTypeRef(cfNum))
		return convertCFNumberToInt64(cfNum)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestCFNumber_UInt32(t *testing.T) {
	f := func(i uint32) uint32 { return i }
	g := func(i uint32) uint32 {
		cfNum := convertUInt32ToCFNumber(i)
		if cfNum == nil {
			t.Fatal("CFNumberRef is NULL (%#v)", i)
		}
		defer cfRelease(cfTypeRef(cfNum))
		return convertCFNumberToUInt32(cfNum)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestCFNumber_Float64(t *testing.T) {
	f := func(f float64) float64 { return f }
	g := func(f float64) float64 {
		cfNum := convertFloat64ToCFNumber(f)
		if cfNum == nil {
			t.Fatal("CFNumberRef is NULL (%#v)", f)
		}
		defer cfRelease(cfTypeRef(cfNum))
		return convertCFNumberToFloat64(cfNum)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestCFBoolean(t *testing.T) {
	f := func(b bool) bool { return b }
	g := func(b bool) bool {
		cfBool := convertBoolToCFBoolean(b)
		if cfBool == nil {
			t.Fatal("CFBooleanRef is NULL (%#v)", b)
		}
		defer cfRelease(cfTypeRef(cfBool))
		return convertCFBooleanToBool(cfBool)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestCFDate(t *testing.T) {
	// We know the CFDate conversion explicitly truncates to milliseconds
	// because CFDates use floating point for representation.
	round := func(nano int64) int64 {
		return int64(time.Duration(nano) / time.Millisecond * time.Millisecond)
	}
	f := func(nano int64) time.Time { return time.Unix(0, round(nano)) }
	g := func(nano int64) time.Time {
		ti := time.Unix(0, round(nano))
		cfDate := convertTimeToCFDate(ti)
		if cfDate == nil {
			t.Fatal("CFDateRef is NULL (%#v)", ti)
		}
		defer cfRelease(cfTypeRef(cfDate))
		return convertCFDateToTime(cfDate)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}

func TestArbitrary(t *testing.T) {
	// test arbitrary values of any plistable type
	f := func(arb Arbitrary) interface{} { a, _ := standardize(arb.Value); return a }
	g := func(arb Arbitrary) interface{} {
		if cfObj, err := convertValueToCFType(reflect.ValueOf(arb.Value)); err != nil {
			t.Error(err)
		} else {
			defer cfRelease(cfTypeRef(cfObj))
			if val, err := convertCFTypeToInterface(cfObj); err != nil {
				t.Error(err)
			} else {
				a, _ := standardize(val)
				return a
			}
		}
		return nil
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		input := err.(*quick.CheckEqualError).In[0].(Arbitrary).Value
		t.Logf("Input value type: %T", input)
		t.Error(err)
	}
}
