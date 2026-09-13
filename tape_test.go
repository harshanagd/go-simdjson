package simdjson

import (
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

func TestTapeInterface(t *testing.T) {
	input := `{"name":"test","count":42,"arr":[1,true,null],"nested":{"x":3.14}}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	v, err := pj.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}

	m := v.(map[string]interface{})
	if m["name"] != "test" {
		t.Fatalf("name: expected 'test', got %v", m["name"])
	}
	if m["count"] != int64(42) {
		t.Fatalf("count: expected 42, got %v (%T)", m["count"], m["count"])
	}
	arr := m["arr"].([]interface{})
	if len(arr) != 3 || arr[0] != int64(1) || arr[1] != true || arr[2] != nil {
		t.Fatalf("arr: expected [1,true,null], got %v", arr)
	}
	nested := m["nested"].(map[string]interface{})
	if nested["x"] != 3.14 {
		t.Fatalf("nested.x: expected 3.14, got %v", nested["x"])
	}
}

func TestTapeRootType(t *testing.T) {
	tests := []struct {
		input    string
		wantType Type
	}{
		{`{"a":1}`, TypeObject},
		{`[1,2]`, TypeArray},
		{`"hello"`, TypeString},
		{`42`, TypeInt64},
		{`3.14`, TypeDouble},
		{`true`, TypeBool},
		{`null`, TypeNull},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pj, _ := Parse([]byte(tt.input), nil)
			defer pj.Close()
			tape, _ := pj.GetTape()
			if tape.RootType() != tt.wantType {
				t.Fatalf("expected %v, got %v", tt.wantType, tape.RootType())
			}
		})
	}
}

func TestTapeUseNumber(t *testing.T) {
	pj, _ := Parse([]byte(`{"i":42,"f":3.14}`), nil)
	defer pj.Close()

	v, err := pj.TapeInterfaceUseNumber()
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]interface{})
	if n, ok := m["i"].(json.Number); !ok || n.String() != "42" {
		t.Fatalf("i: expected json.Number '42', got %v (%T)", m["i"], m["i"])
	}
	if n, ok := m["f"].(json.Number); !ok || n.String() != "3.14" {
		t.Fatalf("f: expected json.Number '3.14', got %v (%T)", m["f"], m["f"])
	}
}

func TestTapeIter(t *testing.T) {
	pj, _ := Parse([]byte(`{"name":"test","count":42,"pi":3.14,"ok":true,"nil":null}`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()

	if iter.Type() != TypeObject {
		t.Fatalf("expected object, got %v", iter.Type())
	}

	obj, _ := iter.Object()

	t.Run("FindKey string", func(t *testing.T) {
		v, ok := obj.FindKey("name")
		if !ok {
			t.Fatal("not found")
		}
		s, _ := v.String()
		if s != "test" {
			t.Fatalf("expected 'test', got %q", s)
		}
	})

	t.Run("FindKey int", func(t *testing.T) {
		v, _ := obj.FindKey("count")
		n, _ := v.Int()
		if n != 42 {
			t.Fatalf("expected 42, got %d", n)
		}
	})

	t.Run("FindKey float", func(t *testing.T) {
		v, _ := obj.FindKey("pi")
		f, _ := v.Float()
		if f != 3.14 {
			t.Fatalf("expected 3.14, got %f", f)
		}
	})

	t.Run("FindKey bool", func(t *testing.T) {
		v, _ := obj.FindKey("ok")
		b, _ := v.Bool()
		if !b {
			t.Fatal("expected true")
		}
	})

	t.Run("FindKey missing", func(t *testing.T) {
		if _, ok := obj.FindKey("missing"); ok {
			t.Fatal("expected not-found")
		}
	})

	t.Run("Count", func(t *testing.T) {
		if obj.Count() != 5 {
			t.Fatalf("expected 5, got %d", obj.Count())
		}
	})
}

func TestTapeObjectForEach(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1,"b":"two","c":true}`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	obj, _ := iter.Object()

	var keys []string
	err := obj.ForEach(func(key string, val TapeIter) error {
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Fatalf("expected [a,b,c], got %v", keys)
	}
}

func TestTapeObjectMap(t *testing.T) {
	pj, _ := Parse([]byte(demo_json), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	obj, _ := iter.Object()

	m, err := obj.Map(nil)
	if err != nil {
		t.Fatal(err)
	}
	img := m["Image"].(map[string]interface{})
	if img["Width"] != int64(800) {
		t.Fatalf("Width: expected 800, got %v", img["Width"])
	}
}

func TestTapeFindPath(t *testing.T) {
	pj, _ := Parse([]byte(demo_json), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	obj, _ := iter.Object()

	v, ok := obj.FindPath("Image", "Thumbnail", "Url")
	if !ok {
		t.Fatal("not found")
	}
	s, _ := v.String()
	if s != "http://www.example.com/image/481989943" {
		t.Fatalf("expected URL, got %q", s)
	}

	if _, ok := obj.FindPath("Image", "Missing"); ok {
		t.Fatal("expected not-found for missing path")
	}
}

func TestTapeArray(t *testing.T) {
	pj, _ := Parse([]byte(`[10,20,30]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()

	if arr.Count() != 3 {
		t.Fatalf("expected 3, got %d", arr.Count())
	}

	vals, _ := arr.AsInteger()
	if len(vals) != 3 || vals[0] != 10 || vals[1] != 20 || vals[2] != 30 {
		t.Fatalf("expected [10,20,30], got %v", vals)
	}
}

func TestTapeArrayAsString(t *testing.T) {
	pj, _ := Parse([]byte(`["a","b","c"]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()

	vals, _ := arr.AsString()
	if len(vals) != 3 || vals[0] != "a" || vals[1] != "b" || vals[2] != "c" {
		t.Fatalf("expected [a,b,c], got %v", vals)
	}
}

func TestTapeArrayAsFloat(t *testing.T) {
	pj, _ := Parse([]byte(`[1.1,2.2,3.3]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()

	vals, _ := arr.AsFloat()
	if len(vals) != 3 || vals[0] != 1.1 || vals[1] != 2.2 || vals[2] != 3.3 {
		t.Fatalf("expected [1.1,2.2,3.3], got %v", vals)
	}
}

// deleteFromArray parses input, deletes every element for which drop returns
// true via the mutation API, and returns a TapeArray over the resulting tape.
// The deletions leave NOP padding behind, which is what these tests exercise.
func deleteFromArray(t *testing.T, input string, drop func(i Iter) bool) (*ParsedJson, TapeArray) {
	t.Helper()
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	iter, err := pj.Iter()
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	arr, err := iter.Array(nil)
	if err != nil {
		t.Fatalf("Array: %v", err)
	}
	arr.DeleteElems(drop)

	tape, err := pj.GetTape()
	if err != nil {
		t.Fatalf("GetTape: %v", err)
	}
	ti := tape.Iter()
	tarr, err := ti.Array()
	if err != nil {
		t.Fatalf("TapeIter.Array: %v", err)
	}
	return pj, tarr
}

// deleteFromObject is the object counterpart of deleteFromArray.
func deleteFromObject(t *testing.T, input string, drop func(key []byte, i Iter) bool) (*ParsedJson, TapeObject) {
	t.Helper()
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	iter, err := pj.Iter()
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	if err := obj.DeleteElems(drop, nil); err != nil {
		t.Fatalf("DeleteElems: %v", err)
	}

	tape, err := pj.GetTape()
	if err != nil {
		t.Fatalf("GetTape: %v", err)
	}
	ti := tape.Iter()
	tobj, err := ti.Object()
	if err != nil {
		t.Fatalf("TapeIter.Object: %v", err)
	}
	return pj, tobj
}

// TestTapeArrayForEachSkipsNops covers the NOP padding DeleteElems leaves on the
// tape. ForEach previously handed the callback an iterator sitting on a NOP,
// whose Type() is a non-value, so every derived helper failed on any array that
// had had an element removed.
func TestTapeArrayForEachSkipsNops(t *testing.T) {
	tests := []struct {
		name  string
		input string
		drop  func(i Iter) bool
		want  []int64
	}{
		{
			name:  "delete_first",
			input: `[1,2,3,4]`,
			drop:  func(i Iter) bool { v, _ := i.Int(); return v == 1 },
			want:  []int64{2, 3, 4},
		},
		{
			name:  "delete_middle",
			input: `[1,2,3,4]`,
			drop:  func(i Iter) bool { v, _ := i.Int(); return v == 2 || v == 3 },
			want:  []int64{1, 4},
		},
		{
			name:  "delete_last",
			input: `[1,2,3,4]`,
			drop:  func(i Iter) bool { v, _ := i.Int(); return v == 4 },
			want:  []int64{1, 2, 3},
		},
		{
			name:  "delete_all",
			input: `[1,2,3]`,
			drop:  func(i Iter) bool { return true },
			want:  []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pj, arr := deleteFromArray(t, tt.input, tt.drop)
			defer pj.Close()

			var got []int64
			err := arr.ForEach(func(val TapeIter) error {
				v, err := val.Int()
				if err != nil {
					return err
				}
				got = append(got, v)
				return nil
			})
			if err != nil {
				t.Fatalf("ForEach: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ForEach yielded %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("ForEach yielded %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestTapeArrayAsIntegerAfterDelete checks the derived helpers, which all route
// through ForEach and so all inherited the NOP defect.
func TestTapeArrayAsIntegerAfterDelete(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3,4]`, func(i Iter) bool {
		v, _ := i.Int()
		return v == 2
	})
	defer pj.Close()

	got, err := arr.AsInteger()
	if err != nil {
		t.Fatalf("AsInteger after delete: %v", err)
	}
	want := []int64{1, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("AsInteger = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AsInteger = %v, want %v", got, want)
		}
	}
}

// TestTapeArrayAsStringAfterDelete covers the string path, which fails on a NOP
// with a type error rather than silently, since readString would be handed the
// NOP's payload.
func TestTapeArrayAsStringAfterDelete(t *testing.T) {
	pj, arr := deleteFromArray(t, `["a","b","c"]`, func(i Iter) bool {
		v, _ := i.String()
		return v == "b"
	})
	defer pj.Close()

	got, err := arr.AsString()
	if err != nil {
		t.Fatalf("AsString after delete: %v", err)
	}
	want := []string{"a", "c"}
	if len(got) != len(want) {
		t.Fatalf("AsString = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AsString = %v, want %v", got, want)
		}
	}
}

// TestTapeArrayAsFloatAfterDelete covers the float path. Structurally identical
// to the integer path but on a distinct tag, and doubles are 2-word entries so
// the NOP run length differs.
func TestTapeArrayAsFloatAfterDelete(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1.5,2.5,3.5]`, func(i Iter) bool {
		v, _ := i.Float()
		return v == 2.5
	})
	defer pj.Close()

	got, err := arr.AsFloat()
	if err != nil {
		t.Fatalf("AsFloat after delete: %v", err)
	}
	want := []float64{1.5, 3.5}
	if len(got) != len(want) {
		t.Fatalf("AsFloat = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AsFloat = %v, want %v", got, want)
		}
	}
}

// TestTapeArrayForEachNestedAfterDelete exercises the one path the scalar tests
// miss: skipValue taking its object/array branch on the element immediately
// following a NOP run. A container advances via its header's end index rather
// than a fixed width, so an off-by-one in the skip would land mid-container.
func TestTapeArrayForEachNestedAfterDelete(t *testing.T) {
	pj, arr := deleteFromArray(t, `[{"x":1},{"y":2},{"z":3}]`, func(i Iter) bool {
		obj, err := i.Object(nil)
		if err != nil {
			return false
		}
		return obj.FindKey("x", nil) != nil
	})
	defer pj.Close()

	var keys []string
	err := arr.ForEach(func(val TapeIter) error {
		obj, err := val.Object()
		if err != nil {
			return err
		}
		return obj.ForEach(func(key string, _ TapeIter) error {
			keys = append(keys, key)
			return nil
		})
	})
	if err != nil {
		t.Fatalf("ForEach over nested objects after delete: %v", err)
	}
	if len(keys) != 2 || keys[0] != "y" || keys[1] != "z" {
		t.Fatalf("keys = %v, want [y z]", keys)
	}
}

// --- NOP handling across the remaining tape walkers ---
//
// Every walker below previously read the tape without stepping over the NOP
// padding DeleteElems leaves behind, so each reported a NOP as if it were a
// value. Deleting the FIRST entry is the discriminating case: the NOP then sits
// exactly where the walker starts.

func TestTapeArrayIterSkipsNops(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool {
		v, _ := i.Int()
		return v == 1
	})
	defer pj.Close()

	it := arr.Iter()
	if got := it.Type(); got != TypeInt64 {
		t.Fatalf("Iter().Type() = %v, want %v", got, TypeInt64)
	}
	v, err := it.Int()
	if err != nil || v != 2 {
		t.Fatalf("Iter().Int() = %v, %v; want 2, nil", v, err)
	}
}

func TestTapeArrayIterAllDeletedIsPastEnd(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool { return true })
	defer pj.Close()

	it := arr.Iter()
	if got := it.Type(); got != Type(-1) {
		t.Fatalf("Iter().Type() on fully deleted array = %v, want Type(-1)", got)
	}
}

func TestTapeArrayFirstTypeSkipsNops(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool {
		v, _ := i.Int()
		return v == 1
	})
	defer pj.Close()

	if got := arr.FirstType(); got != TypeInt64 {
		t.Fatalf("FirstType() = %v, want %v", got, TypeInt64)
	}

	pj2, arr2 := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool { return true })
	defer pj2.Close()
	if got := arr2.FirstType(); got != Type(-1) {
		t.Fatalf("FirstType() on fully deleted array = %v, want Type(-1)", got)
	}
}

func TestTapeArrayInterfaceSkipsNops(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool {
		v, _ := i.Int()
		return v == 1
	})
	defer pj.Close()

	// Before the fix this returned [nil 2 3]: readValue reports a NOP as a nil
	// value, which the open-coded walk appended.
	got, err := arr.Interface()
	if err != nil {
		t.Fatalf("Interface: %v", err)
	}
	if len(got) != 2 || got[0] != int64(2) || got[1] != int64(3) {
		t.Fatalf("Interface() = %#v, want [2 3]", got)
	}
}

func TestTapeArrayInterfaceUseNumberSkipsNops(t *testing.T) {
	// Interface() now delegates to readArray/readArrayNum; check the UseNumber
	// branch is still selected after the rewrite.
	pj, err := Parse([]byte(`[1,2,3]`), nil, UseNumber())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	iter, _ := pj.Iter()
	a, _ := iter.Array(nil)
	a.DeleteElems(func(i Iter) bool { s, _ := i.StringCvt(); return s == "1" })

	tape, _ := pj.GetTape()
	ti := tape.Iter()
	tarr, err := ti.Array()
	if err != nil {
		t.Fatalf("Array: %v", err)
	}
	got, err := tarr.Interface()
	if err != nil {
		t.Fatalf("Interface: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Interface() = %#v, want 2 elements", got)
	}
	if _, ok := got[0].(json.Number); !ok {
		t.Fatalf("Interface()[0] = %T, want json.Number (UseNumber not honoured)", got[0])
	}
}

func TestTapeObjectIterSkipsNops(t *testing.T) {
	pj, obj := deleteFromObject(t, `{"a":1,"b":2}`, func(key []byte, i Iter) bool {
		return string(key) == "a"
	})
	defer pj.Close()

	it := obj.Iter()
	if got := it.Type(); got != TypeString {
		t.Fatalf("Iter().Type() = %v, want %v (the key \"b\")", got, TypeString)
	}
	k, err := it.String()
	if err != nil || k != "b" {
		t.Fatalf("Iter().String() = %q, %v; want \"b\", nil", k, err)
	}
}

func TestTapeObjectIterAllDeletedIsPastEnd(t *testing.T) {
	pj, obj := deleteFromObject(t, `{"a":1,"b":2}`, func(key []byte, i Iter) bool { return true })
	defer pj.Close()

	it := obj.Iter()
	if got := it.Type(); got != Type(-1) {
		t.Fatalf("Iter().Type() on fully deleted object = %v, want Type(-1)", got)
	}
}

func TestTapeObjectMapSkipsNops(t *testing.T) {
	pj, obj := deleteFromObject(t, `{"a":1,"b":2}`, func(key []byte, i Iter) bool {
		return string(key) == "a"
	})
	defer pj.Close()

	// Before the fix this returned an empty map with a nil error: the walk broke
	// on the leading NOP's tag, silently discarding every remaining key.
	m, err := obj.Map(nil)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if len(m) != 1 || m["b"] != int64(2) {
		t.Fatalf("Map() = %#v, want map[b:2]", m)
	}
}

func TestTapeObjectMapMergesIntoDst(t *testing.T) {
	pj, obj := deleteFromObject(t, `{"a":1,"b":2}`, func(key []byte, i Iter) bool {
		return string(key) == "a"
	})
	defer pj.Close()

	dst := map[string]interface{}{"keep": "me", "b": "overwrite me"}
	m, err := obj.Map(dst)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if m["keep"] != "me" {
		t.Errorf("Map(dst) dropped a pre-existing key not present in the object")
	}
	if m["b"] != int64(2) {
		t.Errorf("Map(dst)[\"b\"] = %#v, want 2 (existing key should be overwritten)", m["b"])
	}
}

func TestTapeIterAdvanceSkipsNops(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool {
		v, _ := i.Int()
		return v == 2
	})
	defer pj.Close()

	cur := arr.Iter()
	if got := cur.Type(); got != TypeInt64 {
		t.Fatalf("start Type() = %v, want %v", got, TypeInt64)
	}
	if got := cur.PeekNext(); got != TypeInt64 {
		t.Fatalf("PeekNext() over a NOP run = %v, want %v", got, TypeInt64)
	}
	if got := cur.Advance(); got != TypeInt64 {
		t.Fatalf("Advance() over a NOP run = %v, want %v", got, TypeInt64)
	}
	v, err := cur.Int()
	if err != nil || v != 3 {
		t.Fatalf("Int() after Advance = %v, %v; want 3, nil", v, err)
	}
}

func TestTapeIterAdvanceIntoSkipsNops(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1,"b":2}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	if err := obj.DeleteElems(func(key []byte, i Iter) bool { return string(key) == "a" }, nil); err != nil {
		t.Fatalf("DeleteElems: %v", err)
	}

	tape, _ := pj.GetTape()
	root := tape.Iter()
	if got := root.AdvanceInto(); got != TypeString {
		t.Fatalf("AdvanceInto() = %v, want %v (the key \"b\")", got, TypeString)
	}
	k, err := root.String()
	if err != nil || k != "b" {
		t.Fatalf("String() after AdvanceInto = %q, %v; want \"b\", nil", k, err)
	}
}

func TestTapeIterAdvanceIntoEmptyContainer(t *testing.T) {
	for _, input := range []string{`[]`, `{}`} {
		t.Run(input, func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			defer pj.Close()
			tape, _ := pj.GetTape()
			ti := tape.Iter()
			if got := ti.AdvanceInto(); got != Type(-1) {
				t.Fatalf("AdvanceInto() on %s = %v, want Type(-1)", input, got)
			}
		})
	}
}

func TestTapeIterAdvanceIntoFullyDeletedContainer(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	if err := obj.DeleteElems(func(key []byte, i Iter) bool { return true }, nil); err != nil {
		t.Fatalf("DeleteElems: %v", err)
	}

	tape, _ := pj.GetTape()
	ti := tape.Iter()
	if got := ti.AdvanceInto(); got != Type(-1) {
		t.Fatalf("AdvanceInto() on a fully deleted object = %v, want Type(-1)", got)
	}
}

func TestLargeNumbers(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantType  Type
		wantFloat float64
	}{
		{"max_int64", `9223372036854775807`, false, TypeInt64, 0},
		{"min_int64", `-9223372036854775808`, false, TypeInt64, 0},
		{"max_uint64", `18446744073709551615`, false, TypeUint64, 0},
		{"overflow_uint64", `18446744073709551616`, true, TypeNull, 0},
		{"huge_int", `99999999999999999999`, true, TypeNull, 0},
		{"huge_negative", `-99999999999999999999`, true, TypeNull, 0},
		{"float_big", `1e308`, false, TypeDouble, 1e308},
		{"float_tiny", `5e-324`, false, TypeDouble, 5e-324},
		{"float_overflow", `1e309`, true, TypeNull, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pj, err := Parse([]byte(tt.input), nil)
			if tt.wantErr {
				if err == nil {
					pj.Close()
					t.Fatal("expected parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			defer pj.Close()

			if pj.RootType() != tt.wantType {
				t.Fatalf("expected type %v, got %v", tt.wantType, pj.RootType())
			}

			if tt.wantType == TypeDouble {
				v, err := pj.RootDouble()
				if err != nil {
					t.Fatalf("RootDouble failed: %v", err)
				}
				if v != tt.wantFloat {
					t.Fatalf("expected %v, got %v", tt.wantFloat, v)
				}
			}
		})
	}
}

func TestLargeNumberStringCvt(t *testing.T) {
	// max uint64 via StringCvt
	pj, _ := Parse([]byte(`18446744073709551615`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	s, err := iter.StringCvt()
	if err != nil {
		t.Fatal(err)
	}
	if s != "18446744073709551615" {
		t.Fatalf("expected '18446744073709551615', got %q", s)
	}
}

func TestLargeNumberUseNumber(t *testing.T) {
	// Numbers that overflow int64 but fit in uint64
	pj, _ := Parse([]byte(`{"big":18446744073709551615}`), nil, UseNumber())
	defer pj.Close()
	v, _ := pj.TapeInterfaceUseNumber()
	m := v.(map[string]interface{})
	n := m["big"].(json.Number)
	if n.String() != "18446744073709551615" {
		t.Fatalf("expected '18446744073709551615', got %q", n.String())
	}
}

func TestTapeAdvance(t *testing.T) {
	pj, _ := Parse([]byte(`[1,"two",true]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()
	ai := arr.Iter()

	if ai.Type() != TypeInt64 {
		t.Fatalf("first: expected int64, got %v", ai.Type())
	}
	if ai.Advance() != TypeString {
		t.Fatal("second: expected string")
	}
	if ai.Advance() != TypeBool {
		t.Fatal("third: expected bool")
	}
}

func TestTapePeekNext(t *testing.T) {
	pj, _ := Parse([]byte(`[1,"two"]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()
	ai := arr.Iter()

	if ai.PeekNext() != TypeString {
		t.Fatalf("peek: expected string, got %v", ai.PeekNext())
	}
	if ai.Type() != TypeInt64 {
		t.Fatal("should still be at int64")
	}
}

func TestTapeAdvanceInto(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1,"b":2}`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()

	typ := iter.AdvanceInto()
	if typ != TypeString {
		t.Fatalf("expected string (first key), got %v", typ)
	}
	s, _ := iter.String()
	if s != "a" {
		t.Fatalf("expected 'a', got %q", s)
	}
}

func TestTapeFindElement(t *testing.T) {
	pj, _ := Parse([]byte(demo_json), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()

	v, ok := iter.FindElement("Image", "Width")
	if !ok {
		t.Fatal("not found")
	}
	n, _ := v.Int()
	if n != 800 {
		t.Fatalf("expected 800, got %d", n)
	}
}

func TestTapeStringCvt(t *testing.T) {
	pj, _ := Parse([]byte(`[42, 3.14, true, null, "hi"]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()
	ai := arr.Iter()

	tests := []string{"42", "3.14", "true", "null", "hi"}
	for i, want := range tests {
		s, err := ai.StringCvt()
		if err != nil {
			t.Fatalf("[%d] err: %v", i, err)
		}
		if s != want {
			t.Fatalf("[%d] expected %q, got %q", i, want, s)
		}
		if i < len(tests)-1 {
			ai.Advance()
		}
	}
}

func TestTapeArrayFirstType(t *testing.T) {
	pj, _ := Parse([]byte(`[true, 1]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()
	if arr.FirstType() != TypeBool {
		t.Fatalf("expected bool, got %v", arr.FirstType())
	}
}

func TestTapeArrayInterface(t *testing.T) {
	pj, _ := Parse([]byte(`[1,"two",true]`), nil)
	defer pj.Close()
	tape, _ := pj.GetTape()
	iter := tape.Iter()
	arr, _ := iter.Array()
	v, err := arr.Interface()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != int64(1) || v[1] != "two" || v[2] != true {
		t.Fatalf("expected [1,two,true], got %v", v)
	}
}

// --- #8: UseNumber reaches the tape API ---

func TestUseNumberAppliesToTapeAPI(t *testing.T) {
	pj, err := Parse([]byte(`{"n":42,"arr":[1,2]}`), nil, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	// Tape.Interface via ParsedJson
	v, err := pj.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]interface{})
	if _, ok := m["n"].(json.Number); !ok {
		t.Errorf("TapeInterface ignored UseNumber: got %T", m["n"])
	}

	tape, err := pj.GetTape()
	if err != nil {
		t.Fatal(err)
	}

	// TapeIter.Interface
	ti := tape.Iter()
	tv, err := ti.Interface()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tv.(map[string]interface{})["n"].(json.Number); !ok {
		t.Error("TapeIter.Interface ignored UseNumber")
	}

	obj, err := ti.Object()
	if err != nil {
		t.Fatal(err)
	}

	// TapeObject.Map
	mp, err := obj.Map(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mp["n"].(json.Number); !ok {
		t.Errorf("TapeObject.Map ignored UseNumber: got %T", mp["n"])
	}

	// TapeArray.Interface
	arrIter, ok := obj.FindKey("arr")
	if !ok {
		t.Fatal("key 'arr' not found")
	}
	arr, err := arrIter.Array()
	if err != nil {
		t.Fatal(err)
	}
	av, err := arr.Interface()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := av[0].(json.Number); !ok {
		t.Errorf("TapeArray.Interface ignored UseNumber: got %T", av[0])
	}
}

func TestUseNumberSurvivesTapeClone(t *testing.T) {
	pj, err := Parse([]byte(`{"n":42}`), nil, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	cloned := pj.Clone(nil)
	pj.Close()

	v, err := cloned.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(map[string]interface{})["n"].(json.Number); !ok {
		t.Error("Tape.Clone dropped useNumber")
	}
}

// TestTapeObjectForEachPropagatesKeyError covers the readString error that
// ForEach previously discarded (`key, _ :=`), which silently yielded an empty
// key rather than reporting a corrupt string offset. Not reachable from Parse,
// so the tape is built directly; a corrupt tape is reachable in practice via
// Serializer.Deserialize, which performs no structural validation.
func TestTapeObjectForEachPropagatesKeyError(t *testing.T) {
	// Key entry's payload points past the end of the string buffer.
	keyEntry := uint64(tagString)<<56 | 64
	tape := &Tape{
		data:        []uint64{0, keyEntry, uint64(tagNull) << 56},
		strings:     make([]byte, 8),
		copyStrings: true,
	}
	obj := &TapeObject{tape: tape, startIdx: 1, endIdx: 3}

	called := false
	err := obj.ForEach(func(key string, val TapeIter) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("ForEach accepted an out-of-range key offset without an error")
	}
	if called {
		t.Error("callback was invoked despite the key failing to decode")
	}

	// Map delegates to ForEach, so it must surface the same error.
	if _, err := obj.Map(nil); err == nil {
		t.Error("Map did not propagate ForEach's key error")
	}
}

// TestTapeObjectFindKeySkipsNops covers the FindKey guard conversion. Deleting
// the FIRST key puts a NOP exactly where the scan starts, so an unguarded scan
// breaks immediately and reports every remaining key as missing.
func TestTapeObjectFindKeySkipsNops(t *testing.T) {
	pj, obj := deleteFromObject(t, `{"a":1,"b":2,"c":3}`, func(key []byte, i Iter) bool {
		return string(key) == "a"
	})
	defer pj.Close()

	for _, want := range []struct {
		key string
		val int64
	}{{"b", 2}, {"c", 3}} {
		it, ok := obj.FindKey(want.key)
		if !ok {
			t.Fatalf("FindKey(%q) not found after deleting a preceding key", want.key)
		}
		v, err := it.Int()
		if err != nil || v != want.val {
			t.Fatalf("FindKey(%q).Int() = %v, %v; want %d, nil", want.key, v, err, want.val)
		}
	}
	if _, ok := obj.FindKey("a"); ok {
		t.Error("FindKey found the deleted key")
	}
}

// TestReadObjectSkipsNops reaches readObject/readObjectNum, the object twins of
// readArray/readArrayNum. Map no longer routes through them (it delegates to
// ForEach), so a whole-value Interface() on a mutated object is the only path.
func TestReadObjectSkipsNops(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []ParserOption
		want interface{}
	}{
		{"readObject", nil, int64(2)},
		{"readObjectNum", []ParserOption{UseNumber()}, json.Number("2")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pj, err := Parse([]byte(`{"outer":{"a":1,"b":2}}`), nil, tc.opts...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			outer, _ := iter.Object(nil)
			inner, err := outer.FindKey("outer", nil).Iter.Object(nil)
			if err != nil {
				t.Fatalf("inner Object: %v", err)
			}
			if err := inner.DeleteElems(func(key []byte, i Iter) bool {
				return string(key) == "a"
			}, nil); err != nil {
				t.Fatalf("DeleteElems: %v", err)
			}

			tape, _ := pj.GetTape()
			v, err := tape.Interface()
			if err != nil {
				t.Fatalf("Tape.Interface after delete: %v", err)
			}
			m, ok := v.(map[string]interface{})
			if !ok {
				t.Fatalf("Interface() = %T, want map", v)
			}
			got, ok := m["outer"].(map[string]interface{})
			if !ok {
				t.Fatalf("outer = %T, want map", m["outer"])
			}
			if len(got) != 1 || got["b"] != tc.want {
				t.Fatalf("outer = %#v, want map[b:%#v]", got, tc.want)
			}
		})
	}
}

// TestTapeBoolAfterNop covers a NOP skip landing on a `false` element, which the
// int64-based tests above do not reach. The raw tag is 'f' and must be reported
// as TypeBool.
//
// Note this does NOT discriminate the switch to Tag.Type() in FirstType and
// PeekNext: the inline `if tag == tagFalse` form they replaced maps `false`
// identically. Tag.Type() differs only on a zero tag (TagEnd -> Type(-1)), which
// a well-formed parse never produces — see the RootType issue for that case.
func TestTapeBoolAfterNop(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,false,true]`, func(i Iter) bool {
		v, err := i.Int()
		return err == nil && v == 1
	})
	defer pj.Close()

	if got := arr.FirstType(); got != TypeBool {
		t.Errorf("FirstType() after a NOP, on `false` = %v, want %v", got, TypeBool)
	}

	// PeekNext across a NOP onto `false`.
	pj2, err := Parse([]byte(`[1,2,false]`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj2.Close()
	iter, _ := pj2.Iter()
	a, _ := iter.Array(nil)
	a.DeleteElems(func(i Iter) bool { v, err := i.Int(); return err == nil && v == 2 })

	tape, _ := pj2.GetTape()
	rootIter := tape.Iter()
	tarr, err := rootIter.Array()
	if err != nil {
		t.Fatalf("Array: %v", err)
	}
	cur := tarr.Iter()
	if got := cur.PeekNext(); got != TypeBool {
		t.Errorf("PeekNext() across a NOP, onto `false` = %v, want %v", got, TypeBool)
	}
}

// TestTapeIterExhaustedAccessorsDoNotPanic covers the consequence of Iter()
// positioning past the tape for an empty container: every accessor must return
// an error rather than indexing out of range.
func TestTapeIterExhaustedAccessorsDoNotPanic(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool { return true })
	defer pj.Close()

	it := arr.Iter()
	if got := it.Type(); got != Type(-1) {
		t.Fatalf("Type() = %v, want Type(-1)", got)
	}
	if _, err := it.Int(); err == nil {
		t.Error("Int() on an exhausted iterator returned no error")
	}
	if _, err := it.Uint(); err == nil {
		t.Error("Uint() on an exhausted iterator returned no error")
	}
	if _, err := it.Float(); err == nil {
		t.Error("Float() on an exhausted iterator returned no error")
	}
	if _, err := it.Bool(); err == nil {
		t.Error("Bool() on an exhausted iterator returned no error")
	}
	if _, err := it.String(); err == nil {
		t.Error("String() on an exhausted iterator returned no error")
	}
	if _, err := it.Object(); err == nil {
		t.Error("Object() on an exhausted iterator returned no error")
	}
	if _, err := it.Array(); err == nil {
		t.Error("Array() on an exhausted iterator returned no error")
	}
	if got := it.AdvanceInto(); got != Type(-1) {
		t.Errorf("AdvanceInto() on an exhausted iterator = %v, want Type(-1)", got)
	}
	// Advance and PeekNext call skipValue, which reads the tag at the cursor, so
	// they need the bound checked on the INPUT rather than only on the result.
	if got := it.Advance(); got != Type(-1) {
		t.Errorf("Advance() on an exhausted iterator = %v, want Type(-1)", got)
	}
	if got := it.PeekNext(); got != Type(-1) {
		t.Errorf("PeekNext() on an exhausted iterator = %v, want Type(-1)", got)
	}
}

// TestIterExhaustedPeekNextTagDoesNotPanic is the Iter-layer counterpart: it
// builds its own skipValue call and so needs its own input guard.
func TestIterExhaustedPeekNextTagDoesNotPanic(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool { return true })
	defer pj.Close()

	cur := arr.Iter()
	ei := Iter{tape: cur.tape, tapeIdx: cur.idx}
	if got := ei.PeekNextTag(); got != TagEnd {
		t.Errorf("PeekNextTag() on an exhausted iterator = %q, want TagEnd", rune(got))
	}
	if got := ei.Advance(); got != Type(-1) {
		t.Errorf("Iter.Advance() on an exhausted iterator = %v, want Type(-1)", got)
	}
	if got := ei.PeekNext(); got != Type(-1) {
		t.Errorf("Iter.PeekNext() on an exhausted iterator = %v, want Type(-1)", got)
	}
	if got := ei.AdvanceInto(); got != TagEnd {
		t.Errorf("Iter.AdvanceInto() on an exhausted iterator = %q, want TagEnd", rune(got))
	}
	var dst Iter
	if got, err := ei.AdvanceIter(&dst); got != Type(-1) || err != nil {
		t.Errorf("AdvanceIter() on an exhausted iterator = %v, %v; want Type(-1), nil", got, err)
	}
}

// TestRootTypeAgreesAcrossLayers covers the tag-to-Type mapping at the root.
// Tape.RootType previously returned the raw tag, so a document whose root is
// `false` reported unknown(102) while ParsedJson.RootType (which routes through
// TapeIter.Type) reported bool — the two disagreed on the same document.
func TestRootTypeAgreesAcrossLayers(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Type
	}{
		{`false`, TypeBool},
		{`true`, TypeBool},
		{`{"a":1}`, TypeObject},
		{`[1]`, TypeArray},
		{`"s"`, TypeString},
		{`1`, TypeInt64},
		{`1.5`, TypeDouble},
		{`null`, TypeNull},
	} {
		t.Run(tc.input, func(t *testing.T) {
			pj, err := Parse([]byte(tc.input), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			defer pj.Close()
			tape, err := pj.GetTape()
			if err != nil {
				t.Fatalf("GetTape: %v", err)
			}
			if got := tape.RootType(); got != tc.want {
				t.Errorf("Tape.RootType() = %v, want %v", got, tc.want)
			}
			if got := pj.RootType(); got != tc.want {
				t.Errorf("ParsedJson.RootType() = %v, want %v", got, tc.want)
			}
			ti := tape.Iter()
			if got := ti.Type(); got != tc.want {
				t.Errorf("TapeIter.Type() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestZeroTagReportsEndSentinel covers an in-bounds zero tag, which a well-formed
// parse never produces but Serializer.Deserialize can yield from a corrupt
// payload since it performs no structural validation. Every accessor must agree
// that it is the end sentinel rather than one calling it Type(0).
func TestZeroTagReportsEndSentinel(t *testing.T) {
	tape := &Tape{data: []uint64{
		uint64(tagRoot) << 56,
		0, // zero tag where the root value should be
	}}

	if got := tape.RootType(); got != Type(-1) {
		t.Errorf("Tape.RootType() on a zero tag = %v, want Type(-1)", got)
	}
	ti := TapeIter{tape: tape, idx: 1}
	if got := ti.Type(); got != Type(-1) {
		t.Errorf("TapeIter.Type() on a zero tag = %v, want Type(-1)", got)
	}
	if got := Tag(tape.tapeTagAt(1)).Type(); got != Type(-1) {
		t.Errorf("Tag.Type() on a zero tag = %v, want Type(-1)", got)
	}
}

// TestTapeArrayForEachTrailingNop covers a NOP run at the very END of a
// container, where the post-advance skip must stop rather than yielding the
// padding as a value.
//
// It is a boundary case, not an overrun guard: the closing bracket halts the
// scan and the `pos < endIdx` loop bound blocks callbacks past end, so a
// too-large skip limit would be masked here.
func TestTapeArrayForEachTrailingNop(t *testing.T) {
	pj, arr := deleteFromArray(t, `[1,2,3]`, func(i Iter) bool {
		v, _ := i.Int()
		return v == 2 || v == 3
	})
	defer pj.Close()

	var got []int64
	if err := arr.ForEach(func(val TapeIter) error {
		v, err := val.Int()
		if err != nil {
			return err
		}
		got = append(got, v)
		return nil
	}); err != nil {
		t.Fatalf("ForEach: %v", err)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("ForEach = %v, want [1]", got)
	}
}

// --- #12: a tape truncated between a numeric tag word and its value word ---
//
// Parse never produces this, but Serializer.Deserialize rebuilds a Tape from
// arbitrary bytes with only length-prefix checks and no structural validation, so
// a corrupt payload can. Every read of the second word must error rather than
// index out of range. The tapes below are built directly because the guard is not
// reachable through the public parse path.

// truncatedNumericTape returns a tape whose last entry is a numeric TAG with no
// following value word, and a cursor positioned on it.
func truncatedNumericTape(tag byte) (*Tape, TapeIter) {
	tp := &Tape{data: []uint64{
		uint64(tagRoot) << 56,
		uint64(tag) << 56, // tag word at index 1, no value word at index 2
	}}
	return tp, TapeIter{tape: tp, idx: 1}
}

func TestTapeIterNumericAccessorsOnTruncatedTape(t *testing.T) {
	t.Run("Int", func(t *testing.T) {
		_, ti := truncatedNumericTape(tagInt64)
		if _, err := ti.Int(); err == nil {
			t.Error("Int() on a truncated int64 entry returned no error")
		}
	})
	t.Run("Uint_from_uint_tag", func(t *testing.T) {
		_, ti := truncatedNumericTape(tagUint64)
		if _, err := ti.Uint(); err == nil {
			t.Error("Uint() on a truncated uint64 entry returned no error")
		}
	})
	t.Run("Uint_from_int_tag", func(t *testing.T) {
		// Uint also accepts an int64 tag, a second path to the same read.
		_, ti := truncatedNumericTape(tagInt64)
		if _, err := ti.Uint(); err == nil {
			t.Error("Uint() on a truncated int64 entry returned no error")
		}
	})
	for _, tag := range []struct {
		name string
		tag  byte
	}{{"double", tagDouble}, {"int", tagInt64}, {"uint", tagUint64}} {
		t.Run("Float_from_"+tag.name, func(t *testing.T) {
			// Float accepts all three numeric tags, so each is a distinct read.
			_, ti := truncatedNumericTape(tag.tag)
			if _, err := ti.Float(); err == nil {
				t.Errorf("Float() on a truncated %s entry returned no error", tag.name)
			}
		})
	}
}

func TestReadValueOnTruncatedTape(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tag       byte
		useNumber bool
	}{
		{"readValue_int", tagInt64, false},
		{"readValue_uint", tagUint64, false},
		{"readValue_double", tagDouble, false},
		{"readValueNum_int", tagInt64, true},
		{"readValueNum_uint", tagUint64, true},
		{"readValueNum_double", tagDouble, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tp, ti := truncatedNumericTape(tc.tag)
			tp.useNumber = tc.useNumber

			// TapeIter.Interface routes to readValue / readValueNum.
			if _, err := ti.Interface(); err == nil {
				t.Error("TapeIter.Interface() on a truncated numeric entry returned no error")
			}
			// Tape.Interface enters at index 1, the same entry.
			if _, err := tp.Interface(); err == nil {
				t.Error("Tape.Interface() on a truncated numeric entry returned no error")
			}
		})
	}
}

func TestMarshalTapeOnTruncatedTape(t *testing.T) {
	for _, tc := range []struct {
		name string
		tag  byte
	}{{"int", tagInt64}, {"uint", tagUint64}, {"double", tagDouble}} {
		t.Run(tc.name, func(t *testing.T) {
			tp, _ := truncatedNumericTape(tc.tag)
			it := Iter{tape: tp, tapeIdx: 1}
			if _, err := it.MarshalJSON(); err == nil {
				t.Errorf("MarshalJSON() on a truncated %s entry returned no error", tc.name)
			}
		})
	}
}

// TestStringCvtOnTruncatedTape covers the layer above the numeric accessors: both
// StringCvt implementations previously discarded their error (`v, _ := ti.Int()`),
// so a truncated numeric entry produced "0" with a nil error instead of surfacing
// the failure. Array.AsStringCvt inherits the fix by delegation.
func TestStringCvtOnTruncatedTape(t *testing.T) {
	for _, tc := range []struct {
		name string
		tag  byte
	}{{"int", tagInt64}, {"uint", tagUint64}, {"double", tagDouble}} {
		t.Run(tc.name, func(t *testing.T) {
			tp, ti := truncatedNumericTape(tc.tag)

			s, err := ti.StringCvt()
			if err == nil {
				t.Errorf("TapeIter.StringCvt() on a truncated %s entry returned %q with no error", tc.name, s)
			}

			it := Iter{tape: tp, tapeIdx: 1}
			s, err = it.StringCvt()
			if err == nil {
				t.Errorf("Iter.StringCvt() on a truncated %s entry returned %q with no error", tc.name, s)
			}
		})
	}
}

// TestRootDocGuardIsConsistentAcrossEntryPoints locks the four entry points that
// read from tape index 1 to one answer about whether a document is there.
//
// They used to disagree. RootType, Interface and InterfaceUseNumber each tested
// `len(t.data) < 2`, which confirms a value slot exists but never looks at index 0
// — so a tape with no root marker passed. ForEach tested hasRootAt(0), which
// confirms the marker but not that a value follows. Neither implies the other, and
// on a tape of length 2 with no root marker ForEach reported zero documents with a
// NIL ERROR while the other three refused it. All four now share hasRootDoc.
//
// None of this is reachable from Parse. Serializer.Deserialize rebuilds a Tape
// from arbitrary bytes checking length prefixes but not structure, so it can
// produce either shape.
func TestRootDocGuardIsConsistentAcrossEntryPoints(t *testing.T) {
	tests := []struct {
		name     string
		data     []uint64
		wantDoc  bool
		wantType Type
		wantVal  interface{}
	}{
		{
			// Faithful single-document block: opening root payload is the index one
			// past the closing root — the padding slot simdjson never writes.
			name:     "well_formed",
			data:     rootBlock(0, tagInt64, 42),
			wantDoc:  true,
			wantType: TypeInt64,
			wantVal:  int64(42),
		},
		{
			// Passes a bare length check, fails hasRootAt(0): this is the row where
			// ForEach used to succeed silently.
			name:     "length_ok_but_no_root_marker",
			data:     []uint64{uint64(tagInt64) << 56, 42},
			wantDoc:  false,
			wantType: Type(-1),
		},
		{
			// Passes hasRootAt(0), fails a length check: a marker with no value.
			name:     "root_marker_with_no_value",
			data:     []uint64{uint64(tagRoot) << 56},
			wantDoc:  false,
			wantType: Type(-1),
		},
		{
			name:     "empty",
			data:     []uint64{},
			wantDoc:  false,
			wantType: Type(-1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tape := &Tape{data: tt.data}
			pj := &ParsedJson{tape: *tape, hasTape: true, copyStrings: true}

			if got := tape.hasRootDoc(); got != tt.wantDoc {
				t.Fatalf("hasRootDoc() = %v, want %v", got, tt.wantDoc)
			}

			if got := tape.RootType(); got != tt.wantType {
				t.Errorf("RootType() = %v, want %v", got, tt.wantType)
			}

			iv, iErr := tape.Interface()
			_, nErr := tape.InterfaceUseNumber()
			calls := 0
			fErr := pj.ForEach(func(i Iter) error { calls++; return nil })

			if tt.wantDoc {
				if iErr != nil {
					t.Errorf("Interface() = %v, want nil", iErr)
				} else if iv != tt.wantVal {
					t.Errorf("Interface() = %#v, want %#v", iv, tt.wantVal)
				}
				if nErr != nil {
					t.Errorf("InterfaceUseNumber() = %v, want nil", nErr)
				}
				if fErr != nil || calls != 1 {
					t.Errorf("ForEach: %d calls, err %v; want 1 call, nil", calls, fErr)
				}
				return
			}

			// No document: all four must refuse, and ForEach must not report
			// success with nothing done.
			if iErr == nil {
				t.Error("Interface() accepted a tape with no root document")
			}
			if nErr == nil {
				t.Error("InterfaceUseNumber() accepted a tape with no root document")
			}
			if fErr == nil {
				t.Error("ForEach returned nil for a tape with no root document")
			}
			if calls != 0 {
				t.Errorf("ForEach called fn %d times for a tape with no root document", calls)
			}
		})
	}
}

// TestTapeObjectFindKeyIgnoresUndecodableKey covers the failed-decode path. FindKey
// discarded readString's error, so an undecodable key decoded to "" and then
// matched a lookup for the empty key, handing back an unrelated value. The fix must
// not also skip a key that is genuinely the empty string.
func TestTapeObjectFindKeyIgnoresUndecodableKey(t *testing.T) {
	// keyOffset lands in data[2]; strs backs the string buffer.
	build := func(keyOffset uint64, strs []byte) TapeObject {
		tp := &Tape{
			data: []uint64{
				uint64(tagRoot) << 56,
				uint64(tagObject)<<56 | 5, // '{' at 1, endIdx 5
				uint64(tagString)<<56 | keyOffset,
				uint64(tagTrue) << 56,   // value at 3
				uint64(tagObjEnd) << 56, // '}' at 4
			},
			strings: strs,
		}
		return TapeObject{tape: tp, startIdx: 2, endIdx: 4}
	}

	t.Run("undecodable key never matches", func(t *testing.T) {
		obj := build(999, []byte{}) // offset past an empty buffer
		if ti, ok := obj.FindKey(""); ok {
			got, _ := ti.Bool()
			t.Errorf("FindKey(\"\") matched an undecodable key and returned %v", got)
		}
	})

	t.Run("genuinely empty key still matches", func(t *testing.T) {
		obj := build(0, []byte{0, 0, 0, 0}) // uint32 length 0 => ("", nil)
		ti, ok := obj.FindKey("")
		if !ok {
			t.Fatal("FindKey(\"\") missed a key that is genuinely the empty string")
		}
		if got, err := ti.Bool(); err != nil || !got {
			t.Errorf("value = %v, %v; want true, nil", got, err)
		}
	})
}

// TestTapeIterObjectArrayPastEnd covers the guards on Object and Array. Without
// them a past-end iterator fell through to the tag check and reported the wrong
// reason ("element is not an object" rather than being out of range).
func TestTapeIterObjectArrayPastEnd(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	tape, err := pj.GetTape()
	if err != nil {
		t.Fatalf("GetTape: %v", err)
	}

	for _, idx := range []int{len(tape.data), len(tape.data) + 10, -1} {
		ti := TapeIter{tape: tape, idx: idx}
		if _, err := ti.Object(); err == nil {
			t.Errorf("idx %d: Object() returned no error past end of tape", idx)
		} else if !strings.Contains(err.Error(), "past end") {
			t.Errorf("idx %d: Object() = %q, want a past-end error", idx, err)
		}
		if _, err := ti.Array(); err == nil {
			t.Errorf("idx %d: Array() returned no error past end of tape", idx)
		} else if !strings.Contains(err.Error(), "past end") {
			t.Errorf("idx %d: Array() = %q, want a past-end error", idx, err)
		}
	}
}

// TestTapeLookupsZeroValueOnMiss pins the by-value not-found contract: a failed
// lookup must yield a zero TapeIter, not a half-populated one that reads the tape.
func TestTapeLookupsZeroValueOnMiss(t *testing.T) {
	pj, err := Parse([]byte(`{"Image":{"Width":800},"n":1}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()
	tape, _ := pj.GetTape()
	root := tape.Iter()
	obj, err := root.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	var zero TapeIter
	for _, tc := range []struct {
		name string
		got  func() (TapeIter, bool)
	}{
		{"FindKey", func() (TapeIter, bool) { return obj.FindKey("nope") }},
		{"FindPath missing leaf", func() (TapeIter, bool) { return obj.FindPath("Image", "nope") }},
		{"FindPath non-object intermediate", func() (TapeIter, bool) { return obj.FindPath("n", "Width") }},
		{"FindPath empty", func() (TapeIter, bool) { return obj.FindPath() }},
		{"FindElement", func() (TapeIter, bool) { return root.FindElement("nope") }},
		{"FindElement empty", func() (TapeIter, bool) { return root.FindElement() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ti, ok := tc.got()
			if ok {
				t.Fatal("lookup unexpectedly succeeded")
			}
			if ti != zero {
				t.Errorf("not-found returned %+v, want the zero TapeIter", ti)
			}
		})
	}
}

// TestValidateAcceptsEveryRealTape is the false-rejection guard for the trust
// boundary. validate() gates Serializer.Deserialize, and every walker indexes the
// tape on the assumption it passed, so a tape this package produced must never be
// rejected. Rejecting a valid tape is a worse failure than the panics validate exists
// to prevent, so this covers parsed files, literals, NDJSON multi-block tapes, and
// tapes carrying NOP runs left by mutation.
func TestValidateAcceptsEveryRealTape(t *testing.T) {
	t.Run("parsed files", func(t *testing.T) {
		for _, f := range benchmarkFiles {
			data := loadTestFile(t, f)
			pj, err := Parse(data, nil)
			if err != nil {
				t.Fatalf("%s: Parse: %v", f, err)
			}
			if err := pj.tape.Validate(); err != nil {
				t.Errorf("%s: rejected a parsed tape: %v", f, err)
			}
			pj.Close()
		}
	})

	t.Run("literals and edge shapes", func(t *testing.T) {
		for _, js := range []string{
			`{}`, `[]`, `null`, `true`, `false`, `0`, `-1`, `1.5e10`, `""`, `"x"`,
			`{"a":1}`, `[[[]]]`, `{"a":{"b":{"c":[]}}}`, `[{},{},[]]`,
			`18446744073709551615`, `{"":null}`, `[1,"two",3.0,null,true]`,
		} {
			pj, err := Parse([]byte(js), nil)
			if err != nil {
				t.Fatalf("%s: Parse: %v", js, err)
			}
			if err := pj.tape.Validate(); err != nil {
				t.Errorf("%s: rejected: %v", js, err)
			}
			pj.Close()
		}
	})

	t.Run("NDJSON multi-block tapes", func(t *testing.T) {
		// Several root blocks, each followed by a padding word validate must step over
		// rather than read: simdjson never writes it, so it holds stale memory.
		for _, nd := range []string{"1\n2\n3\n", "{\"a\":1}\n{\"b\":2}\n", "{}\n[]\n{}\n", "[1]\n2\n"} {
			pj, err := ParseND([]byte(nd), nil)
			if err != nil {
				t.Fatalf("%q: ParseND: %v", nd, err)
			}
			if err := pj.tape.Validate(); err != nil {
				t.Errorf("%q: rejected: %v", nd, err)
			}
			pj.Close()
		}
	})

	t.Run("after mutation", func(t *testing.T) {
		// Each mutation must actually change the document, or "validate accepted it"
		// would be true of a no-op.
		for _, tc := range []struct {
			name string
			fn   func(*ParsedJson)
			want string
		}{
			{"DeleteElems on an object", func(pj *ParsedJson) {
				it, _ := pj.Iter()
				o, _ := it.Object(nil)
				_ = o.DeleteElems(func(k []byte, i Iter) bool { return string(k) == "b" }, nil)
			}, `{"a":1,"arr":[1,2,3],"z":"end"}`},
			{"DeleteElems on an array", func(pj *ParsedJson) {
				it, _ := pj.Iter()
				o, _ := it.Object(nil)
				a, _ := o.FindKey("arr", nil).Iter.Array(nil)
				a.DeleteElems(func(i Iter) bool { v, _ := i.Int(); return v == 2 })
			}, `{"a":1,"b":2,"arr":[1,3],"z":"end"}`},
			{"SetNull over a container", func(pj *ParsedJson) {
				it, _ := pj.Iter()
				o, _ := it.Object(nil)
				_ = o.FindKey("arr", nil).Iter.SetNull()
			}, `{"a":1,"b":2,"arr":null,"z":"end"}`},
			{"SetString shrinking a number", func(pj *ParsedJson) {
				it, _ := pj.Iter()
				o, _ := it.Object(nil)
				_ = o.FindKey("b", nil).Iter.SetString("replaced")
			}, `{"a":1,"b":"replaced","arr":[1,2,3],"z":"end"}`},
			{"two separate deletes leaving abutting NOP runs", func(pj *ParsedJson) {
				// validate bounds each NOP's skip by its contiguous run rather than
				// requiring every skip in a run to share one target, because this shape
				// produces runs whose skips point at different ends. Deleting adjacent
				// keys in separate calls is what makes them abut.
				it, _ := pj.Iter()
				o, _ := it.Object(nil)
				_ = o.DeleteElems(func(k []byte, i Iter) bool { return string(k) == "b" }, nil)
				it2, _ := pj.Iter()
				o2, _ := it2.Object(nil)
				_ = o2.DeleteElems(func(k []byte, i Iter) bool { return string(k) == "arr" }, nil)
			}, `{"a":1,"z":"end"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				pj, err := Parse([]byte(`{"a":1,"b":2,"arr":[1,2,3],"z":"end"}`), nil)
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				defer pj.Close()
				tc.fn(pj)
				if err := pj.tape.Validate(); err != nil {
					t.Fatalf("rejected a mutated tape: %v", err)
				}
				after, _ := pj.Iter()
				got, err := after.MarshalJSON()
				if err != nil {
					t.Fatalf("MarshalJSON: %v", err)
				}
				if string(got) != tc.want {
					t.Errorf("mutation produced %s, want %s", got, tc.want)
				}
			})
		}
	})

	t.Run("serializer round trips", func(t *testing.T) {
		for _, js := range []string{`{"a":1,"b":[1,2,{"c":"x"}]}`, `[]`, `{}`, `[[1,2],"after"]`, `1e300`, `{"":null}`} {
			pj, err := Parse([]byte(js), nil)
			if err != nil {
				t.Fatalf("%s: Parse: %v", js, err)
			}
			ser := NewSerializer()
			got, err := ser.Deserialize(ser.Serialize(nil, *pj), nil)
			if err != nil {
				t.Errorf("%s: round trip rejected: %v", js, err)
			} else if _, err := got.tape.Interface(); err != nil {
				t.Errorf("%s: round trip tape unreadable: %v", js, err)
			}
			pj.Close()
		}
	})
}

// TestValidateRejectsCorruptTape covers the structural invariants walkers depend on.
// Each shape below previously reached a walker and panicked, looped, or over-allocated
// because the walkers took these values from the tape without checking them; the
// boundary now rejects them once instead.
func TestValidateRejectsCorruptTape(t *testing.T) {
	// A valid `[1,2,3]`-shaped reference to mutate from is not needed: these are built
	// directly, which is the only way to express a tape Parse cannot produce.
	tests := []struct {
		name string
		data []uint64
		want string
	}{
		{
			"array end index past the tape",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagArray)<<56 | 100000, tapeEntry(tagArrEnd, 1), tapeEntry(tagRoot, 0), 0},
			"invalid end index",
		},
		{
			"object end index past the tape",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagObject)<<56 | 100000, tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0},
			"invalid end index",
		},
		{
			"end index points backwards",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagObject) << 56, tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0},
			"invalid end index",
		},
		{
			"end index is the container itself",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagObject)<<56 | 1, tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0},
			"invalid end index",
		},
		{
			"end index leaves no room for a closing tag",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagObject)<<56 | 2, tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0},
			"invalid end index",
		},
		{
			"nested end index cannot advance a walk",
			[]uint64{tapeEntry(tagRoot, 6), uint64(tagArray)<<56 | 5, uint64(tagArray)<<56 | 2, tapeEntry(tagArrEnd, 2), tapeEntry(tagArrEnd, 1), tapeEntry(tagRoot, 0), 0},
			"invalid end index",
		},
		{
			"container not closed by a matching tag",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagObject)<<56 | 3, tapeEntry(tagArrEnd, 1), tapeEntry(tagRoot, 0), 0},
			"does not close it",
		},
		{
			"array not closed by a matching tag",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagArray)<<56 | 3, tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0},
			"does not close it",
		},
		{
			"element count exceeds the container extent",
			[]uint64{tapeEntry(tagRoot, 4), uint64(tagObject)<<56 | uint64(containerCountMask)<<32 | 3, tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0},
			"exceeds its extent",
		},
		{
			// Count() reports this field and DeleteElems rewrites it, so a count that
			// merely fits inside the extent is not enough: it must match the contents.
			// One pair here ("k":true), declared as none.
			"element count disagrees with the contents",
			[]uint64{
				tapeEntry(tagRoot, 6), uint64(tagObject)<<56 | 0<<32 | 5,
				tapeEntry(tagString, 0), tapeEntry(tagTrue, 0),
				tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"declares 0 elements but holds 1",
		},
		{
			// A numeric in the block's final slot would borrow the closing root marker
			// as its value word.
			"numeric entry has no value word inside its block",
			[]uint64{tapeEntry(tagRoot, 3), tapeEntry(tagInt64, 0), tapeEntry(tagRoot, 0), 0},
			"no value word",
		},
		{
			"no root marker at index 0",
			[]uint64{tapeEntry(tagInt64, 0), 7},
			"expected a root marker",
		},
		{
			"closing root missing",
			[]uint64{tapeEntry(tagRoot, 3), tapeEntry(tagNull, 0), tapeEntry(tagNull, 0), 0},
			"no closing root",
		},
		{
			"closing root does not point back",
			[]uint64{tapeEntry(tagRoot, 3), tapeEntry(tagNull, 0), tapeEntry(tagRoot, 1), 0},
			"does not point back",
		},
		{
			"unknown tag",
			[]uint64{tapeEntry(tagRoot, 3), uint64('Q') << 56, tapeEntry(tagRoot, 0), 0},
			"unknown tag",
		},
		{
			"object holds a bare value with no key",
			[]uint64{
				tapeEntry(tagRoot, 5), uint64(tagObject)<<56 | 4,
				tapeEntry(tagNull, 0), tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"expected a string key",
		},
		{
			"numeric where an object key belongs",
			[]uint64{
				tapeEntry(tagRoot, 7), uint64(tagObject)<<56 | 6,
				tapeEntry(tagInt64, 0), 7, tapeEntry(tagNull, 0),
				tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"expected a string key",
		},
		{
			"container where an object key belongs",
			[]uint64{
				tapeEntry(tagRoot, 7), uint64(tagObject)<<56 | 6,
				uint64(tagArray)<<56 | 5, tapeEntry(tagArrEnd, 2),
				tapeEntry(tagNull, 0), tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"expected a string key",
		},
		{
			// A valid pair first, so the phase must flip back, not just start right.
			"valid pair followed by a stray value",
			[]uint64{
				tapeEntry(tagRoot, 7), uint64(tagObject)<<56 | 1<<32 | 6,
				tapeEntry(tagString, 0), tapeEntry(tagNull, 0), tapeEntry(tagNull, 0),
				tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"expected a string key",
		},
		{
			"object closes with a key that has no value",
			[]uint64{
				tapeEntry(tagRoot, 5), uint64(tagObject)<<56 | 4,
				tapeEntry(tagString, 0), tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"has a key with no value",
		},
		{
			// No object reader skips NOPs between a key and its value, so a NOP in a
			// value slot desyncs them by one entry: ForEach yielded the same key twice,
			// with the NOP and then the closing tag as its value.
			"NOP where an object value belongs",
			[]uint64{
				tapeEntry(tagRoot, 7), uint64(tagObject)<<56 | 1<<32 | 6,
				tapeEntry(tagString, 0), uint64(tagNop)<<56 | 1, tapeEntry(tagNull, 0),
				tapeEntry(tagObjEnd, 1), tapeEntry(tagRoot, 0), 0,
			},
			"NOP where a value belongs",
		},
		{
			// The readers jump to a container's end index rather than walking tags in
			// order, so validate must not honour a NOP's skip: a hostile skip would
			// hide entries a reader still reaches. Here the NOP at 3 claims skip 5,
			// which would carry a skip-following cursor from 3 straight to 8, leaving
			// the self-referential container at 6 uninspected — and a reader following
			// the container at 2 to its end index lands exactly on it, where
			// skipValue returns 6 forever.
			"NOP skip hides a self-referential container",
			[]uint64{
				tapeEntry(tagRoot, 9),    // 0
				uint64(tagArray)<<56 | 8, // 1
				uint64(tagArray)<<56 | 6, // 2
				uint64(tagNop)<<56 | 5,   // 3  skip jumps 3 -> 8
				0,                        // 4  hidden
				tapeEntry(tagArrEnd, 0),  // 5
				uint64(tagArray)<<56 | 6, // 6  hidden, end index == itself
				tapeEntry(tagArrEnd, 0),  // 7
				tapeEntry(tagRoot, 0),    // 8
			},
			"leaves its run",
		},
		{
			// The mirror of the case above. Readers honour a NOP's skip, so a skip that
			// lands on a numeric's VALUE word hands them 64 bits of tape data to decode
			// as an entry — bits the walk consumes opaquely and never tag-checks. Here
			// the smuggled container at 4 has an end index equal to its own position,
			// so a reader steered onto it never advances.
			"NOP skip lands a reader on a numeric value word",
			[]uint64{
				tapeEntry(tagRoot, 7),            // 0
				uint64(tagArray)<<56 | 1<<32 | 6, // 1 '[' end 6, count 1
				uint64(tagNop)<<56 | 2,           // 2 NOP skip 2 -> reader jumps to 4
				tapeEntry(tagInt64, 0),           // 3 numeric; 4 is its value word
				uint64(tagArray)<<56 | 4,         // 4 smuggled container, end == self
				tapeEntry(tagArrEnd, 0),          // 5
				tapeEntry(tagRoot, 0),            // 6
				0,                                // 7
			},
			"leaves its run",
		},
		{
			// A child must close before its parent: the inner container claims the
			// parent's own closing tag as its end.
			"container end index escapes its parent",
			[]uint64{
				tapeEntry(tagRoot, 6),    // 0, closing root at 5
				uint64(tagArray)<<56 | 5, // 1, closes at 4
				uint64(tagArray)<<56 | 5, // 2, claims the parent's close
				tapeEntry(tagArrEnd, 0),  // 3
				tapeEntry(tagArrEnd, 0),  // 4
				tapeEntry(tagRoot, 0),    // 5
				0,                        // 6
			},
			"escapes its parent",
		},
		{
			// The closing tag must sit exactly where the end index points, not merely
			// be a closing tag of the right kind somewhere inside.
			"closing tag is not where the end index points",
			[]uint64{
				tapeEntry(tagRoot, 7),    // 0, closing root at 6
				uint64(tagArray)<<56 | 6, // 1, end index says its close is at 5
				tapeEntry(tagNull, 0),    // 2
				tapeEntry(tagArrEnd, 0),  // 3, but a close appears here
				tapeEntry(tagNull, 0),    // 4
				tapeEntry(tagArrEnd, 0),  // 5
				tapeEntry(tagRoot, 0),    // 6
				0,                        // 7
			},
			"not where end index",
		},
		{
			"closing tag with no open container",
			[]uint64{tapeEntry(tagRoot, 3), tapeEntry(tagArrEnd, 0), tapeEntry(tagRoot, 0), 0},
			"closing tag with no open container",
		},
		{
			// A numeric as its container's last entry would take the closing tag as
			// its value word.
			"numeric borrows a closing tag as its value",
			[]uint64{
				tapeEntry(tagRoot, 5),
				uint64(tagArray)<<56 | 4,
				tapeEntry(tagInt64, 0),
				tapeEntry(tagArrEnd, 0),
				tapeEntry(tagRoot, 0),
				0,
			},
			"no value word",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := &Tape{data: tt.data}
			err := tp.Validate()
			if err == nil {
				t.Fatal("accepted a corrupt tape")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

// TestValidateRejectsExcessiveNesting covers the one invariant the recursive tape
// readers assume that structural checks alone do not give. readValue, readObject,
// readArray and marshalTape recurse once per level, and a Go stack overflow is fatal —
// recover cannot catch it. A ~32MB deserialized tape nested 2,000,000 deep crashed the
// process outright, so validate bounds depth at what simdjson itself can emit.
func TestValidateRejectsExcessiveNesting(t *testing.T) {
	// nested builds `[[[...1...]]]` directly:
	//   0: open root | 1..depth: opens | depth+1,depth+2: int64 + value word
	//   depth+3..2*depth+2: closes | 2*depth+3: close root | 2*depth+4: padding
	nested := func(depth int) *Tape {
		closeIdx := 2*depth + 3
		d := make([]uint64, 2*depth+5)
		d[0] = tapeEntry(tagRoot, uint64(closeIdx+1))
		for p := 1; p <= depth; p++ {
			// Each level holds exactly one element: the next array, or the int.
			d[p] = uint64(tagArray)<<56 | 1<<32 | uint64(2*depth+4-p)
		}
		d[depth+1] = tapeEntry(tagInt64, 0)
		d[depth+2] = 7
		for k := 1; k <= depth; k++ {
			d[depth+2+k] = tapeEntry(tagArrEnd, uint64(depth+1-k))
		}
		d[closeIdx] = tapeEntry(tagRoot, 0)
		return &Tape{data: d}
	}

	// simdjson refuses to parse deeper than maxTapeDepth, so anything at or under it
	// must still be accepted, and the readers must handle it.
	for _, depth := range []int{1, 2, 64, maxTapeDepth - 1, maxTapeDepth} {
		tp := nested(depth)
		if err := tp.Validate(); err != nil {
			t.Fatalf("depth %d rejected: %v", depth, err)
		}
		if _, err := tp.Interface(); err != nil {
			t.Errorf("depth %d: Interface: %v", depth, err)
		}
	}

	for _, depth := range []int{maxTapeDepth + 1, maxTapeDepth * 4} {
		err := nested(depth).Validate()
		if err == nil {
			t.Errorf("depth %d accepted", depth)
		} else if !strings.Contains(err.Error(), "nesting deeper than") {
			t.Errorf("depth %d: error = %q, want a nesting-depth error", depth, err)
		}
	}
}

// TestStringOffsetsAreGuardedAtReadTime documents the one invariant validate()
// deliberately does not establish. readStringBytes is the only path to the string
// buffer and must read the length prefix to slice at all, so bounds-checking there is
// intrinsic; repeating it at the boundary cost 45% of the validation walk for no
// safety gain. A bad offset must therefore surface as a read error, not a panic.
func TestStringOffsetsAreGuardedAtReadTime(t *testing.T) {
	for _, tt := range []struct {
		name    string
		strings []byte
		payload uint64
		want    string
	}{
		{"offset past the buffer", make([]byte, 8), 500, "offset 500 out of bounds"},
		{"length prefix overruns the buffer", func() []byte {
			b := make([]byte, 8)
			binary.NativeEndian.PutUint32(b[0:4], 9999)
			return b
		}(), 0, "length 9999"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tp := &Tape{
				data:    []uint64{tapeEntry(tagRoot, 3), tapeEntry(tagString, tt.payload), tapeEntry(tagRoot, 0), 0},
				strings: tt.strings,
			}
			if err := tp.Validate(); err != nil {
				t.Errorf("validate should not inspect string offsets, got %v", err)
			}
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("reading the string panicked instead of erroring: %v", r)
				}
			}()
			ti := tp.Iter()
			if _, err := ti.String(); err == nil {
				t.Error("expected a read error for an out-of-bounds string")
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

// Validate's contract is that a tape it accepts can be walked by every reader without
// panicking or failing to terminate. The readers hold no bounds checks of their own, so
// these two tests are what keeps that contract honest.
//
// Both drive the real trust boundary: a candidate tape is serialized and handed to
// Deserialize, which calls Validate. Only tapes it accepts get walked. A panic fails the
// test directly; non-termination shows up as budget exhaustion, and a loop inside a
// reader that the budget cannot reach shows up as the ambient go test timeout.

// contractWalker bounds the walk. Real documents measure at most 1 step per tape word on
// either layer, so the 16x budget is slack for correct readers and a trip wire otherwise.
type contractWalker struct {
	t    *testing.T
	left int
}

func contractBudget(n int) int { return 16*n + 1024 }

func (w *contractWalker) step() {
	w.left--
	if w.left <= 0 {
		w.t.Fatal("walk budget exhausted: a reader is not making progress")
	}
}

// contractWalkTape covers the tape layer, including the four readers that used to check
// for a string key themselves: TapeObject.FindKey, TapeObject.ForEach, readObject (via
// Interface) and readObjectNum (via InterfaceUseNumber, the only route to it).
func contractWalkTape(t *testing.T, tp *Tape) {
	w := &contractWalker{t: t, left: contractBudget(len(tp.data))}
	_, _ = tp.Interface()
	_, _ = tp.InterfaceUseNumber()
	_ = tp.RootType()
	_ = tp.Clone()
	ti := tp.Iter()
	for ti.Type() != Type(-1) {
		w.step()
		contractWalkValue(ti, w, 0)
		if ti.Advance() == Type(-1) {
			return
		}
	}
}

// contractWalkValue inspects one value and recurses into its children, never advancing to
// a sibling, which keeps the walk linear in the tape length.
func contractWalkValue(ti TapeIter, w *contractWalker, depth int) {
	if depth > maxTapeDepth {
		return
	}
	w.step()
	switch ti.Type() {
	case TypeObject:
		o, err := ti.Object()
		if err != nil {
			return
		}
		_, _ = o.FindKey("a")
		_, _ = o.FindPath("a", "b")
		_ = o.Count()
		_, _ = o.Map(nil)
		_ = o.ForEach(func(key string, val TapeIter) error {
			_ = key
			contractWalkValue(val, w, depth+1)
			return nil
		})
	case TypeArray:
		a, err := ti.Array()
		if err != nil {
			return
		}
		_ = a.Count()
		_, _ = a.Interface()
		_, _ = a.AsFloat()
		_, _ = a.AsInteger()
		_, _ = a.AsString()
		_ = a.ForEach(func(val TapeIter) error {
			contractWalkValue(val, w, depth+1)
			return nil
		})
	default:
		_, _ = ti.String()
		_, _ = ti.Int()
		_, _ = ti.Float()
		_, _ = ti.Interface()
	}
}

// contractWalkParsed covers the Iter layer, including the three remaining readers that
// used to check for a string key: NextElementBytes, DeleteElems and marshalTape.
func contractWalkParsed(t *testing.T, pj *ParsedJson) {
	w := &contractWalker{t: t, left: contractBudget(len(pj.tape.data))}
	_ = pj.RootType()
	_, _ = pj.FindString("a")
	_ = pj.ForEach(func(i Iter) error {
		w.step()
		_, _ = i.Interface()
		return nil
	})
	it, err := pj.Iter()
	if err != nil {
		return
	}
	var root Iter
	if _, r, err := it.Root(&root); err == nil && r != nil {
		_, _ = r.MarshalJSON()
		_, _ = r.Interface()
		_, _ = r.FindElement(nil, "a", "b")
	}
	for it.Type() != Type(-1) {
		w.step()
		contractWalkIterValue(&it, w, 0)
		if it.Advance() == Type(-1) {
			break
		}
	}
	contractWalkDeletes(t, pj)
}

func contractWalkIterValue(it *Iter, w *contractWalker, depth int) {
	if it == nil || depth > maxTapeDepth {
		return
	}
	w.step()
	_, _ = it.MarshalJSON()
	_, _ = it.Interface()
	switch it.Type() {
	case TypeObject:
		o, err := it.Object(nil)
		if err != nil {
			return
		}
		_, _ = o.Count()
		_, _ = o.Parse(nil)
		_, _ = o.Map(nil)
		_ = o.ForEach(func(key string, v Iter) error {
			_ = key
			contractWalkIterValue(&v, w, depth+1)
			return nil
		})
		// A fresh Object: ForEach consumed the one above. The element count bounds the
		// loop independently of the terminator.
		if o2, err := it.Object(nil); err == nil {
			n, _ := o2.Count()
			var dst Iter
			for k := 0; k <= n+1; k++ {
				w.step()
				_, typ, err := o2.NextElementBytes(&dst)
				if err != nil || typ == Type(-1) {
					break
				}
			}
		}
	case TypeArray:
		a, err := it.Array(nil)
		if err != nil {
			return
		}
		_, _ = a.Count()
		_, _ = a.MarshalJSON()
		_, _ = a.Interface()
		_ = a.ForEach(func(v Iter) error {
			contractWalkIterValue(&v, w, depth+1)
			return nil
		})
	default:
		_, _ = it.String()
		_, _ = it.Int()
		_, _ = it.Float()
		_, _ = it.Bool()
	}
}

// contractWalkDeletes runs both delete paths on clones, after the read walk rather than
// during it, and requires the result to still satisfy the boundary.
func contractWalkDeletes(t *testing.T, pj *ParsedJson) {
	for _, del := range []func(*ParsedJson){
		func(p *ParsedJson) {
			if it, err := p.Iter(); err == nil {
				if o, err := it.Object(nil); err == nil {
					_ = o.DeleteElems(func(key []byte, i Iter) bool { return true }, nil)
				}
			}
		},
		func(p *ParsedJson) {
			if it, err := p.Iter(); err == nil {
				if a, err := it.Array(nil); err == nil {
					a.DeleteElems(func(i Iter) bool { return true })
				}
			}
		},
	} {
		clone := pj.Clone(nil)
		del(clone)
		if err := clone.tape.Validate(); err != nil {
			t.Fatalf("a delete produced a tape that fails the boundary: %v", err)
		}
	}
}

// contractCheck runs candidate tape words through Deserialize and, if accepted, walks
// every reader. Reports whether the tape was accepted.
func contractCheck(t *testing.T, data []uint64, strs []byte) bool {
	ser := NewSerializer()
	in := ParsedJson{tape: Tape{data: data, strings: strs}, hasTape: true}
	got, err := ser.Deserialize(ser.Serialize(nil, in), nil)
	if err != nil {
		return false
	}
	contractWalkTape(t, &got.tape)
	contractWalkParsed(t, got)
	return true
}

var contractSeeds = []string{
	`{}`, `[]`, `{"":null}`, `{"a":1}`, `[1,2,3]`,
	`{"a":1,"b":"x","c":[1,2,{"d":null}],"e":{"f":true}}`,
	`[{"k":1},{"k":2},[[1]],"s",1.5,18446744073709551615]`,
	`{"n":{"n":{"n":{"n":[1,{"z":false}]}}}}`,
}

// TestTapeContractUnderMutation mutates real tapes so plain go test exercises the
// contract, not just the fuzz seed corpus.
//
// Most mutations are width-preserving retags. A mutation that changes an entry's width
// desynchronises the container end indices and is rejected on framing alone, so it never
// reaches the key/value rules.
func TestTapeContractUnderMutation(t *testing.T) {
	// One-word tags, so a retag leaves every container's extent intact.
	oneWord := []byte{tagString, tagNull, tagTrue, tagFalse, tagNop, tagBigint}
	anyTag := []byte{
		tagRoot, tagString, tagInt64, tagUint64, tagDouble, tagNull, tagTrue, tagFalse,
		tagObject, tagObjEnd, tagArray, tagArrEnd, tagNop, tagBigint, 'Q',
	}
	rng := rand.New(rand.NewSource(0xC0FFEE))
	accepted := 0

	for _, seed := range contractSeeds {
		pj, err := Parse([]byte(seed), nil)
		if err != nil {
			t.Fatalf("Parse %q: %v", seed, err)
		}
		base, strs := pj.tape.data, pj.tape.strings

		for i := 0; i < 1500; i++ {
			data := make([]uint64, len(base))
			copy(data, base)
			for m := 0; m < 1+rng.Intn(3); m++ {
				j := rng.Intn(len(data))
				switch rng.Intn(6) {
				case 0, 1, 2: // width-preserving retag, keeps the framing valid
					data[j] = uint64(oneWord[rng.Intn(len(oneWord))])<<56 | (data[j] & payloadMask)
				case 3:
					data[j] = uint64(anyTag[rng.Intn(len(anyTag))])<<56 | (data[j] & payloadMask)
				case 4:
					data[j] = (data[j] & ^uint64(payloadMask)) | uint64(rng.Intn(len(data)+4))
				case 5:
					k := rng.Intn(len(data))
					data[j], data[k] = data[k], data[j]
				}
			}
			if contractCheck(t, data, strs) {
				accepted++
			}
		}
	}
	// Guards against the mutation scheme drifting into producing only rejects, which
	// would leave the readers unexercised and the test silently vacuous.
	if accepted < 100 {
		t.Fatalf("only %d mutants were accepted; the readers are barely exercised", accepted)
	}
	t.Logf("%d accepted mutants walked", accepted)
}

// FuzzTapeContract explores the contract without a bound on shapes.
//
// The input drives mutations against a real parsed tape rather than supplying tape words
// directly. Raw words almost always break a container's framing and get rejected before
// the key/value rules run — measured: 25M raw-word executions failed to place a NOP in an
// object's value slot, a shape the structured form reaches in seconds.
//
// Input layout: byte 0 selects the seed document, then each following 4-byte group is one
// mutation as (op, index low, index high, argument).
func FuzzTapeContract(f *testing.F) {
	seeds := make([][]uint64, 0, len(contractSeeds))
	strs := make([][]byte, 0, len(contractSeeds))
	for _, seed := range contractSeeds {
		pj, err := Parse([]byte(seed), nil)
		if err != nil {
			f.Fatalf("Parse %q: %v", seed, err)
		}
		seeds = append(seeds, pj.tape.data)
		strs = append(strs, pj.tape.strings)
	}

	oneWord := []byte{tagString, tagNull, tagTrue, tagFalse, tagNop, tagBigint}
	anyTag := []byte{
		tagRoot, tagString, tagInt64, tagUint64, tagDouble, tagNull, tagTrue, tagFalse,
		tagObject, tagObjEnd, tagArray, tagArrEnd, tagNop, tagBigint, 'Q',
	}

	// Seed the corpus with one mutation per document, including a NOP retag.
	for i := range contractSeeds {
		f.Add([]byte{byte(i), 0, 2, 0, 4})
		f.Add([]byte{byte(i), 1, 3, 0, 1})
		f.Add([]byte{byte(i), 3, 2, 0, 9})
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) < 5 || len(raw) > 4096 {
			return
		}
		which := int(raw[0]) % len(seeds)
		base := seeds[which]
		data := make([]uint64, len(base))
		copy(data, base)

		for p := 1; p+3 < len(raw); p += 4 {
			idx := (int(raw[p+1]) | int(raw[p+2])<<8) % len(data)
			arg := int(raw[p+3])
			switch raw[p] % 5 {
			case 0, 1: // width-preserving retag: keeps container extents intact
				data[idx] = uint64(oneWord[arg%len(oneWord)])<<56 | (data[idx] & payloadMask)
			case 2:
				data[idx] = uint64(anyTag[arg%len(anyTag)])<<56 | (data[idx] & payloadMask)
			case 3:
				data[idx] = (data[idx] & ^uint64(payloadMask)) | uint64(arg%(len(data)+4))
			case 4:
				other := arg % len(data)
				data[idx], data[other] = data[other], data[idx]
			}
		}
		contractCheck(t, data, strs[which])
	})
}
