package simdjson

import (
	"encoding/json"
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
		v := obj.FindKey("name")
		if v == nil {
			t.Fatal("not found")
		}
		s, _ := v.String()
		if s != "test" {
			t.Fatalf("expected 'test', got %q", s)
		}
	})

	t.Run("FindKey int", func(t *testing.T) {
		v := obj.FindKey("count")
		n, _ := v.Int()
		if n != 42 {
			t.Fatalf("expected 42, got %d", n)
		}
	})

	t.Run("FindKey float", func(t *testing.T) {
		v := obj.FindKey("pi")
		f, _ := v.Float()
		if f != 3.14 {
			t.Fatalf("expected 3.14, got %f", f)
		}
	})

	t.Run("FindKey bool", func(t *testing.T) {
		v := obj.FindKey("ok")
		b, _ := v.Bool()
		if !b {
			t.Fatal("expected true")
		}
	})

	t.Run("FindKey missing", func(t *testing.T) {
		if obj.FindKey("missing") != nil {
			t.Fatal("expected nil")
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

	v := obj.FindPath("Image", "Thumbnail", "Url")
	if v == nil {
		t.Fatal("not found")
	}
	s, _ := v.String()
	if s != "http://www.example.com/image/481989943" {
		t.Fatalf("expected URL, got %q", s)
	}

	if obj.FindPath("Image", "Missing") != nil {
		t.Fatal("expected nil for missing path")
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

	v := iter.FindElement("Image", "Width")
	if v == nil {
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
