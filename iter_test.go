package simdjson

import (
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"
)

func TestIterType(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}
	if iter.Type() != TypeObject {
		t.Fatalf("expected object, got %v", iter.Type())
	}
}

func TestIterObject(t *testing.T) {
	pj, err := Parse([]byte(`{"name":"simdjson","version":"4.0.1","count":42}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}

	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}

	// FindKey string
	elem := obj.FindKey("name", nil)
	if elem == nil {
		t.Fatal("FindKey('name') returned nil")
	}
	val, err := elem.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if val != "simdjson" {
		t.Fatalf("expected 'simdjson', got %q", val)
	}

	// FindKey missing
	if obj.FindKey("missing", nil) != nil {
		t.Fatal("expected nil for missing key")
	}

	// Count
	n, err := obj.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3 keys, got %d", n)
	}
}

func TestIterValues(t *testing.T) {
	pj, err := Parse([]byte(`{"s":"hello","i":-42,"u":99,"f":3.14,"b":true}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	t.Run("string", func(t *testing.T) {
		e := obj.FindKey("s", nil)
		v, err := e.Iter.String()
		if err != nil || v != "hello" {
			t.Fatalf("got %q, err=%v", v, err)
		}
	})

	t.Run("int64", func(t *testing.T) {
		e := obj.FindKey("i", nil)
		v, err := e.Iter.Int()
		if err != nil || v != -42 {
			t.Fatalf("got %d, err=%v", v, err)
		}
	})

	t.Run("uint64", func(t *testing.T) {
		e := obj.FindKey("u", nil)
		v, err := e.Iter.Uint()
		if err != nil || v != 99 {
			t.Fatalf("got %d, err=%v", v, err)
		}
	})

	t.Run("float64", func(t *testing.T) {
		e := obj.FindKey("f", nil)
		v, err := e.Iter.Float()
		if err != nil || v != 3.14 {
			t.Fatalf("got %f, err=%v", v, err)
		}
	})

	t.Run("bool", func(t *testing.T) {
		e := obj.FindKey("b", nil)
		v, err := e.Iter.Bool()
		if err != nil || !v {
			t.Fatalf("got %v, err=%v", v, err)
		}
	})
}

func TestObjectIteration(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1,"b":"two","c":true}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	keys := make([]string, 0)
	err = obj.ForEach(func(key string, i Iter) error {
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"a", "b", "c"}
	for i, k := range expected {
		if keys[i] != k {
			t.Fatalf("key[%d]: expected %q, got %q", i, k, keys[i])
		}
	}
}

func TestArrayIteration(t *testing.T) {
	pj, err := Parse([]byte(`[10,20,30]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, err := iter.Array(nil)
	if err != nil {
		t.Fatal(err)
	}

	n, _ := arr.Count()
	if n != 3 {
		t.Fatalf("expected 3 elements, got %d", n)
	}

	var vals []int64
	err = arr.ForEach(func(i Iter) error {
		v, err := i.Int()
		if err != nil {
			return err
		}
		vals = append(vals, v)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 || vals[0] != 10 || vals[1] != 20 || vals[2] != 30 {
		t.Fatalf("expected [10,20,30], got %v", vals)
	}
}

func TestArrayOnNonArray(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	_, err = iter.Array(nil)
	if err == nil {
		t.Fatal("expected error calling Array() on object")
	}
}

func TestIterInterface(t *testing.T) {
	input := `{"name":"test","count":42,"tags":["a","b"],"nested":{"x":true,"y":null},"pi":3.14}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	v, err := iter.Interface()
	if err != nil {
		t.Fatal(err)
	}

	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "test" {
		t.Fatalf("name: expected 'test', got %v", m["name"])
	}
	if m["count"] != int64(42) {
		t.Fatalf("count: expected 42, got %v (%T)", m["count"], m["count"])
	}
	tags, ok := m["tags"].([]interface{})
	if !ok || len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Fatalf("tags: expected [a,b], got %v", m["tags"])
	}
	nested, ok := m["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested: expected map, got %T", m["nested"])
	}
	if nested["x"] != true {
		t.Fatalf("nested.x: expected true, got %v", nested["x"])
	}
	if nested["y"] != nil {
		t.Fatalf("nested.y: expected nil, got %v", nested["y"])
	}
	if m["pi"] != 3.14 {
		t.Fatalf("pi: expected 3.14, got %v", m["pi"])
	}
}

func TestIterInterfaceArray(t *testing.T) {
	pj, err := Parse([]byte(`[1,"two",true,null,[3.14]]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	v, err := iter.Interface()
	if err != nil {
		t.Fatal(err)
	}

	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", v)
	}
	if len(arr) != 5 {
		t.Fatalf("expected 5 elements, got %d", len(arr))
	}
	if arr[0] != int64(1) {
		t.Fatalf("[0]: expected int64(1), got %v (%T)", arr[0], arr[0])
	}
	if arr[1] != "two" {
		t.Fatalf("[1]: expected 'two', got %v", arr[1])
	}
	if arr[2] != true {
		t.Fatalf("[2]: expected true, got %v", arr[2])
	}
	if arr[3] != nil {
		t.Fatalf("[3]: expected nil, got %v", arr[3])
	}
	inner, ok := arr[4].([]interface{})
	if !ok || len(inner) != 1 || inner[0] != 3.14 {
		t.Fatalf("[4]: expected [3.14], got %v", arr[4])
	}
}

func TestObjectMap(t *testing.T) {
	pj, err := Parse([]byte(demo_json), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	m, err := obj.Map(nil)
	if err != nil {
		t.Fatal(err)
	}

	img, ok := m["Image"].(map[string]interface{})
	if !ok {
		t.Fatalf("Image: expected map, got %T", m["Image"])
	}
	if img["Width"] != int64(800) {
		t.Fatalf("Width: expected 800, got %v", img["Width"])
	}
	if img["Animated"] != false {
		t.Fatalf("Animated: expected false, got %v", img["Animated"])
	}
	ids, ok := img["IDs"].([]interface{})
	if !ok || len(ids) != 4 {
		t.Fatalf("IDs: expected 4-element array, got %v", img["IDs"])
	}
}

func TestIterObjectOnNonObject(t *testing.T) {
	pj, err := Parse([]byte(`[1,2,3]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	_, err = iter.Object(nil)
	if err == nil {
		t.Fatal("expected error calling Object() on array")
	}
}

func TestStringCvt(t *testing.T) {
	pj, err := Parse([]byte(`{"s":"hello","i":42,"f":3.14,"b":true,"n":null}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	tests := []struct {
		key  string
		want string
	}{
		{"s", "hello"},
		{"i", "42"},
		{"f", "3.14"},
		{"b", "true"},
		{"n", "null"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			e := obj.FindKey(tt.key, nil)
			got, err := e.Iter.StringCvt()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestUseNumber(t *testing.T) {
	pj, err := Parse([]byte(`{"i":42,"f":3.14,"big":9223372036854775807}`), nil, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	v, err := iter.Interface()
	if err != nil {
		t.Fatal(err)
	}

	m := v.(map[string]interface{})

	if n, ok := m["i"].(json.Number); !ok {
		t.Fatalf("i: expected json.Number, got %T", m["i"])
	} else if n.String() != "42" {
		t.Fatalf("i: expected '42', got %q", n.String())
	}

	if n, ok := m["f"].(json.Number); !ok {
		t.Fatalf("f: expected json.Number, got %T", m["f"])
	} else if n.String() != "3.14" {
		t.Fatalf("f: expected '3.14', got %q", n.String())
	}

	if n, ok := m["big"].(json.Number); !ok {
		t.Fatalf("big: expected json.Number, got %T", m["big"])
	} else if n.String() != "9223372036854775807" {
		t.Fatalf("big: expected '9223372036854775807', got %q", n.String())
	}
}

func TestStringBytes(t *testing.T) {
	pj, err := Parse([]byte(`"hello"`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()
	iter, _ := pj.Iter()
	b, err := iter.StringBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("expected 'hello', got %q", b)
	}
}

func TestFindPath(t *testing.T) {
	pj, err := Parse([]byte(demo_json), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	t.Run("nested", func(t *testing.T) {
		elem, err := obj.FindPath(nil, "Image", "Thumbnail", "Url")
		if err != nil {
			t.Fatal(err)
		}
		v, _ := elem.Iter.String()
		if v != "http://www.example.com/image/481989943" {
			t.Fatalf("expected URL, got %q", v)
		}
	})

	t.Run("single", func(t *testing.T) {
		elem, err := obj.FindPath(nil, "Image")
		if err != nil {
			t.Fatal(err)
		}
		if elem.Iter.Type() != TypeObject {
			t.Fatalf("expected object, got %v", elem.Iter.Type())
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := obj.FindPath(nil, "Image", "Missing")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestFindElement(t *testing.T) {
	pj, err := Parse([]byte(demo_json), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	elem, err := iter.FindElement(nil, "Image", "Width")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := elem.Iter.Int()
	if v != 800 {
		t.Fatalf("expected 800, got %d", v)
	}
}

func TestNextElement(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1,"b":"two","c":true}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	var dst Iter
	name, typ, err := obj.NextElement(&dst)
	if err != nil || name != "a" || typ != TypeInt64 {
		t.Fatalf("first: name=%q type=%v err=%v", name, typ, err)
	}

	name, typ, err = obj.NextElement(&dst)
	if err != nil || name != "b" || typ != TypeString {
		t.Fatalf("second: name=%q type=%v err=%v", name, typ, err)
	}

	name, typ, err = obj.NextElement(&dst)
	if err != nil || name != "c" || typ != TypeBool {
		t.Fatalf("third: name=%q type=%v err=%v", name, typ, err)
	}

	// Done
	name, _, err = obj.NextElement(&dst)
	if err != nil || name != "" {
		t.Fatalf("expected done, got name=%q err=%v", name, err)
	}
}

func TestArrayInterface(t *testing.T) {
	pj, err := Parse([]byte(`[1,"two",true]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	v, err := arr.Interface()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != int64(1) || v[1] != "two" || v[2] != true {
		t.Fatalf("expected [1,two,true], got %v", v)
	}
}

func TestArrayAsFloat(t *testing.T) {
	pj, err := Parse([]byte(`[1.1, 2.2, 3.3]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	v, err := arr.AsFloat()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != 1.1 || v[1] != 2.2 || v[2] != 3.3 {
		t.Fatalf("expected [1.1,2.2,3.3], got %v", v)
	}
}

func TestArrayAsInteger(t *testing.T) {
	pj, err := Parse([]byte(`[10, -20, 30]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	v, err := arr.AsInteger()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != 10 || v[1] != -20 || v[2] != 30 {
		t.Fatalf("expected [10,-20,30], got %v", v)
	}
}

func TestArrayAsUint64(t *testing.T) {
	pj, err := Parse([]byte(`[1, 2, 18446744073709551615]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	v, err := arr.AsUint64()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != 1 || v[1] != 2 || v[2] != 18446744073709551615 {
		t.Fatalf("got %v", v)
	}
}

func TestArrayAsString(t *testing.T) {
	pj, err := Parse([]byte(`["a", "b", "c"]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	v, err := arr.AsString()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != "a" || v[1] != "b" || v[2] != "c" {
		t.Fatalf("expected [a,b,c], got %v", v)
	}
}

func TestArrayAsStringCvt(t *testing.T) {
	pj, err := Parse([]byte(`[1, "two", true, null]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	v, err := arr.AsStringCvt()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 4 || v[0] != "1" || v[1] != "two" || v[2] != "true" || v[3] != "null" {
		t.Fatalf("expected [1,two,true,null], got %v", v)
	}
}

// --- #9: string byte slices are cap-bounded and honour copyStrings ---

func TestIterStringBytesIsCapBounded(t *testing.T) {
	pj, err := Parse([]byte(`{"a":"hello","b":"world"}`), nil, WithCopyStrings(false))
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}

	elem := obj.FindKey("a", nil)
	if elem == nil {
		t.Fatal("key 'a' not found")
	}
	b, err := elem.Iter.StringBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("got %q, want %q", string(b), "hello")
	}
	if cap(b) != len(b) {
		t.Fatalf("cap(%d) != len(%d): slice not cap-bounded, append would corrupt the buffer", cap(b), len(b))
	}

	// Appending must reallocate rather than write into the next string's
	// length prefix.
	_ = append(b, '!') //nolint:gocritic // deliberate: exercises the cap bound

	other := obj.FindKey("b", nil)
	if other == nil {
		t.Fatal("key 'b' not found")
	}
	s, err := other.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if s != "world" {
		t.Fatalf("adjacent string corrupted by append: got %q, want %q", s, "world")
	}
}

func TestIterStringBytesRespectsCopyStrings(t *testing.T) {
	// Default (copyStrings true): the result must be independent of the buffer.
	pj, err := Parse([]byte(`{"a":"hello","b":"world"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}
	elem := obj.FindKey("a", nil)
	if elem == nil {
		t.Fatal("key 'a' not found")
	}
	b, err := elem.Iter.StringBytes()
	if err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = 'X'
	}

	// Mutating the copy must not disturb the tape.
	again := obj.FindKey("a", nil)
	s, err := again.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Fatalf("StringBytes returned a view into the buffer despite copyStrings: got %q", s)
	}
}

func TestIterStringBytesRejectsNonString(t *testing.T) {
	pj, err := Parse([]byte(`{"n":42}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}
	elem := obj.FindKey("n", nil)
	if elem == nil {
		t.Fatal("key 'n' not found")
	}
	if _, err := elem.Iter.StringBytes(); err == nil {
		t.Fatal("StringBytes accepted a numeric element; the payload was read as a string offset")
	}
}

// TestIterStringBytesPastEnd covers the "iterator past end of tape" guard.
// Before it, this indexed tape.data out of range and panicked.
func TestIterStringBytesPastEnd(t *testing.T) {
	pj, err := Parse([]byte(`"hello"`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}

	// Walk off the end of the tape. Type() reports Type(-1) once exhausted, so
	// check before each Advance (Advance itself is unguarded).
	for i := 0; i < 16 && iter.Type() != Type(-1); i++ {
		iter.Advance()
	}
	if iter.Type() != Type(-1) {
		t.Fatal("could not position the iterator past the end of the tape")
	}

	if _, err := iter.StringBytes(); err == nil {
		t.Fatal("StringBytes did not report an exhausted iterator")
	}
}

func TestNextElementBytesCopiesKeysByDefault(t *testing.T) {
	pj, err := Parse([]byte(`{"alpha":1,"beta":2}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}

	var keys [][]byte
	for {
		name, _, err := obj.NextElementBytes(nil)
		if err != nil {
			t.Fatal(err)
		}
		if name == nil {
			break
		}
		keys = append(keys, name)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// Overwrite the returned key; the tape's string buffer must be unaffected.
	for i := range keys[0] {
		keys[0][i] = 'X'
	}
	obj2, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}
	if obj2.FindKey("alpha", nil) == nil {
		t.Fatal("mutating a NextElementBytes key corrupted the tape's string buffer")
	}
}

// TestNextElementBytesTruncatedTape covers the "key has no value" guard. A key
// entry in the final tape slot is not reachable from Parse, so the tape is
// constructed directly — the shape Serializer.Deserialize can produce from a
// corrupt payload.
func TestNextElementBytesTruncatedTape(t *testing.T) {
	// String buffer: [4-byte native-endian length]["alpha"][NUL]
	strs := make([]byte, 4+5+1)
	binary.NativeEndian.PutUint32(strs[0:4], 5)
	copy(strs[4:], "alpha")

	// data[0] stands in for the root entry; data[1] is a string key with no
	// following value word, so the value index lands one past the end.
	// Payload is the string-buffer offset, which is 0 here.
	keyEntry := uint64(tagString) << 56
	tape := &Tape{
		data:        []uint64{0, keyEntry},
		strings:     strs,
		copyStrings: true,
	}
	obj := &Object{
		tobj:        &TapeObject{tape: tape, startIdx: 1, endIdx: 2},
		iterPos:     1,
		copyStrings: true,
	}

	name, _, err := obj.NextElementBytes(nil)
	if err == nil {
		t.Fatalf("NextElementBytes accepted a key with no value word, returning %q", string(name))
	}
	if name != nil {
		t.Errorf("expected a nil name alongside the error, got %q", string(name))
	}
}

// TestNextElementBytesPropagatesStringError covers the readStringBytes error
// that NextElementBytes previously discarded: an out-of-range string offset must
// surface rather than yielding a silently empty key.
func TestNextElementBytesPropagatesStringError(t *testing.T) {
	// Key entry points past the end of the string buffer.
	keyEntry := uint64(tagString)<<56 | 64
	tape := &Tape{
		data:        []uint64{0, keyEntry, 0},
		strings:     make([]byte, 8),
		copyStrings: true,
	}
	obj := &Object{
		tobj:        &TapeObject{tape: tape, startIdx: 1, endIdx: 3},
		iterPos:     1,
		copyStrings: true,
	}

	if _, _, err := obj.NextElementBytes(nil); err == nil {
		t.Fatal("NextElementBytes swallowed an out-of-bounds string offset")
	}
}

// TestNextElementBytesSkipsNops verifies that NOP padding left by DeleteElems
// does not end iteration early. Before the skip loop, a deleted key made
// NextElementBytes return the end sentinel at that point, silently truncating
// the remainder of the object.
func TestNextElementBytesSkipsNops(t *testing.T) {
	cases := []struct {
		name   string
		delete string
		want   []string
	}{
		{"delete first", "a", []string{"b", "c"}},
		{"delete middle", "b", []string{"a", "c"}},
		{"delete last", "c", []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, err := Parse([]byte(`{"a":1,"b":2,"c":3}`), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			iter, err := pj.Iter()
			if err != nil {
				t.Fatal(err)
			}
			obj, err := iter.Object(nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := obj.DeleteElems(func(key []byte, _ Iter) bool {
				return string(key) == tc.delete
			}, nil); err != nil {
				t.Fatal(err)
			}

			// Fresh Object so iterPos starts at the top of the (now gapped) object.
			fresh, err := iter.Object(nil)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for {
				name, _, err := fresh.NextElementBytes(nil)
				if err != nil {
					t.Fatal(err)
				}
				if name == nil {
					break
				}
				got = append(got, string(name))
			}

			if len(got) != len(tc.want) {
				t.Fatalf("after deleting %q got keys %v, want %v", tc.delete, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("after deleting %q got keys %v, want %v", tc.delete, got, tc.want)
				}
			}
		})
	}
}

// TestNextElementBytesFalseValueIsBool guards the tagFalse mapping that moved
// from an inline special-case into Tag.Type().
func TestNextElementBytesFalseValueIsBool(t *testing.T) {
	pj, err := Parse([]byte(`{"a":false,"b":true}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, err := pj.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"a", "b"} {
		name, typ, err := obj.NextElementBytes(nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(name) != want {
			t.Fatalf("got key %q, want %q", string(name), want)
		}
		if typ != TypeBool {
			t.Errorf("key %q: got type %v, want bool", want, typ)
		}
	}
}

// TestNextElementBytesUsesTagType covers the Tag.Type() mapping for a zero tag,
// which Tag.Type() reports as Type(-1). The previous inline special-case handled
// only tagFalse and returned Type(0) here.
func TestNextElementBytesUsesTagType(t *testing.T) {
	// String buffer: [4-byte native-endian length]["k"][NUL]
	strs := make([]byte, 4+1+1)
	binary.NativeEndian.PutUint32(strs[0:4], 1)
	copy(strs[4:], "k")

	keyEntry := uint64(tagString) << 56
	tape := &Tape{
		// data[2] is the value entry and carries a zero tag.
		data:        []uint64{0, keyEntry, 0, 0},
		strings:     strs,
		copyStrings: true,
	}
	obj := &Object{
		tobj:        &TapeObject{tape: tape, startIdx: 1, endIdx: 4},
		iterPos:     1,
		copyStrings: true,
	}

	name, typ, err := obj.NextElementBytes(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(name) != "k" {
		t.Fatalf("got key %q, want %q", string(name), "k")
	}
	if typ != Type(-1) {
		t.Fatalf("got type %v, want Type(-1) via Tag.Type() for a zero tag", typ)
	}
}

// TestIterPeekNextTagSkipsNops covers the Iter-layer peek, which builds its own
// skipValue call rather than delegating to TapeIter.PeekNext and so needed its
// own NOP guard. Before the fix it reported TagNop as if it were a value tag.
func TestIterPeekNextTagSkipsNops(t *testing.T) {
	pj, err := Parse([]byte(`[1,2,3]`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	arr, err := iter.Array(nil)
	if err != nil {
		t.Fatalf("Array: %v", err)
	}
	arr.DeleteElems(func(i Iter) bool {
		v, _ := i.Int()
		return v == 2
	})

	// Position an Iter on the first element, then peek across the NOP run.
	tape, _ := pj.GetTape()
	rootIter := tape.Iter()
	tarr, err := rootIter.Array()
	if err != nil {
		t.Fatalf("TapeIter.Array: %v", err)
	}
	cur := tarr.Iter()
	it := Iter{tape: cur.tape, tapeIdx: cur.idx}

	if got := it.PeekNextTag(); got != TagInteger {
		t.Fatalf("PeekNextTag() over a NOP run = %q, want %q", rune(got), rune(TagInteger))
	}
}

// TestIterAdvanceIntoSkipsNops covers the Iter-layer wrapper, whose doc claimed
// NOP entries were skipped before the underlying TapeIter method actually did.
func TestIterAdvanceIntoSkipsNops(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1,"b":2}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	if err := obj.DeleteElems(func(key []byte, i Iter) bool { return string(key) == "a" }, nil); err != nil {
		t.Fatalf("DeleteElems: %v", err)
	}

	root, _ := pj.Iter()
	if got := root.AdvanceInto(); got != TagString {
		t.Fatalf("AdvanceInto() = %q, want %q (the key \"b\")", rune(got), rune(TagString))
	}
}

// TestIterRootIsCursorRelative covers Iter.Root, which hardcoded tape index 1:
//
//	// Position at the first element after root tag (index 1)
//	dst.tapeIdx = 1
//
// For a single-document tape index 1 IS the root value, so this looked right. On a
// ParseND tape it always returned the FIRST document with a nil error, whatever
// the cursor was on — so iterating an NDJSON stream and calling Root() on the
// third record silently handed back the first.
func TestIterRootIsCursorRelative(t *testing.T) {
	pj, err := ParseND([]byte("{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	it, err := pj.Iter()
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	for doc := 1; doc <= 3; doc++ {
		ty, root, err := it.Root(nil)
		if err != nil {
			t.Fatalf("document %d: Root: %v", doc, err)
		}
		if ty != TypeObject {
			t.Errorf("document %d: Root type = %v, want %v", doc, ty, TypeObject)
		}
		v, err := root.Interface()
		if err != nil {
			t.Fatalf("document %d: Interface: %v", doc, err)
		}
		want := map[string]interface{}{"n": int64(doc)}
		if !reflect.DeepEqual(v, want) {
			t.Errorf("document %d: Root() = %#v, want %#v", doc, v, want)
		}
		if doc < 3 && it.Advance() == Type(-1) {
			t.Fatalf("walk ended early after document %d", doc)
		}
	}
}

// TestIterRootFromNestedCursor checks Root resolves the enclosing document from a
// cursor inside a container, not just from one sitting on a document's value.
func TestIterRootFromNestedCursor(t *testing.T) {
	pj, err := ParseND([]byte("{\"n\":1}\n{\"n\":2,\"deep\":{\"x\":[7]}}\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	it, _ := pj.Iter()
	if it.Advance() == Type(-1) {
		t.Fatal("could not reach the second document")
	}
	obj, err := it.Object(nil)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	deep, err := obj.FindKey("deep", nil).Iter.Object(nil)
	if err != nil {
		t.Fatalf("nested Object: %v", err)
	}
	inner := deep.FindKey("x", nil)

	_, root, err := inner.Iter.Root(nil)
	if err != nil {
		t.Fatalf("Root from a nested cursor: %v", err)
	}
	v, err := root.Interface()
	if err != nil {
		t.Fatalf("Interface: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("Root() = %T, want map", v)
	}
	if m["n"] != int64(2) {
		t.Errorf("Root() resolved to the wrong document: n = %#v, want 2", m["n"])
	}
}

// TestIterRootOutsideAnyDocument covers the cursor being past the end. Root used
// to return index 1 and a nil error regardless; it now reports the failure while
// still handing back a usable dst, since the method never used to fail and a
// caller ignoring the error must not receive a nil pointer.
func TestIterRootOutsideAnyDocument(t *testing.T) {
	pj, err := ParseND([]byte("1\n2\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	it, _ := pj.Iter()
	for step := 0; step < 8 && it.Advance() != Type(-1); step++ {
	}

	ty, root, err := it.Root(nil)
	if err == nil {
		t.Error("Root() on an exhausted cursor returned no error")
	}
	if root == nil {
		t.Fatal("Root() returned a nil dst alongside the error")
	}
	if ty != Type(-1) {
		t.Errorf("Root() type = %v, want Type(-1)", ty)
	}
	if got := root.Type(); got != Type(-1) {
		t.Errorf("dst.Type() = %v, want Type(-1)", got)
	}
}

// TestIterRootSingleDocumentUnchanged pins the behaviour the old hardcoded index
// got right, so the rewrite cannot regress the common case.
func TestIterRootSingleDocumentUnchanged(t *testing.T) {
	pj, err := Parse([]byte(`{"a":{"b":1}}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()

	it, _ := pj.Iter()
	obj, _ := it.Object(nil)
	nested := obj.FindKey("a", nil)

	for _, tc := range []struct {
		name string
		from Iter
	}{
		{"from_root", it},
		{"from_nested", nested.Iter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from := tc.from
			ty, root, err := from.Root(nil)
			if err != nil {
				t.Fatalf("Root: %v", err)
			}
			if ty != TypeObject {
				t.Errorf("type = %v, want %v", ty, TypeObject)
			}
			v, err := root.Interface()
			if err != nil {
				t.Fatalf("Interface: %v", err)
			}
			want := map[string]interface{}{"a": map[string]interface{}{"b": int64(1)}}
			if !reflect.DeepEqual(v, want) {
				t.Errorf("Root() = %#v, want the whole document %#v", v, want)
			}
		})
	}
}

// TestIterRootWhenAlreadyAtRoot covers a cursor sitting ON a root marker rather
// than on a document's value.
//
// rootDocContaining originally took the block's extent as (root, payload) —
// excluding the marker itself — so a cursor at index 0 reported "not inside a root
// document" for a perfectly good tape. Asking Root() where you already are should
// answer, not fail. The extent is now [root, payload).
//
// pj.Iter() starts at the value (root+1), so this position is not reachable
// through the normal API; the iterators are built directly.
func TestIterRootWhenAlreadyAtRoot(t *testing.T) {
	pj, err := ParseND([]byte("{\"n\":1}\n{\"n\":2}\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()
	tape, err := pj.GetTape()
	if err != nil {
		t.Fatalf("GetTape: %v", err)
	}

	// Locate each block's opening root rather than hardcoding indices.
	var roots []int
	for root := 0; tape.hasRootDocAt(root); root = tape.nextRootDoc(root) {
		roots = append(roots, root)
	}
	if len(roots) != 2 {
		t.Fatalf("found %d root blocks, want 2", len(roots))
	}

	for doc, root := range roots {
		it := Iter{tape: tape, tapeIdx: root, copyStrings: true}
		ty, dst, err := it.Root(nil)
		if err != nil {
			t.Errorf("cursor on root marker at %d: Root: %v", root, err)
			continue
		}
		if ty != TypeObject {
			t.Errorf("cursor on root marker at %d: type = %v, want %v", root, ty, TypeObject)
		}
		v, err := dst.Interface()
		if err != nil {
			t.Errorf("cursor on root marker at %d: Interface: %v", root, err)
			continue
		}
		want := map[string]interface{}{"n": int64(doc + 1)}
		if !reflect.DeepEqual(v, want) {
			t.Errorf("cursor on root marker at %d: Root() = %#v, want %#v", root, v, want)
		}
	}
}

// TestIterRootIsIdempotent checks that feeding Root's own result back into Root
// returns the same position — the property that makes it safe to call defensively
// without knowing whether the cursor has already been normalised.
func TestIterRootIsIdempotent(t *testing.T) {
	for _, input := range []string{
		"{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n",
		"1\n2\n3\n", // scalar documents: the tightest blocks
		"{\"a\":1}\n",
	} {
		pj, err := ParseND([]byte(input), nil)
		if err != nil {
			t.Fatalf("ParseND(%q): %v", input, err)
		}

		it, _ := pj.Iter()
		for doc := 0; doc < 4; doc++ {
			_, first, err := it.Root(nil)
			if err != nil {
				t.Fatalf("%q doc %d: Root: %v", input, doc, err)
			}
			_, second, err := first.Root(nil)
			if err != nil {
				t.Fatalf("%q doc %d: Root(Root()): %v", input, doc, err)
			}
			if first.tapeIdx != second.tapeIdx {
				t.Errorf("%q doc %d: Root is not idempotent: %d then %d",
					input, doc, first.tapeIdx, second.tapeIdx)
			}
			if it.Advance() == Type(-1) {
				break
			}
		}
		pj.Close()
	}
}

// TestIterRootIntoAliasedDst covers dst aliasing the receiver — Root(&it) — since
// reuse is the idiom this library promotes and the implementation copies *i into
// dst before reading i.tapeIdx.
func TestIterRootIntoAliasedDst(t *testing.T) {
	pj, err := ParseND([]byte("{\"n\":1}\n{\"n\":2}\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	it, _ := pj.Iter()
	if it.Advance() == Type(-1) {
		t.Fatal("could not reach the second document")
	}
	obj, err := it.Object(nil)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	nested := obj.FindKey("n", nil).Iter

	ty, dst, err := nested.Root(&nested)
	if err != nil {
		t.Fatalf("Root(&self): %v", err)
	}
	if dst != &nested {
		t.Errorf("Root(&self) returned a different pointer than the dst passed in")
	}
	if ty != TypeObject {
		t.Errorf("type = %v, want %v", ty, TypeObject)
	}
	v, err := dst.Interface()
	if err != nil {
		t.Fatalf("Interface: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok || m["n"] != int64(2) {
		t.Errorf("Root(&self) = %#v, want the second document", v)
	}
}
