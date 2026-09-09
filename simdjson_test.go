package simdjson

import (
	"encoding/json"
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

// --- #6: Parse returns the caller's parser on error and clears its tape ---

func TestParseErrorReturnsParserAndClearsTape(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	pj, err := Parse([]byte(`{"a":1}`), pj)
	if err != nil {
		t.Fatal(err)
	}
	if !pj.hasTape {
		t.Fatal("expected a tape after a successful parse")
	}

	got, err := Parse([]byte(`{invalid`), pj)
	if err == nil {
		t.Fatal("expected an error for malformed input")
	}
	if got == nil {
		t.Fatal("Parse returned nil on error, discarding the caller's parser")
	}
	if got != pj {
		t.Fatal("Parse returned a different ParsedJson than the one passed for reuse")
	}
	if got.hasTape {
		t.Fatal("failed parse left hasTape set, exposing the previous document")
	}
	if _, err := got.GetTape(); err == nil {
		t.Fatal("GetTape succeeded after a failed parse; stale tape is still readable")
	}
}

func TestParseEmptyInputReturnsParser(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	pj, err := Parse([]byte(`{"a":1}`), pj)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Parse(nil, pj)
	if err == nil {
		t.Fatal("expected an error for empty input")
	}
	if got == nil {
		t.Fatal("Parse returned nil for empty input, discarding the caller's parser")
	}
	if got.hasTape {
		t.Fatal("empty input left the previous tape in place")
	}
}

// --- #7: parser options do not accumulate across reuse or pooling ---

func TestParseOptionsDoNotLeakAcrossReuse(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	pj, err := Parse([]byte(`{"n":42}`), pj, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	v, err := pj.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(map[string]interface{})["n"].(json.Number); !ok {
		t.Fatalf("UseNumber not applied: got %T", v.(map[string]interface{})["n"])
	}

	// Second parse passes no options; the first call's must not persist.
	pj, err = Parse([]byte(`{"n":42}`), pj)
	if err != nil {
		t.Fatal(err)
	}
	v, err = pj.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}
	n := v.(map[string]interface{})["n"]
	if _, isNum := n.(json.Number); isNum {
		t.Fatal("UseNumber leaked into a later parse that passed no options")
	}
	if _, ok := n.(int64); !ok {
		t.Fatalf("expected int64 without UseNumber, got %T", n)
	}
}

func TestBigIntOptionDoesNotLeakAcrossReuse(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	big := []byte(`123456789012345678901234567890`)

	pj, err := Parse(big, pj, UseBigInt())
	if err != nil {
		t.Fatalf("UseBigInt should accept an oversized integer: %v", err)
	}

	// Without UseBigInt the same input must be rejected again. If bigInt leaked,
	// this silently succeeds.
	if _, err = Parse(big, pj); err == nil {
		t.Fatal("UseBigInt leaked: oversized integer accepted without the option")
	}
}

func TestResetRestoresDefaultOptions(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	if _, err := Parse([]byte(`{"n":1}`), pj, UseNumber(), WithCopyStrings(false)); err != nil {
		t.Fatal(err)
	}
	pj.Reset()

	if !pj.copyStrings {
		t.Error("Reset did not restore copyStrings to its default")
	}
	if pj.useNumber {
		t.Error("Reset did not clear useNumber")
	}
	if pj.bigInt {
		t.Error("Reset did not clear bigInt")
	}
	if pj.hasTape {
		t.Error("Reset did not clear hasTape")
	}
}

// TestPutParserResetsBeforePooling asserts deterministically that PutParser
// resets the parser before returning it to the pool. It deliberately inspects
// the instance just handed back — safe because it only reads state and never
// parses with it again — because asserting via GetParser would depend on
// sync.Pool returning the same instance.
//
// Do not add t.Parallel() to this test: the post-PutParser read is only safe
// while no other goroutine can retrieve the pooled instance.
func TestPutParserResetsBeforePooling(t *testing.T) {
	pj := GetParser()
	if _, err := Parse([]byte(`{"n":1}`), pj, UseNumber(), UseBigInt(), WithCopyStrings(false)); err != nil {
		t.Fatal(err)
	}
	if !pj.hasTape || !pj.useNumber || !pj.bigInt || pj.copyStrings {
		t.Fatal("precondition failed: options were not applied by Parse")
	}

	PutParser(pj)

	if pj.hasTape {
		t.Error("PutParser pooled a parser still holding a tape")
	}
	if !pj.copyStrings {
		t.Error("PutParser pooled a parser with copyStrings still disabled")
	}
	if pj.useNumber {
		t.Error("PutParser pooled a parser with useNumber still set")
	}
	if pj.bigInt {
		t.Error("PutParser pooled a parser with bigInt still set")
	}
}

// TestGetParserYieldsCleanState checks the resulting pool invariant: whatever
// GetParser hands back carries no tape and default options.
//
// Do not add t.Parallel() to this test — see TestPutParserResetsBeforePooling.
func TestGetParserYieldsCleanState(t *testing.T) {
	for i := 0; i < 8; i++ {
		pj := GetParser()
		if pj.hasTape {
			t.Fatal("GetParser returned a parser still holding a tape")
		}
		if !pj.copyStrings || pj.useNumber || pj.bigInt {
			t.Fatal("GetParser returned a parser with non-default options")
		}
		if _, err := Parse([]byte(`{"n":1}`), pj, UseNumber(), UseBigInt()); err != nil {
			t.Fatal(err)
		}
		PutParser(pj)
	}
}

// --- #10: Clone never returns nil and clears a reused dst ---

func TestCloneNilDstWithNoTape(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	cloned := pj.Clone(nil)
	if cloned == nil {
		t.Fatal("Clone(nil) returned nil for a parser with no tape")
	}
	if cloned.hasTape {
		t.Fatal("clone of a tapeless parser reports a tape")
	}
	if _, err := cloned.Iter(); err == nil {
		t.Fatal("expected an error iterating a clone with no tape")
	}
}

func TestCloneReusedDstClearsStaleTape(t *testing.T) {
	src, err := Parse([]byte(`{"x":"original"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	dst := src.Clone(nil)
	src.Close()

	if _, err := dst.GetTape(); err != nil {
		t.Fatalf("clone should hold a tape: %v", err)
	}

	// Cloning a tapeless parser into the same dst must clear it, not leave the
	// previous document readable.
	empty := GetParser()
	defer PutParser(empty)
	got := empty.Clone(dst)

	if got != dst {
		t.Fatal("Clone did not reuse the supplied dst")
	}
	if got.hasTape {
		t.Fatal("reused dst kept its stale tape after cloning a tapeless parser")
	}
	if _, err := got.GetTape(); err == nil {
		t.Fatal("stale tape still readable through the reused clone target")
	}
}

// TestCloneCopiesOptionsWhenSourceHasNoTape covers the option-flag copy on
// Clone's no-tape path. Parse applies opts before rejecting empty input, so the
// parser below carries options without holding a tape.
func TestCloneCopiesOptionsWhenSourceHasNoTape(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	pj, err := Parse(nil, pj, UseNumber(), WithCopyStrings(false))
	if err == nil {
		t.Fatal("expected an error for empty input")
	}
	if pj.hasTape {
		t.Fatal("precondition failed: parser should hold no tape")
	}
	if !pj.useNumber || pj.copyStrings {
		t.Fatal("precondition failed: options were not applied before the empty-input check")
	}

	dst := pj.Clone(nil)
	if dst == nil {
		t.Fatal("Clone(nil) returned nil")
	}
	if dst.hasTape {
		t.Error("clone of a tapeless parser reports a tape")
	}
	if !dst.useNumber {
		t.Error("Clone dropped useNumber on the no-tape path")
	}
	if dst.copyStrings {
		t.Error("Clone dropped copyStrings on the no-tape path")
	}
	if dst.bigInt != pj.bigInt {
		t.Error("Clone dropped bigInt on the no-tape path")
	}
}

func TestCloneOutlivesSourceClose(t *testing.T) {
	pj, err := Parse([]byte(`{"name":"test","count":42}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	cloned := pj.Clone(nil)
	pj.Close()

	iter, err := cloned.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}
	elem := obj.FindKey("name", nil)
	if elem == nil {
		t.Fatal("key 'name' not found in clone")
	}
	s, err := elem.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if s != "test" {
		t.Fatalf("got %q, want %q", s, "test")
	}
}

// TestStringLifetimeSurvivesReparseAndClose locks in the memory contract that the
// doc comments on Iter.String, Iter.StringRef, WithCopyStrings and the Tape struct
// now describe.
//
// Those docs previously claimed strings "point into parser-owned memory and are
// only valid until the next Parse call or Close". That was true of an earlier
// zero-copy design but not of this code: Parse memcpys both C++ buffers into
// freshly allocated Go slices (copyByteSlice does make([]byte, n) every call), so
// an earlier tape's buffer is never overwritten in place — it is simply dropped.
// Values read from it stay valid for as long as the caller holds them, under
// either WithCopyStrings setting.
func TestStringLifetimeSurvivesReparseAndClose(t *testing.T) {
	for _, copyStrings := range []bool{true, false} {
		name := "copy"
		if !copyStrings {
			name = "nocopy"
		}
		t.Run(name, func(t *testing.T) {
			pj, err := Parse([]byte(`{"k":"first-value"}`), nil, WithCopyStrings(copyStrings))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			iter, _ := pj.Iter()
			obj, err := iter.Object(nil)
			if err != nil {
				t.Fatalf("Object: %v", err)
			}
			e := obj.FindKey("k", nil)
			s, err := e.Iter.String()
			if err != nil {
				t.Fatalf("String: %v", err)
			}
			ref, err := e.Iter.StringRef()
			if err != nil {
				t.Fatalf("StringRef: %v", err)
			}
			b, err := e.Iter.StringBytes()
			if err != nil {
				t.Fatalf("StringBytes: %v", err)
			}
			if s != "first-value" || ref != "first-value" || string(b) != "first-value" {
				t.Fatalf("initial reads = %q / %q / %q", s, ref, string(b))
			}

			// Reparse a DIFFERENT document into the same parser. A buffer reused in
			// place would corrupt the values captured above.
			if _, err := Parse([]byte(`{"k":"second-value-longer"}`), pj, WithCopyStrings(copyStrings)); err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if s != "first-value" || ref != "first-value" || string(b) != "first-value" {
				t.Errorf("after reparse: %q / %q / %q; want all first-value", s, ref, string(b))
			}

			// Close frees the C++ parser. Go-managed values must be unaffected.
			pj.Close()
			if s != "first-value" || ref != "first-value" || string(b) != "first-value" {
				t.Errorf("after Close: %q / %q / %q; want all first-value", s, ref, string(b))
			}
		})
	}
}

// TestStringBytesCapIsPinnedUnderNoCopy is the counterpart to the lifetime test:
// under WithCopyStrings(false) the []byte aliases the tape's buffer, so its
// capacity must be pinned to its length. Appending to it must reallocate rather
// than overwrite the adjacent string's length prefix.
func TestStringBytesCapIsPinnedUnderNoCopy(t *testing.T) {
	pj, err := Parse([]byte(`{"a":"aaaa","b":"bbbb"}`), nil, WithCopyStrings(false))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer pj.Close()

	iter, _ := pj.Iter()
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	a, err := obj.FindKey("a", nil).Iter.StringBytes()
	if err != nil {
		t.Fatalf("StringBytes: %v", err)
	}
	if cap(a) != len(a) {
		t.Fatalf("cap=%d len=%d; cap must be pinned to len", cap(a), len(a))
	}

	// Appending must not disturb the neighbouring value.
	_ = append(a, 'X')
	b, err := obj.FindKey("b", nil).Iter.StringBytes()
	if err != nil {
		t.Fatalf("StringBytes(b) after append: %v", err)
	}
	if string(b) != "bbbb" {
		t.Errorf("neighbouring value corrupted by append: %q, want \"bbbb\"", string(b))
	}
}
