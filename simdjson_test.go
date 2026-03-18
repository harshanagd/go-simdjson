package simdjson

import (
	"testing"
)

func TestParseValidJSON(t *testing.T) {
	pj, err := Parse([]byte(`{"hello": "world"}`), nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pj.Close()

	if pj.RootType() != TypeObject {
		t.Fatalf("expected object, got %v", pj.RootType())
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`), nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseEmpty(t *testing.T) {
	_, err := Parse([]byte{}, nil)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestRootTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType Type
	}{
		{"object", `{"a":1}`, TypeObject},
		{"array", `[1,2,3]`, TypeArray},
		{"string", `"hello"`, TypeString},
		{"int64", `42`, TypeInt64},
		{"negative", `-10`, TypeInt64},
		{"double", `3.14`, TypeDouble},
		{"bool_true", `true`, TypeBool},
		{"bool_false", `false`, TypeBool},
		{"null", `null`, TypeNull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pj, err := Parse([]byte(tt.input), nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			defer pj.Close()
			if pj.RootType() != tt.wantType {
				t.Fatalf("expected %v, got %v", tt.wantType, pj.RootType())
			}
		})
	}
}

func TestFindString(t *testing.T) {
	pj, err := Parse([]byte(`{"name":"simdjson","version":"4.0.1"}`), nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pj.Close()

	val, err := pj.FindString("name")
	if err != nil {
		t.Fatalf("FindString failed: %v", err)
	}
	if val != "simdjson" {
		t.Fatalf("expected 'simdjson', got %q", val)
	}

	val, err = pj.FindString("version")
	if err != nil {
		t.Fatalf("FindString failed: %v", err)
	}
	if val != "4.0.1" {
		t.Fatalf("expected '4.0.1', got %q", val)
	}
}

func TestFindStringMissing(t *testing.T) {
	pj, err := Parse([]byte(`{"a":"b"}`), nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pj.Close()

	_, err = pj.FindString("missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestRootValueExtraction(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		pj, err := Parse([]byte(`"hello"`), nil)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer pj.Close()
		val, err := pj.RootString()
		if err != nil {
			t.Fatalf("RootString failed: %v", err)
		}
		if val != "hello" {
			t.Fatalf("expected 'hello', got %q", val)
		}
	})

	t.Run("int64", func(t *testing.T) {
		pj, err := Parse([]byte(`-42`), nil)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer pj.Close()
		val, err := pj.RootInt64()
		if err != nil {
			t.Fatalf("RootInt64 failed: %v", err)
		}
		if val != -42 {
			t.Fatalf("expected -42, got %d", val)
		}
	})

	t.Run("double", func(t *testing.T) {
		pj, err := Parse([]byte(`3.14`), nil)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer pj.Close()
		val, err := pj.RootDouble()
		if err != nil {
			t.Fatalf("RootDouble failed: %v", err)
		}
		if val != 3.14 {
			t.Fatalf("expected 3.14, got %f", val)
		}
	})

	t.Run("bool", func(t *testing.T) {
		pj, err := Parse([]byte(`true`), nil)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer pj.Close()
		val, err := pj.RootBool()
		if err != nil {
			t.Fatalf("RootBool failed: %v", err)
		}
		if !val {
			t.Fatal("expected true")
		}
	})
}

func TestRootCount(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		pj, err := Parse([]byte(`[1,2,3]`), nil)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer pj.Close()
		n, err := pj.RootCount()
		if err != nil {
			t.Fatalf("RootCount failed: %v", err)
		}
		if n != 3 {
			t.Fatalf("expected 3, got %d", n)
		}
	})

	t.Run("object", func(t *testing.T) {
		pj, err := Parse([]byte(`{"a":1,"b":2}`), nil)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer pj.Close()
		n, err := pj.RootCount()
		if err != nil {
			t.Fatalf("RootCount failed: %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2, got %d", n)
		}
	})
}

func TestParserReuse(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	// First parse
	pj, err := Parse([]byte(`{"a":1}`), pj)
	if err != nil {
		t.Fatalf("first parse failed: %v", err)
	}
	if pj.RootType() != TypeObject {
		t.Fatalf("expected object, got %v", pj.RootType())
	}

	// Reuse for second parse
	pj, err = Parse([]byte(`[1,2]`), pj)
	if err != nil {
		t.Fatalf("second parse failed: %v", err)
	}
	if pj.RootType() != TypeArray {
		t.Fatalf("expected array, got %v", pj.RootType())
	}
}

func TestTypeString(t *testing.T) {
	if TypeObject.String() != "object" {
		t.Fatalf("expected 'object', got %q", TypeObject.String())
	}
	if TypeArray.String() != "array" {
		t.Fatalf("expected 'array', got %q", TypeArray.String())
	}
}

func TestSupportedCPU(t *testing.T) {
	if !SupportedCPU() {
		t.Fatal("SupportedCPU() returned false")
	}
}

func TestActiveImplementation(t *testing.T) {
	impl := ActiveImplementation()
	if impl == "" {
		t.Fatal("ActiveImplementation() returned empty string")
	}
	t.Logf("active SIMD implementation: %s", impl)
}

func BenchmarkParse(b *testing.B) {
	data := []byte(`{"key":"value","number":42,"array":[1,2,3],"nested":{"a":"b"}}`)
	pj := GetParser()
	defer PutParser(pj)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pj, _ = Parse(data, pj)
	}
}

func BenchmarkFindString(b *testing.B) {
	data := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`)
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = pj.FindString("$schema")
	}
}
