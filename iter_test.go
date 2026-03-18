package simdjson

import (
	"encoding/json"
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
