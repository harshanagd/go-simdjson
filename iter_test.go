package simdjson

import "testing"

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
