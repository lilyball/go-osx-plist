package plist

import (
	"testing"
	"testing/quick"
)

func TestCFData(t *testing.T) {
	f := func(data []byte) []byte { return data }
	g := func(data []byte) []byte {
		cfData := convertBytesToCFData(data)
		defer cfRelease(cfTypeRef(cfData))
		return convertCFDataToBytes(cfData)
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		t.Error(err)
	}
}
