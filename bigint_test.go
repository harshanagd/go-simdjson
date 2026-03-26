package simdjson

import (
	"encoding/json"
	"testing"
)

const bigIntJSON = `{"small":42,"big":123456789012345678901,"neg":-99999999999999999999999}`

// --- UseBigInt tests ---

func TestBigIntParse(t *testing.T) {
	pj, err := Parse([]byte(bigIntJSON), nil, UseBigInt())
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

	// small should still be int64
	if _, ok := m["small"].(int64); !ok {
		t.Fatalf("small: expected int64, got %T", m["small"])
	}
	// big should be json.Number
	if n, ok := m["big"].(json.Number); !ok {
		t.Fatalf("big: expected json.Number, got %T", m["big"])
	} else if n.String() != "123456789012345678901" {
		t.Fatalf("big: expected 123456789012345678901, got %s", n)
	}
	// neg should be json.Number
	if n, ok := m["neg"].(json.Number); !ok {
		t.Fatalf("neg: expected json.Number, got %T", m["neg"])
	} else if n.String() != "-99999999999999999999999" {
		t.Fatalf("neg: expected -99999999999999999999999, got %s", n)
	}
}

func TestBigIntWithoutOption(t *testing.T) {
	_, err := Parse([]byte(`{"big":123456789012345678901}`), nil)
	if err == nil {
		t.Fatal("expected error for big integer without UseBigInt")
	}
}

func TestBigIntTapeIter(t *testing.T) {
	pj, err := Parse([]byte(`[123456789012345678901]`), nil, UseBigInt())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	tape, _ := pj.GetTape()
	ti := tape.Iter()
	obj, _ := ti.Array()
	ai := obj.Iter()
	if ai.Type() != TypeBigInt {
		t.Fatalf("expected TypeBigInt, got %v", ai.Type())
	}
	n, err := ai.BigInt()
	if err != nil {
		t.Fatal(err)
	}
	if n.String() != "123456789012345678901" {
		t.Fatalf("expected 123456789012345678901, got %s", n)
	}
}

func TestBigIntMarshalJSON(t *testing.T) {
	pj, err := Parse([]byte(`{"val":123456789012345678901}`), nil, UseBigInt())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	b, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// Should contain raw digits, not quoted
	if s != `{"val":123456789012345678901}` {
		t.Fatalf("unexpected marshal: %s", s)
	}
}

// --- UseNumber tests (all numbers as json.Number, including big ints) ---

func TestUseNumberWithBigInt(t *testing.T) {
	pj, err := Parse([]byte(bigIntJSON), nil, UseNumber())
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

	// All numbers should be json.Number
	for _, key := range []string{"small", "big", "neg"} {
		if _, ok := m[key].(json.Number); !ok {
			t.Fatalf("%s: expected json.Number, got %T", key, m[key])
		}
	}
	if m["small"].(json.Number).String() != "42" {
		t.Fatalf("small: expected 42, got %s", m["small"])
	}
	if m["big"].(json.Number).String() != "123456789012345678901" {
		t.Fatalf("big: expected 123456789012345678901, got %s", m["big"])
	}
}

func TestUseNumberNormalNumbers(t *testing.T) {
	pj, err := Parse([]byte(`{"i":42,"f":3.14,"u":18446744073709551615}`), nil, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	v, _ := iter.Interface()
	m := v.(map[string]interface{})

	if m["i"].(json.Number).String() != "42" {
		t.Fatalf("i: got %s", m["i"])
	}
	if m["f"].(json.Number).String() != "3.14" {
		t.Fatalf("f: got %s", m["f"])
	}
	if m["u"].(json.Number).String() != "18446744073709551615" {
		t.Fatalf("u: got %s", m["u"])
	}
}

// --- Mixed: UseBigInt without UseNumber ---

func TestBigIntNormalNumbersUnaffected(t *testing.T) {
	pj, err := Parse([]byte(`{"i":42,"f":3.14,"big":123456789012345678901}`), nil, UseBigInt())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	v, _ := iter.Interface()
	m := v.(map[string]interface{})

	// Normal numbers stay native types
	if _, ok := m["i"].(int64); !ok {
		t.Fatalf("i: expected int64, got %T", m["i"])
	}
	if _, ok := m["f"].(float64); !ok {
		t.Fatalf("f: expected float64, got %T", m["f"])
	}
	// Big int is json.Number
	if _, ok := m["big"].(json.Number); !ok {
		t.Fatalf("big: expected json.Number, got %T", m["big"])
	}
}

func TestBigIntInArray(t *testing.T) {
	pj, err := Parse([]byte(`[1, 123456789012345678901, 3]`), nil, UseBigInt())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	v, _ := iter.Interface()
	arr := v.([]interface{})
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	if _, ok := arr[0].(int64); !ok {
		t.Fatalf("[0]: expected int64, got %T", arr[0])
	}
	if n, ok := arr[1].(json.Number); !ok {
		t.Fatalf("[1]: expected json.Number, got %T", arr[1])
	} else if n.String() != "123456789012345678901" {
		t.Fatalf("[1]: got %s", n)
	}
	if _, ok := arr[2].(int64); !ok {
		t.Fatalf("[2]: expected int64, got %T", arr[2])
	}
}
