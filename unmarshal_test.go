package plist

import (
	"encoding/json"
	"reflect"
	"testing"
	"testing/quick"
)

// The tests here are based off of the ones in encoding/json

type T struct {
	X string
	Y int
	Z int `plist:"-"`
}

type tx struct {
	x int
}

var txType = reflect.TypeOf((*tx)(nil)).Elem()

// A type that can unmarshal itself.

type unmarshaler struct {
	T bool
}

func (u *unmarshaler) UnmarshalPlist(plist interface{}) error {
	*u = unmarshaler{true} // All we need to see that UnmarshalPlist is called
	return nil
}

type ustruct struct {
	M unmarshaler
}

var (
	um0, um1 unmarshaler // target2 of unmarshaling
	ump      = &um1
	umtrue   = unmarshaler{true}
	umslice  = []unmarshaler{{true}}
	umslicep = new([]unmarshaler)
	umstruct = ustruct{unmarshaler{true}}
)

type unmarshalTest struct {
	in  string
	ptr interface{}
	out interface{}
	err error
}

var unmarshalTests = []unmarshalTest{
	// basic types
	{`true`, new(bool), true, nil},
	{`1`, new(int), 1, nil},
	{`1.2`, new(float64), 1.2, nil},
	{`-5`, new(int16), int16(-5), nil},
	{`"a\u1234"`, new(string), "a\u1234", nil},
	{`"http:\/\/"`, new(string), "http://", nil},
	{`"g-clef: \uD834\uDD1E"`, new(string), "g-clef: \U0001D11E", nil},
	{`"invalid: \uD834x\uDD1E"`, new(string), "invalid: \uFFFDx\uFFFD", nil},
	// skip the null one
	{`{"X": [1,2,3], "Y": 4}`, new(T), T{Y: 4}, &UnmarshalTypeError{"CFArray", reflect.TypeOf("")}},
	{`{"x": 1}`, new(tx), tx{}, &UnmarshalFieldError{"x", txType, txType.Field(0)}},

	// Z has a "-" tag.
	{`{"Y": 1, "Z": 2}`, new(T), T{Y: 1}, nil},

	// array tests
	{`[1, 2, 3]`, new([3]int), [3]int{1, 2, 3}, nil},
	{`[1, 2, 3]`, new([1]int), [1]int{1}, nil},
	{`[1, 2, 3]`, new([5]int), [5]int{1, 2, 3, 0, 0}, nil},

	// unmarshal interface test
	{`{"T":false}`, &um0, umtrue, nil}, // use "false" so test will fail if custom unmarshaler is not called
	{`{"T":false}`, &ump, &umtrue, nil},
	{`[{"T":false}]`, &umslice, umslice, nil},
	{`[{"T":false}]`, &umslicep, &umslice, nil},
	{`{"M":{"T":false}}`, &umstruct, umstruct, nil},

	// interface{} tests
	{`{"a":3,"m":{"s":[3,5,"yes"],"n":2.4},"b":false}`, new(interface{}), map[string]interface{}{"a": 3, "m": map[string]interface{}{"s": []interface{}{3, 5, "yes"}, "n": 2.4}, "b": false}, nil},
}

func TestUnmarshal(t *testing.T) {
	for i, tt := range unmarshalTests {
		var in interface{}
		if err := json.Unmarshal([]byte(tt.in), &in); err != nil {
			t.Errorf("#%d: %#v", i, err)
			continue
		}
		indata, err := Marshal(in, XMLFormat)
		if err != nil {
			t.Errorf("#%d: %#v", i, err)
			continue
		}
		if tt.ptr == nil {
			// why is this here? encoding/json's tests do this. But why?
			continue
		}
		v := reflect.New(reflect.TypeOf(tt.ptr).Elem())
		if _, err := Unmarshal(indata, v.Interface()); !reflect.DeepEqual(err, tt.err) {
			t.Errorf("#%d: %v want %v", i, err, tt.err)
			continue
		}
		// call standardize for the interface{} test(s)
		a := v.Elem().Interface()
		b := tt.out
		if v.Elem().Kind() == reflect.Interface {
			a, _ = standardize(v.Elem().Interface())
			b, _ = standardize(tt.out)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("#%d: mismatch\nhave: %#+v\nwant: %#+v", i, v.Elem().Interface(), tt.out)
			continue
		}
	}
}

func TestMarshalUnmarshalArbitrary(t *testing.T) {
	// this uses Arbitrary from convert_test.go
	f := func(arb Arbitrary) interface{} { a, _ := standardize(arb.Value); return a }
	g := func(arb Arbitrary) interface{} {
		data, err := Marshal(arb.Value, XMLFormat)
		if err != nil {
			t.Error(err)
			return nil
		}
		var result interface{}
		format, err := Unmarshal(data, &result)
		if err != nil {
			t.Error(err)
			return nil
		}
		if format != XMLFormat {
			t.Error(err)
			return nil
		}
		a, _ := standardize(result)
		return a
	}
	if err := quick.CheckEqual(f, g, nil); err != nil {
		out1 := err.(*quick.CheckEqualError).Out1[0]
		out2 := err.(*quick.CheckEqualError).Out2[0]
		findDifferences(t, out1, out2)
		t.Error(err)
	}
}
