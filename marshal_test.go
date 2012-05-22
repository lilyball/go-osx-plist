package plist

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// Crib some of the test data from encoding/json

type Optionals struct {
	Sr string `plist:"sr"`
	So string `plist:"so,omitempty"`
	Sw string `plist:"-"`

	Ir int `plist:"omitempty"` // actually named omitempty, not an option
	Io int `plist:"io,omitempty"`

	Slr []string `plist:"slr,random"`
	Slo []string `plist:"slo,omitempty"`

	Mr map[string]interface{} `plist:"mr"`
	Mo map[string]interface{} `plist:",omitempty"`
}

var optionalsExpected = `{
	"sr": "",
	"omitempty": 0,
	"slr": [],
	"mr": {}
}`

func TestOmitEmpty(t *testing.T) {
	var o Optionals
	o.Sw = "something"
	o.Mr = map[string]interface{}{}
	o.Mo = map[string]interface{}{}

	data, err := Marshal(&o, CFPropertyListXMLFormat_v1_0)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: replace with Unmarshal()
	got, _, err := CFPropertyListCreateWithData(data)
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(optionalsExpected), &expected); err != nil {
		t.Fatal(err)
	}
	got, _ = standardize(got)
	sExpected, _ := standardize(expected)
	expected = sExpected.(map[string]interface{})
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got: %#v\nwant: %#v\n", got, expected)
	}
}

var unsupportedValues = []interface{}{
	math.NaN(),
	math.Inf(-1),
	math.Inf(1),
}

func TestUnsupportedValues(t *testing.T) {
	for _, v := range unsupportedValues {
		if _, err := Marshal(v, CFPropertyListXMLFormat_v1_0); err != nil {
			if _, ok := err.(*UnsupportedValueError); !ok {
				t.Errorf("for %v, got %T want UnsupportedValueError", v, err)
			}
		} else {
			t.Errorf("for %v, expected error", v)
		}
	}
}

// Ref has Marshaler and Unmarshaler methods with pointer receiver.
type Ref int

func (*Ref) MarshalPlist() (interface{}, error) {
	return "ref", nil
}

func (r *Ref) UnmarshalPlist(plist interface{}) error {
	*r = 12
	return nil
}

// Val has Marshaler methods with value receiver.
type Val int

func (Val) MarshalPlist() (interface{}, error) {
	return "val", nil
}

func TestRefValMarshal(t *testing.T) {
	var s = struct {
		R0 Ref
		R1 *Ref
		V0 Val
		V1 *Val
	}{
		R0: 12,
		R1: new(Ref),
		V0: 13,
		V1: new(Val),
	}
	var expected interface{}
	const want = `{"R0":"ref","R1":"ref","V0":"val","V1":"val"}`
	err := json.Unmarshal([]byte(want), &expected)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Marshal(&s, CFPropertyListXMLFormat_v1_0)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: replace with Unmarshal
	got, _, err := CFPropertyListCreateWithData(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %#v, want %#v", got, expected)
	}
}
