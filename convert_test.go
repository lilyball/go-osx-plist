package plist

import (
	"testing"
	"testing/quick"
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
