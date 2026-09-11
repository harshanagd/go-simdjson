// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetFloat(t *testing.T) {
	input := `{"a":1,"b":3.14}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Set int to float
	elem := obj.FindKey("a", nil)
	if elem == nil {
		t.Fatal("key 'a' not found")
	}
	if err := elem.Iter.SetFloat(99.9); err != nil {
		t.Fatal(err)
	}
	f, err := elem.Iter.Float()
	if err != nil {
		t.Fatal(err)
	}
	if f != 99.9 {
		t.Errorf("SetFloat: got %v, want 99.9", f)
	}

	// Verify via Interface()
	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != 99.9 {
		t.Errorf("Interface: a = %v, want 99.9", m["a"])
	}

	// Bool should fail
	boolInput := `{"v":true}`
	pj2, _ := Parse([]byte(boolInput), nil)
	defer pj2.Close()
	iter3, _ := pj2.Iter()
	obj2, _ := iter3.Object(nil)
	if err := obj2.FindKey("v", nil).Iter.SetFloat(1.0); err == nil {
		t.Error("SetFloat on bool should fail")
	}
}

func TestSetInt(t *testing.T) {
	input := `{"a":3.14,"b":"hello","c":42}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	elem := obj.FindKey("a", nil)
	if err := elem.Iter.SetInt(-100); err != nil {
		t.Fatal(err)
	}
	v, err := elem.Iter.Int()
	if err != nil {
		t.Fatal(err)
	}
	if v != -100 {
		t.Errorf("SetInt: got %v, want -100", v)
	}
}

func TestSetUInt(t *testing.T) {
	input := `{"a":3.14}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	elem := obj.FindKey("a", nil)
	if err := elem.Iter.SetUInt(18446744073709551615); err != nil {
		t.Fatal(err)
	}
	v, err := elem.Iter.Uint()
	if err != nil {
		t.Fatal(err)
	}
	if v != 18446744073709551615 {
		t.Errorf("SetUInt: got %v, want max uint64", v)
	}
}

func TestSetString(t *testing.T) {
	input := `{"a":42,"b":"old"}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Set int to string
	elem := obj.FindKey("a", nil)
	if err := elem.Iter.SetString("new value"); err != nil {
		t.Fatal(err)
	}
	s, err := elem.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if s != "new value" {
		t.Errorf("SetString: got %q, want %q", s, "new value")
	}

	// Set string to string
	elem = obj.FindKey("b", nil)
	if err := elem.Iter.SetString("replaced"); err != nil {
		t.Fatal(err)
	}
	s, _ = elem.Iter.String()
	if s != "replaced" {
		t.Errorf("SetString: got %q, want %q", s, "replaced")
	}
}

func TestSetBool(t *testing.T) {
	input := `{"a":true,"b":false,"c":null}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// true → false
	elem := obj.FindKey("a", nil)
	if err := elem.Iter.SetBool(false); err != nil {
		t.Fatal(err)
	}
	v, _ := elem.Iter.Bool()
	if v != false {
		t.Error("SetBool(false): got true")
	}

	// false → true
	elem = obj.FindKey("b", nil)
	if err := elem.Iter.SetBool(true); err != nil {
		t.Fatal(err)
	}
	v, _ = elem.Iter.Bool()
	if v != true {
		t.Error("SetBool(true): got false")
	}

	// null → true
	elem = obj.FindKey("c", nil)
	if err := elem.Iter.SetBool(true); err != nil {
		t.Fatal(err)
	}
	v, _ = elem.Iter.Bool()
	if v != true {
		t.Error("SetBool on null: got false")
	}

	// int should fail
	intInput := `{"v":42}`
	pj2, _ := Parse([]byte(intInput), nil)
	defer pj2.Close()
	iter2, _ := pj2.Iter()
	obj2, _ := iter2.Object(nil)
	if err := obj2.FindKey("v", nil).Iter.SetBool(true); err == nil {
		t.Error("SetBool on int should fail")
	}
}

func TestSetNull(t *testing.T) {
	cases := []struct {
		name  string
		input string
		key   string
	}{
		{"bool", `{"v":true}`, "v"},
		{"int", `{"v":42}`, "v"},
		{"uint", `{"v":9223372036854775808}`, "v"},
		{"float", `{"v":3.14}`, "v"},
		{"string", `{"v":"hello"}`, "v"},
		{"object", `{"v":{"a":1}}`, "v"},
		{"array", `{"v":[1,2,3]}`, "v"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, err := Parse([]byte(tc.input), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			obj, _ := iter.Object(nil)
			elem := obj.FindKey(tc.key, nil)
			if elem == nil {
				t.Fatalf("key %q not found", tc.key)
			}
			if err := elem.Iter.SetNull(); err != nil {
				t.Fatal(err)
			}
			if elem.Iter.Type() != TypeNull {
				t.Errorf("type after SetNull = %v, want null", elem.Iter.Type())
			}

			// Verify via Interface() — value should be nil
			iter2, _ := pj.Iter()
			val, err := iter2.Interface()
			if err != nil {
				t.Fatal(err)
			}
			m := val.(map[string]interface{})
			if m[tc.key] != nil {
				t.Errorf("Interface()[%q] = %v, want nil", tc.key, m[tc.key])
			}
		})
	}
}

func TestSetNullOnNull(t *testing.T) {
	pj, _ := Parse([]byte(`{"v":null}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	if err := obj.FindKey("v", nil).Iter.SetNull(); err != nil {
		t.Fatal(err)
	}
}

func TestObjectDeleteElems(t *testing.T) {
	input := `{"one":1,"two":2.02,"three":"33333","four":false,"five":{"k":"v"},"six":[1,2]}`
	cases := []struct {
		name   string
		delete func(key string) bool
		want   map[string]bool // keys that should remain
	}{
		{
			"delete all",
			func(string) bool { return true },
			map[string]bool{},
		},
		{
			"delete none",
			func(string) bool { return false },
			map[string]bool{"one": true, "two": true, "three": true, "four": true, "five": true, "six": true},
		},
		{
			"delete one",
			func(k string) bool { return k == "one" },
			map[string]bool{"two": true, "three": true, "four": true, "five": true, "six": true},
		},
		{
			"delete nested object",
			func(k string) bool { return k == "five" },
			map[string]bool{"one": true, "two": true, "three": true, "four": true, "six": true},
		},
		{
			"delete array",
			func(k string) bool { return k == "six" },
			map[string]bool{"one": true, "two": true, "three": true, "four": true, "five": true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, _ := Parse([]byte(input), nil)
			defer pj.Close()
			iter, _ := pj.Iter()
			obj, _ := iter.Object(nil)
			err := obj.DeleteElems(func(key []byte, i Iter) bool {
				return tc.delete(string(key))
			}, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Verify via Interface()
			iter2, _ := pj.Iter()
			val, err := iter2.Interface()
			if err != nil {
				t.Fatal(err)
			}
			m := val.(map[string]interface{})
			if len(m) != len(tc.want) {
				t.Errorf("got %d keys, want %d", len(m), len(tc.want))
			}
			for k := range tc.want {
				if _, ok := m[k]; !ok {
					t.Errorf("missing key %q", k)
				}
			}
			for k := range m {
				if !tc.want[k] {
					t.Errorf("unexpected key %q", k)
				}
			}
		})
	}
}

func TestObjectDeleteElemsOnlyKeys(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	err := obj.DeleteElems(nil, map[string]struct{}{"a": {}, "c": {}})
	if err != nil {
		t.Fatal(err)
	}
	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if len(m) != 1 {
		t.Errorf("got %d keys, want 1", len(m))
	}
	if _, ok := m["b"]; !ok {
		t.Error("expected key 'b' to remain")
	}
}

func TestArrayDeleteElems(t *testing.T) {
	input := `[1,2.02,"hello",false,true,{"k":"v"},[1,2],null,-42]`
	cases := []struct {
		name string
		del  int // index to delete, -1 = all
		want int // expected remaining count
	}{
		{"delete all", -1, 0},
		{"delete first", 0, 8},
		{"delete string", 2, 8},
		{"delete object", 5, 8},
		{"delete array", 6, 8},
		{"delete last", 8, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, _ := Parse([]byte(input), nil)
			defer pj.Close()
			iter, _ := pj.Iter()
			arr, _ := iter.Array(nil)
			idx := 0
			arr.DeleteElems(func(i Iter) bool {
				del := tc.del < 0 || idx == tc.del
				idx++
				return del
			})

			iter2, _ := pj.Iter()
			val, err := iter2.Interface()
			if err != nil {
				t.Fatal(err)
			}
			a := val.([]interface{})
			if len(a) != tc.want {
				t.Errorf("got %d elements, want %d: %v", len(a), tc.want, a)
			}
		})
	}
}

func TestSetStringThenInterface(t *testing.T) {
	input := `{"a":42,"b":"old"}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	elem := obj.FindKey("a", nil)
	_ = elem.Iter.SetString("new")

	// Full round-trip via Interface
	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != "new" {
		t.Errorf("got %v, want 'new'", m["a"])
	}
}

func TestSetIntThenInterface(t *testing.T) {
	input := `{"a":3.14}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetInt(-999)

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != int64(-999) {
		t.Errorf("got %v (%T), want int64(-999)", m["a"], m["a"])
	}
}

func TestMutationTypeErrors(t *testing.T) {
	input := `{"obj":{"a":1},"arr":[1],"bool":true,"null":null,"int":42,"str":"hi"}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// SetFloat on bool should fail
	if err := obj.FindKey("bool", nil).Iter.SetFloat(1.0); err == nil {
		t.Error("SetFloat on bool should fail")
	}
	// SetFloat on string should fail (1-entry → 2-entry not possible)
	if err := obj.FindKey("str", nil).Iter.SetFloat(1.0); err == nil {
		t.Error("SetFloat on string should fail")
	}
	// SetInt on null should fail
	if err := obj.FindKey("null", nil).Iter.SetInt(1); err == nil {
		t.Error("SetInt on null should fail")
	}
	// SetBool on int should fail
	if err := obj.FindKey("int", nil).Iter.SetBool(true); err == nil {
		t.Error("SetBool on int should fail")
	}
	// SetString on bool should fail
	if err := obj.FindKey("bool", nil).Iter.SetString("x"); err == nil {
		t.Error("SetString on bool should fail")
	}
}

// TestMutationRoundTrip verifies mutations survive Interface() round-trip.
// Adapted from simdjson-go's mutation test patterns.
func TestMutationRoundTrip(t *testing.T) {
	input := `{"name":"Alice","age":30,"active":true,"score":9.5,"data":null}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	_ = obj.FindKey("name", nil).Iter.SetString("Bob")
	_ = obj.FindKey("age", nil).Iter.SetInt(25)
	_ = obj.FindKey("active", nil).Iter.SetBool(false)
	_ = obj.FindKey("score", nil).Iter.SetFloat(8.0)
	_ = obj.FindKey("data", nil).Iter.SetNull()

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})

	if m["name"] != "Bob" {
		t.Errorf("name = %v", m["name"])
	}
	if m["age"] != int64(25) {
		t.Errorf("age = %v", m["age"])
	}
	if m["active"] != false {
		t.Errorf("active = %v", m["active"])
	}
	if m["score"] != 8.0 {
		t.Errorf("score = %v", m["score"])
	}
	if m["data"] != nil {
		t.Errorf("data = %v", m["data"])
	}

	// Verify JSON round-trip
	b, _ := json.Marshal(m)
	var m2 map[string]interface{}
	_ = json.Unmarshal(b, &m2)
	if m2["name"] != "Bob" || m2["active"] != false {
		t.Errorf("JSON round-trip failed: %s", b)
	}
}

// --- NOP edge cases ---

func TestMultipleMutationsSameDoc(t *testing.T) {
	input := `{"a":1,"b":2,"c":3,"d":4}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Mutate all values to different types
	_ = obj.FindKey("a", nil).Iter.SetFloat(1.1)
	_ = obj.FindKey("b", nil).Iter.SetNull()
	_ = obj.FindKey("c", nil).Iter.SetString("three")
	_ = obj.FindKey("d", nil).Iter.SetInt(-4)

	iter2, _ := pj.Iter()
	val, err := iter2.Interface()
	if err != nil {
		t.Fatal(err)
	}
	m := val.(map[string]interface{})
	if m["a"] != 1.1 {
		t.Errorf("a = %v", m["a"])
	}
	if m["b"] != nil {
		t.Errorf("b = %v", m["b"])
	}
	if m["c"] != "three" {
		t.Errorf("c = %v", m["c"])
	}
	if m["d"] != int64(-4) {
		t.Errorf("d = %v", m["d"])
	}
}

func TestMutateAfterDelete(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Delete "b", then mutate "c"
	_ = obj.DeleteElems(func(key []byte, i Iter) bool {
		return string(key) == "b"
	}, nil)
	_ = obj.FindKey("c", nil).Iter.SetInt(99)

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if _, ok := m["b"]; ok {
		t.Error("b should be deleted")
	}
	if m["c"] != int64(99) {
		t.Errorf("c = %v, want 99", m["c"])
	}
	if m["a"] != int64(1) {
		t.Errorf("a = %v, want 1", m["a"])
	}
}

func TestForEachAfterDelete(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.DeleteElems(func(key []byte, i Iter) bool {
		return string(key) == "b"
	}, nil)

	keys := []string{}
	_ = obj.ForEach(func(key string, i Iter) error {
		keys = append(keys, key)
		return nil
	})
	if len(keys) != 2 {
		t.Errorf("ForEach after delete: got %v, want [a c]", keys)
	}
}

func TestDeleteThenDeleteAgain(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Delete "a", then delete "c"
	_ = obj.DeleteElems(func(key []byte, i Iter) bool {
		return string(key) == "a"
	}, nil)
	_ = obj.DeleteElems(func(key []byte, i Iter) bool {
		return string(key) == "c"
	}, nil)

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if len(m) != 1 {
		t.Errorf("got %d keys, want 1: %v", len(m), m)
	}
	if m["b"] != int64(2) {
		t.Errorf("b = %v", m["b"])
	}
}

// --- String append edge cases ---

func TestSetStringMultiple(t *testing.T) {
	input := `{"a":1,"b":2,"c":3}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Set all three to different strings
	_ = obj.FindKey("a", nil).Iter.SetString("first")
	_ = obj.FindKey("b", nil).Iter.SetString("second")
	_ = obj.FindKey("c", nil).Iter.SetString("third")

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != "first" || m["b"] != "second" || m["c"] != "third" {
		t.Errorf("got %v", m)
	}
}

func TestSetStringEmpty(t *testing.T) {
	input := `{"a":42}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetString("")

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != "" {
		t.Errorf("got %q, want empty string", m["a"])
	}
}

func TestSetStringUnicode(t *testing.T) {
	input := `{"a":42}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetString("café 日本語 🎉")

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != "café 日本語 🎉" {
		t.Errorf("got %q", m["a"])
	}
}

func TestSetStringOnString(t *testing.T) {
	input := `{"a":"old"}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetString("new")

	s, _ := obj.FindKey("a", nil).Iter.String()
	if s != "new" {
		t.Errorf("got %q, want 'new'", s)
	}
}

func TestNumberToStringThenInterface(t *testing.T) {
	input := `{"a":42,"b":3.14}`
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	_ = obj.FindKey("a", nil).Iter.SetString("was int")
	_ = obj.FindKey("b", nil).Iter.SetString("was float")

	iter2, _ := pj.Iter()
	val, _ := iter2.Interface()
	m := val.(map[string]interface{})
	if m["a"] != "was int" {
		t.Errorf("a = %v", m["a"])
	}
	if m["b"] != "was float" {
		t.Errorf("b = %v", m["b"])
	}
}

// TestSettersOnTruncatedTape covers the mutation counterpart of the truncated
// numeric entry: SetFloat/SetInt/SetUInt overwrite the second word in place, so
// they must refuse an entry that has no second word rather than panicking.
// Not reachable from Parse — see the truncated-tape tests in tape_test.go.
func TestSettersOnTruncatedTape(t *testing.T) {
	newIter := func(tag byte) Iter {
		tp := &Tape{data: []uint64{
			uint64(tagRoot) << 56,
			uint64(tag) << 56, // numeric tag word with no value word after it
		}}
		return Iter{tape: tp, tapeIdx: 1}
	}

	for _, tag := range []struct {
		name string
		tag  byte
	}{{"double", tagDouble}, {"int", tagInt64}, {"uint", tagUint64}} {
		t.Run("SetFloat_on_"+tag.name, func(t *testing.T) {
			it := newIter(tag.tag)
			if err := it.SetFloat(1.5); err == nil {
				t.Error("SetFloat on a truncated numeric entry returned no error")
			}
		})
		t.Run("SetInt_on_"+tag.name, func(t *testing.T) {
			it := newIter(tag.tag)
			if err := it.SetInt(7); err == nil {
				t.Error("SetInt on a truncated numeric entry returned no error")
			}
		})
		t.Run("SetUInt_on_"+tag.name, func(t *testing.T) {
			it := newIter(tag.tag)
			if err := it.SetUInt(7); err == nil {
				t.Error("SetUInt on a truncated numeric entry returned no error")
			}
		})
	}

	// A well-formed numeric entry must still be settable.
	pj, err := Parse([]byte(`{"n":1}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	e := obj.FindKey("n", nil)
	if err := e.Iter.SetFloat(2.5); err != nil {
		t.Fatalf("SetFloat on a well-formed entry: %v", err)
	}
	v, err := e.Iter.Float()
	if err != nil || v != 2.5 {
		t.Fatalf("Float() after SetFloat = %v, %v; want 2.5, nil", v, err)
	}
}

// TestDeleteElemsZeroValueObject covers the nil-object guard. Its condition changed
// when Object.tobj became a value field, since a value can never be nil.
func TestDeleteElemsZeroValueObject(t *testing.T) {
	var o Object
	err := o.DeleteElems(func(key []byte, i Iter) bool { return true }, nil)
	if err == nil {
		t.Fatal("DeleteElems on a zero Object returned no error")
	}
	if !strings.Contains(err.Error(), "nil object") {
		t.Errorf("err = %q, want a nil-object error", err)
	}
}

// TestSetNullOnContainerPreservesFollowingElements covers SetNull's container case
// NOP-filling one entry too many. endIdx is one PAST the closing tag, so an
// exclusive bound of endIdx+1 overwrote the entry after the container — the next key
// or element. It only showed when the container was NOT its parent's last child,
// which is why round-tripping a single-key object never caught it.
func TestSetNullOnContainerPreservesFollowingElements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		key   string
		want  string
	}{
		{"object value followed by a key", `{"v":{"a":1},"w":2}`, "v", `{"v":null,"w":2}`},
		{"array value followed by a key", `{"v":[1,2],"w":"keep"}`, "v", `{"v":null,"w":"keep"}`},
		{"two following keys", `{"v":{"a":1},"w":2,"x":3}`, "v", `{"v":null,"w":2,"x":3}`},
		{"container is the last child", `{"v":{"a":1}}`, "v", `{"v":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pj, err := Parse([]byte(tt.input), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			obj, err := iter.Object(nil)
			if err != nil {
				t.Fatalf("Object: %v", err)
			}
			elem := obj.FindKey(tt.key, nil)
			if elem == nil {
				t.Fatalf("key %q not found", tt.key)
			}
			if err := elem.Iter.SetNull(); err != nil {
				t.Fatalf("SetNull: %v", err)
			}

			// Read back through both layers: the corruption showed up as an error or
			// a silently missing entry depending on which followed the container.
			after, _ := pj.Iter()
			got, err := after.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON after SetNull: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}

			again, _ := pj.Iter()
			m, err := again.Interface()
			if err != nil {
				t.Fatalf("Interface after SetNull: %v", err)
			}
			var wantMap interface{}
			if err := json.Unmarshal([]byte(tt.want), &wantMap); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}
			gotJSON, _ := json.Marshal(m)
			wantJSON, _ := json.Marshal(wantMap)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("Interface = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// TestSettersRejectTruncatedNumeric covers the setters that shrink a 2-entry numeric
// to 1 entry by writing a NOP at idx+1. SetInt/SetUInt/SetFloat guarded that write;
// SetNull and SetStringBytes did not, so a numeric tag at the last tape entry — which
// a deserialized tape can carry — indexed out of range on assignment.
func TestSettersRejectTruncatedNumeric(t *testing.T) {
	truncated := func() *Tape {
		return &Tape{data: []uint64{
			tapeEntry(tagRoot, 3),
			tapeEntry(tagInt64, 0), // no value word follows
		}}
	}
	for _, tc := range []struct {
		name string
		call func(*Iter) error
	}{
		{"SetNull", func(i *Iter) error { return i.SetNull() }},
		{"SetStringBytes", func(i *Iter) error { return i.SetStringBytes([]byte("x")) }},
		{"SetString", func(i *Iter) error { return i.SetString("x") }},
		{"SetInt", func(i *Iter) error { return i.SetInt(1) }},
		{"SetUInt", func(i *Iter) error { return i.SetUInt(1) }},
		{"SetFloat", func(i *Iter) error { return i.SetFloat(1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on a truncated numeric: %v", r)
				}
			}()
			it := Iter{tape: truncated(), tapeIdx: 1}
			if err := tc.call(&it); err == nil {
				t.Error("expected an error for a numeric with no value word")
			}
		})
	}
}
