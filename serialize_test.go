// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"testing"
)

func TestSerializeRoundTrip(t *testing.T) {
	cases := []string{
		`{"a":1,"b":"hello","c":true,"d":null,"e":[1,2,3]}`,
		`[1,2.5,"three",false,null,{"k":"v"}]`,
		`{}`,
		`[]`,
		`{"nested":{"deep":{"value":42}}}`,
	}
	ser := NewSerializer()
	for _, input := range cases {
		pj, err := Parse([]byte(input), nil)
		if err != nil {
			t.Fatal(err)
		}

		// Serialize
		data := ser.Serialize(nil, *pj)
		if len(data) == 0 {
			t.Fatal("empty serialized output")
		}

		// Deserialize
		pj2, err := ser.Deserialize(data, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Compare Interface() output
		iter1, _ := pj.Iter()
		val1, _ := iter1.Interface()
		iter2, _ := pj2.Iter()
		val2, _ := iter2.Interface()

		b1, _ := json.Marshal(val1)
		b2, _ := json.Marshal(val2)
		if string(b1) != string(b2) {
			t.Errorf("round-trip mismatch:\ninput: %s\ngot:   %s", b1, b2)
		}
		pj.Close()
		pj2.Close()
	}
}

func TestSerializeRoundTripMarshal(t *testing.T) {
	input := `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Animated":false,"IDs":[116,943,234,38793]}}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	ser := NewSerializer()
	data := ser.Serialize(nil, *pj)

	pj2, err := ser.Deserialize(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj2.Close()

	iter, _ := pj2.Iter()
	got, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Errorf("mismatch:\ngot:  %s\nwant: %s", got, input)
	}
}

func TestSerializeAfterMutation(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1,"b":2}`), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetNull()

	ser := NewSerializer()
	data := ser.Serialize(nil, *pj)

	pj2, _ := ser.Deserialize(data, nil)
	defer pj2.Close()

	iter2, _ := pj2.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != nil {
		t.Errorf("a = %v, want nil", m["a"])
	}
	if m["b"] != int64(2) {
		t.Errorf("b = %v, want 2", m["b"])
	}
}

func TestSerializeRealFiles(t *testing.T) {
	ser := NewSerializer()
	for _, name := range []string{"twitter", "canada", "citm_catalog"} {
		t.Run(name, func(t *testing.T) {
			data := loadTestFile(t, name)
			pj, err := Parse(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			serialized := ser.Serialize(nil, *pj)
			pj2, err := ser.Deserialize(serialized, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj2.Close()

			// Compare via MarshalJSON
			iter1, _ := pj.Iter()
			out1, _ := iter1.MarshalJSON()
			iter2, _ := pj2.Iter()
			out2, _ := iter2.MarshalJSON()

			if string(out1) != string(out2) {
				t.Errorf("round-trip mismatch for %s (first 200 bytes):\norig: %.200s\ndeser: %.200s", name, out1, out2)
			}
		})
	}
}

func TestDeserializeErrors(t *testing.T) {
	ser := NewSerializer()

	// Empty
	_, err := ser.Deserialize(nil, nil)
	if err == nil {
		t.Error("expected error for nil input")
	}

	// Bad version
	_, err = ser.Deserialize([]byte{99}, nil)
	if err == nil {
		t.Error("expected error for bad version")
	}

	// Truncated
	_, err = ser.Deserialize([]byte{1, 0, 0}, nil)
	if err == nil {
		t.Error("expected error for truncated input")
	}
}

func TestSerializeReuse(t *testing.T) {
	ser := NewSerializer()
	pj, _ := Parse([]byte(`{"x":1}`), nil)
	defer pj.Close()

	// Serialize twice, reusing serializer
	d1 := ser.Serialize(nil, *pj)
	d2 := ser.Serialize(nil, *pj)

	pj1, _ := ser.Deserialize(d1, nil)
	pj2, _ := ser.Deserialize(d2, nil)
	defer pj1.Close()
	defer pj2.Close()

	iter1, _ := pj1.Iter()
	iter2, _ := pj2.Iter()
	v1, _ := iter1.Interface()
	v2, _ := iter2.Interface()

	b1, _ := json.Marshal(v1)
	b2, _ := json.Marshal(v2)
	if string(b1) != string(b2) {
		t.Errorf("reuse mismatch: %s vs %s", b1, b2)
	}
}
