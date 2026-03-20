// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"testing"
)

func TestWithCopyStrings(t *testing.T) {
	input := []byte(`{"key":"value"}`)
	pj, _ := Parse(input, nil, WithCopyStrings(false))
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	elem := obj.FindKey("key", nil)
	s, _ := elem.Iter.String()
	if s != "value" {
		t.Errorf("got %q, want 'value'", s)
	}
}

func TestRootUint64(t *testing.T) {
	pj, _ := Parse([]byte(`18446744073709551615`), nil)
	defer pj.Close()
	v, err := pj.RootUint64()
	if err != nil {
		t.Fatal(err)
	}
	if v != 18446744073709551615 {
		t.Errorf("got %d", v)
	}
}

func TestIterPeekNext(t *testing.T) {
	pj, _ := Parse([]byte(`[1,"two",true]`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	_ = arr // just need iter positioned at array

	// Use TapeIter to test PeekNext
	tape, _ := pj.GetTape()
	ti := tape.Iter()
	// At root, advance into array
	ti.AdvanceInto()
	// Now at first element (1)
	next := ti.PeekNext()
	if next == Type(-1) {
		t.Error("PeekNext returned end")
	}
}

func TestStringRef(t *testing.T) {
	pj, _ := Parse([]byte(`{"k":"hello"}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	s, err := obj.FindKey("k", nil).Iter.StringRef()
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Errorf("got %q", s)
	}
}

func TestElementsMarshalJSON(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1,"b":"two","c":true}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	elems, err := obj.Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := elems.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("invalid JSON: %s", got)
	}
	if m["a"] != float64(1) || m["b"] != "two" || m["c"] != true {
		t.Errorf("got %v", m)
	}
}

func TestReadArrayNum(t *testing.T) {
	pj, _ := Parse([]byte(`[1,2.5,3]`), nil, UseNumber())
	defer pj.Close()
	iter, _ := pj.Iter()
	val, err := iter.Interface()
	if err != nil {
		t.Fatal(err)
	}
	arr := val.([]interface{})
	if len(arr) != 3 {
		t.Fatalf("got %d elements", len(arr))
	}
	if arr[0] != json.Number("1") {
		t.Errorf("[0] = %v (%T)", arr[0], arr[0])
	}
	if arr[1] != json.Number("2.5") {
		t.Errorf("[1] = %v (%T)", arr[1], arr[1])
	}
}

func TestTapeObjectIter(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1,"b":2}`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	// Use Tape.Interface to verify, then test Iter path
	ti := TapeIter{tape: tape, idx: 1} // skip root, at '{'
	obj, err := ti.Object()
	if err != nil {
		t.Fatal(err)
	}
	oi := obj.Iter()
	s, err := oi.String()
	if err != nil {
		t.Fatal(err)
	}
	if s != "a" {
		t.Errorf("first key = %q, want 'a'", s)
	}
}

func TestTapeIterFloat(t *testing.T) {
	pj, _ := Parse([]byte(`{"v":42}`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	ti := TapeIter{tape: tape, idx: 1}
	obj, _ := ti.Object()
	vi := obj.FindKey("v")
	if vi == nil {
		t.Fatal("key not found")
	}
	f, err := vi.Float()
	if err != nil {
		t.Fatal(err)
	}
	if f != 42.0 {
		t.Errorf("got %v, want 42.0", f)
	}
}

func TestTapeIterInterface(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":[1,true,"hi"]}`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	val, err := tape.Interface()
	if err != nil {
		t.Fatal(err)
	}
	m := val.(map[string]interface{})
	arr := m["a"].([]interface{})
	if len(arr) != 3 {
		t.Errorf("got %d elements", len(arr))
	}
}

func TestTapeArrayIter(t *testing.T) {
	pj, _ := Parse([]byte(`[10,20,30]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	ti := TapeIter{tape: tape, idx: 1}
	arr, err := ti.Array()
	if err != nil {
		t.Fatal(err)
	}
	ai := arr.Iter()
	v, err := ai.Int()
	if err != nil {
		t.Fatal(err)
	}
	if v != 10 {
		t.Errorf("first = %d, want 10", v)
	}
}
