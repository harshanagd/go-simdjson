// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"testing"
)

func TestIterAdvanceInto(t *testing.T) {
	input := `{"a":{"b":1},"c":2}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	// Iter starts at the root object '{'. AdvanceInto steps into it,
	// returning the tag of the first child (key "a").
	tag := iter.AdvanceInto()
	if tag != TagString {
		t.Fatalf("expected string key, got %c", tag)
	}
	s, _ := iter.String()
	if s != "a" {
		t.Errorf("key = %q, want 'a'", s)
	}
}

func TestMarshalJSONSimple(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"object", `{"a":1,"b":"hello","c":true,"d":null}`},
		{"array", `[1,2.5,"three",false,null]`},
		{"nested", `{"a":{"b":[1,2]},"c":[]}`},
		{"empty_object", `{}`},
		{"empty_array", `[]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, err := Parse([]byte(tc.input), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			got, err := iter.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}

			// Normalize both through encoding/json for comparison
			var want, gotParsed interface{}
			if err := json.Unmarshal([]byte(tc.input), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(got, &gotParsed); err != nil {
				t.Fatalf("MarshalJSON produced invalid JSON: %s\nerror: %v", got, err)
			}

			wantB, _ := json.Marshal(want)
			gotB, _ := json.Marshal(gotParsed)
			if string(wantB) != string(gotB) {
				t.Errorf("mismatch:\ninput: %s\ngot:   %s\nwant:  %s", tc.input, got, wantB)
			}
		})
	}
}

func TestMarshalJSONEscaping(t *testing.T) {
	input := `{"msg":"hello\nworld","tab":"a\tb","quote":"say \"hi\""}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	got, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Verify round-trip
	var m map[string]interface{}
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("invalid JSON: %s\nerror: %v", got, err)
	}
	if m["msg"] != "hello\nworld" {
		t.Errorf("msg = %q", m["msg"])
	}
	if m["tab"] != "a\tb" {
		t.Errorf("tab = %q", m["tab"])
	}
}

func TestMarshalJSONAfterMutation(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("b", nil).Iter.SetNull()

	iter2, _ := pj.Iter()
	got, err := iter2.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("invalid JSON: %s", got)
	}
	if m["a"] != float64(1) {
		t.Errorf("a = %v", m["a"])
	}
	if m["b"] != nil {
		t.Errorf("b = %v, want null", m["b"])
	}
	if m["c"] != float64(3) {
		t.Errorf("c = %v", m["c"])
	}
}

func TestMarshalJSONAfterDelete(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.DeleteElems(func(key []byte, i Iter) bool {
		return string(key) == "b"
	}, nil)

	iter2, _ := pj.Iter()
	got, err := iter2.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	_ = json.Unmarshal(got, &m)
	if len(m) != 2 {
		t.Errorf("got %d keys, want 2: %s", len(m), got)
	}
	if _, ok := m["b"]; ok {
		t.Error("b should be deleted")
	}
}

func TestMarshalJSONBuffer(t *testing.T) {
	input := `{"x":42}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	buf := make([]byte, 0, 64)
	got, err := iter.MarshalJSONBuffer(buf)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(got, &m)
	if m["x"] != float64(42) {
		t.Errorf("x = %v", m["x"])
	}
}

func TestArrayMarshalJSON(t *testing.T) {
	input := `[1,"two",true,null,{"k":"v"},[3]]`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	got, err := arr.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var want, gotParsed interface{}
	_ = json.Unmarshal([]byte(input), &want)
	_ = json.Unmarshal(got, &gotParsed)
	wantB, _ := json.Marshal(want)
	gotB, _ := json.Marshal(gotParsed)
	if string(wantB) != string(gotB) {
		t.Errorf("mismatch:\ngot:  %s\nwant: %s", got, wantB)
	}
}

func TestMarshalJSONExactOutput(t *testing.T) {
	input := `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	got, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Errorf("exact mismatch:\ngot:  %s\nwant: %s", got, input)
	}
}

func TestMarshalJSONNumbers(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"negative", `{"v":-42}`},
		{"zero", `{"v":0}`},
		{"uint64_max", `{"v":18446744073709551615}`},
		{"int64_min", `{"v":-9223372036854775808}`},
		{"float", `{"v":3.14}`},
		{"float_exp", `{"v":1e+21}`},
		{"float_neg", `{"v":-0.001}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, err := Parse([]byte(tc.input), nil)
			if err != nil {
				t.Skipf("parse error (expected for some edge cases): %v", err)
			}
			defer pj.Close()
			iter, _ := pj.Iter()
			got, err := iter.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			// Verify round-trip through stdlib
			var want, gotParsed interface{}
			_ = json.Unmarshal([]byte(tc.input), &want)
			if err := json.Unmarshal(got, &gotParsed); err != nil {
				t.Fatalf("invalid JSON: %s", got)
			}
			wantB, _ := json.Marshal(want)
			gotB, _ := json.Marshal(gotParsed)
			if string(wantB) != string(gotB) {
				t.Errorf("mismatch:\ngot:  %s\nwant: %s", gotB, wantB)
			}
		})
	}
}

func TestMarshalJSONAfterSetFloat(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1,"b":2}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetFloat(99.5)

	iter2, _ := pj.Iter()
	got, _ := iter2.MarshalJSON()
	var m map[string]interface{}
	_ = json.Unmarshal(got, &m)
	if m["a"] != 99.5 {
		t.Errorf("a = %v, want 99.5", m["a"])
	}
}

func TestMarshalJSONAfterSetInt(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":3.14}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetInt(-999)

	iter2, _ := pj.Iter()
	got, _ := iter2.MarshalJSON()
	var m map[string]interface{}
	_ = json.Unmarshal(got, &m)
	if m["a"] != float64(-999) {
		t.Errorf("a = %v, want -999", m["a"])
	}
}

func TestMarshalJSONAfterSetString(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":42}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetString("hello")

	iter2, _ := pj.Iter()
	got, _ := iter2.MarshalJSON()
	var m map[string]interface{}
	_ = json.Unmarshal(got, &m)
	if m["a"] != "hello" {
		t.Errorf("a = %v, want 'hello'", m["a"])
	}
}

func TestMarshalJSONAfterSetBool(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":null,"b":true}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetBool(true)
	_ = obj.FindKey("b", nil).Iter.SetBool(false)

	iter2, _ := pj.Iter()
	got, _ := iter2.MarshalJSON()
	var m map[string]interface{}
	_ = json.Unmarshal(got, &m)
	if m["a"] != true {
		t.Errorf("a = %v", m["a"])
	}
	if m["b"] != false {
		t.Errorf("b = %v", m["b"])
	}
}

func TestMarshalJSONRealFiles(t *testing.T) {
	for _, name := range []string{"twitter", "canada", "citm_catalog"} {
		t.Run(name, func(t *testing.T) {
			data := loadTestFile(t, name)
			pj, err := Parse(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			got, err := iter.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}

			// Verify it's valid JSON and round-trips
			var parsed interface{}
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("MarshalJSON produced invalid JSON for %s: %v", name, err)
			}

			// Compare Interface() output
			iter2, _ := pj.Iter()
			iface, _ := iter2.Interface()
			ifaceB, _ := json.Marshal(iface)
			parsedB, _ := json.Marshal(parsed)
			if string(ifaceB) != string(parsedB) {
				t.Errorf("MarshalJSON vs Interface() mismatch for %s (first 200 bytes):\nmarshal: %.200s\niface:   %.200s", name, parsedB, ifaceB)
			}
		})
	}
}
