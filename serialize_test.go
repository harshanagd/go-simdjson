// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/binary"
	"encoding/json"
	"strings"
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

// blobWithLengths builds a serializer blob with arbitrary length prefixes and a
// tape body of bodyWords, to exercise Deserialize's framing checks directly.
func blobWithLengths(tapeLen, strLen uint64, bodyWords int) []byte {
	b := make([]byte, 0, 17+bodyWords*8)
	b = append(b, serializerVersion)
	var w [8]byte
	binary.NativeEndian.PutUint64(w[:], tapeLen)
	b = append(b, w[:]...)
	for i := 0; i < bodyWords; i++ {
		binary.NativeEndian.PutUint64(w[:], 0)
		b = append(b, w[:]...)
	}
	binary.NativeEndian.PutUint64(w[:], strLen)
	b = append(b, w[:]...)
	return b
}

// TestDeserializeRejectsOverflowingLengths covers the length prefixes multiplying
// or adding past the int range: tapeLen*8 and off+strLen wrap, so an unbounded
// make() ran with the raw value and panicked with "makeslice: len out of range".
func TestDeserializeRejectsOverflowingLengths(t *testing.T) {
	tests := []struct {
		name    string
		tapeLen uint64
		strLen  uint64
	}{
		{"tapeLen 2^61 multiplies to zero", 1 << 61, 0},
		{"tapeLen 2^62 multiplies to zero", 1 << 62, 0},
		{"tapeLen 2^60 multiplies negative", 1 << 60, 0},
		{"tapeLen MaxUint64 is negative as int", ^uint64(0), 0},
		{"strLen MaxUint64 is negative as int", 0, ^uint64(0)},
		{"strLen 2^63 is negative as int", 0, 1 << 63},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ser := NewSerializer()
			// A panic here fails the test rather than taking down the run.
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Deserialize panicked instead of erroring: %v", r)
				}
			}()
			if _, err := ser.Deserialize(blobWithLengths(tt.tapeLen, tt.strLen, 0), nil); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// TestDeserializeLengthBoundary pins where the framing bound sits. It tampers only
// the length prefix of a real blob so the tape body stays structurally valid and
// validate() is not what rejects it.
//
// This does NOT discriminate the overflow fix: for values that do not overflow, the
// old multiply and the new division agree, so it passes against both. Its job is to
// stop the bound drifting; TestDeserializeRejectsOverflowingLengths is what covers the
// fix itself.
func TestDeserializeLengthBoundary(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	ser := NewSerializer()
	good := ser.Serialize(nil, *pj)
	tapeLen := len(pj.tape.data)

	// Unmodified: accepted.
	if _, err := ser.Deserialize(good, nil); err != nil {
		t.Fatalf("valid blob rejected: %v", err)
	}

	withTapeLen := func(n uint64) []byte {
		b := make([]byte, len(good))
		copy(b, good)
		binary.NativeEndian.PutUint64(b[1:], n)
		return b
	}

	// The division bound counts the strings-length prefix and string body too, so it
	// is deliberately loose; tapeLen+1 still clears it. Two words past the real length
	// is what the bound itself rejects.
	if _, err := ser.Deserialize(withTapeLen(uint64(tapeLen)+2), nil); err == nil {
		t.Error("tapeLen past the bound: expected an error, got nil")
	} else if !strings.Contains(err.Error(), "truncated tape data") {
		t.Errorf("tapeLen past the bound: got %q, want a truncated tape data error", err)
	}

	// Far past the bound, but below the overflow threshold, so this exercises the
	// bound itself rather than the negative/wrap guards.
	_, err = ser.Deserialize(withTapeLen(uint64(len(good))), nil)
	if err == nil {
		t.Error("tapeLen far past the bound: expected an error, got nil")
	} else if !strings.Contains(err.Error(), "truncated tape data") {
		t.Errorf("tapeLen far past the bound: got %q, want a truncated tape data error", err)
	}
}

// TestDeserializeRejectsStructurallyInvalidTape covers the trust boundary end to end.
// Framing can be perfectly valid while the tape itself is not, and every walker
// indexes the tape assuming it is well formed, so Deserialize must reject it rather
// than hand it on. Before this, navigating such a tape panicked with an index out of
// range.
func TestDeserializeRejectsStructurallyInvalidTape(t *testing.T) {
	pj, err := Parse([]byte(`{"arr":[1,2,3]}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()

	ser := NewSerializer()
	staging, err := ser.Deserialize(ser.Serialize(nil, *pj), nil)
	if err != nil {
		t.Fatalf("Deserialize of a valid blob: %v", err)
	}
	corrupted := false
	for i, w := range staging.tape.data {
		if byte(w>>56) == tagArray {
			staging.tape.data[i] = uint64(tagArray)<<56 | 100000
			corrupted = true
			break
		}
	}
	if !corrupted {
		t.Fatal("no array entry found to corrupt")
	}

	victim, err := ser.Deserialize(ser.Serialize(nil, *staging), nil)
	if err == nil {
		t.Fatalf("accepted a tape whose container end index is past the tape (got %+v)", victim)
	}
	if !strings.Contains(err.Error(), "invalid tape") {
		t.Errorf("error = %q, want it to identify an invalid tape", err)
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
