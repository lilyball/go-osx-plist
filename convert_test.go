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
			t.Fatal("CFDataRef is NULL")
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
