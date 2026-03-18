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
