package simdjson

import (
	"encoding/json"
	"testing"
)

// demo_json matches the test fixture used in minio/simdjson-go.
const demo_json = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

// TestParseAgainstStdlib validates that simdjson accepts/rejects the same
// inputs as encoding/json (adapted from minio/simdjson-go fuzz tests).
func TestParseAgainstStdlib(t *testing.T) {
	cases := []string{
		`{}`, `[]`, `null`, `true`, `false`, `0`, `""`,
		`{"a":1}`, `[1,2,3]`, `{"a":{"b":{"c":true}}}`,
		`"hello world"`, `3.14159`, `-42`, `1e10`,
		`{"key":"value","number":42,"array":[1,2,3],"nested":{"a":"b"}}`,
		demo_json,
		// edge cases
		`{"":0}`,                   // empty key
		`{"a":null,"b":null}`,      // null values
		`[null,null,null]`,         // null array
		`{"a":[1,[2,[3]]]}`,        // nested arrays
		`"\u0041\u0042\u0043"`,     // unicode escapes
		`"\n\t\r\\\""`,            // escape sequences
		`[1.0e+308]`,              // large float
		`[-1.0e+308]`,             // large negative float
		`[9223372036854775807]`,    // max int64
		`[-9223372036854775808]`,   // min int64
		`[18446744073709551615]`,   // max uint64
	}
	for _, input := range cases {
		t.Run(input[:min(len(input), 30)], func(t *testing.T) {
			var stdlibResult interface{}
			stdlibErr := json.Unmarshal([]byte(input), &stdlibResult)

			pj, simdjsonErr := Parse([]byte(input), nil)
			if simdjsonErr != nil && stdlibErr == nil {
				t.Fatalf("simdjson rejected valid JSON: %v", simdjsonErr)
			}
			if simdjsonErr == nil && stdlibErr != nil {
				t.Fatalf("simdjson accepted invalid JSON that stdlib rejected: %v", stdlibErr)
			}
			if pj != nil {
				pj.Close()
			}
		})
	}
}

// TestParseRejectsInvalid validates simdjson rejects malformed JSON
// (adapted from simdjson jsonchecker test suite).
func TestParseRejectsInvalid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"trailing comma obj", `{"a":1,}`},
		{"trailing comma arr", `[1,2,]`},
		{"single quote", `{'a':1}`},
		{"unquoted key", `{a:1}`},
		{"missing colon", `{"a" 1}`},
		{"missing value", `{"a":}`},
		{"double comma", `[1,,2]`},
		{"leading zero", `[01]`},
		{"plus number", `[+1]`},
		{"bare comma", `,`},
		{"bare colon", `:`},
		{"unclosed object", `{"a":1`},
		{"unclosed array", `[1,2`},
		{"unclosed string", `"hello`},
		{"tab in string", "{\"a\":\"\t\"}"},
		{"newline in string", "{\"a\":\"\n\"}"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input), nil)
			if err == nil {
				t.Fatalf("expected error for: %s", tt.input)
			}
		})
	}
}

// TestDemoJSON validates the standard simdjson demo document
// (adapted from minio/simdjson-go TestPrintJson).
func TestDemoJSON(t *testing.T) {
	pj, err := Parse([]byte(demo_json), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	if pj.RootType() != TypeObject {
		t.Fatalf("expected object root, got %v", pj.RootType())
	}

	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// Navigate: root -> "Image" -> object
	imgElem := obj.FindKey("Image", nil)
	if imgElem == nil {
		t.Fatal("Image key not found")
	}
	if imgElem.Iter.Type() != TypeObject {
		t.Fatalf("Image: expected object, got %v", imgElem.Iter.Type())
	}

	imgObj, err := imgElem.Iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Width: 800
	w := imgObj.FindKey("Width", nil)
	if w == nil {
		t.Fatal("Width not found")
	}
	width, err := w.Iter.Int()
	if err != nil {
		t.Fatal(err)
	}
	if width != 800 {
		t.Fatalf("Width: expected 800, got %d", width)
	}

	// Title: string
	title := imgObj.FindKey("Title", nil)
	if title == nil {
		t.Fatal("Title not found")
	}
	titleStr, err := title.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if titleStr != "View from 15th Floor" {
		t.Fatalf("Title: expected 'View from 15th Floor', got %q", titleStr)
	}

	// Animated: false
	anim := imgObj.FindKey("Animated", nil)
	if anim == nil {
		t.Fatal("Animated not found")
	}
	animated, err := anim.Iter.Bool()
	if err != nil {
		t.Fatal(err)
	}
	if animated {
		t.Fatal("Animated: expected false")
	}

	// Thumbnail -> Url
	thumb := imgObj.FindKey("Thumbnail", nil)
	if thumb == nil {
		t.Fatal("Thumbnail not found")
	}
	thumbObj, err := thumb.Iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}
	url := thumbObj.FindKey("Url", nil)
	if url == nil {
		t.Fatal("Url not found")
	}
	urlStr, err := url.Iter.String()
	if err != nil {
		t.Fatal(err)
	}
	if urlStr != "http://www.example.com/image/481989943" {
		t.Fatalf("Url: expected example.com URL, got %q", urlStr)
	}
}

// TestParseAllTestFiles validates all test fixtures parse successfully
// and that root type matches what encoding/json produces
// (adapted from minio/simdjson-go TestVerifyTape).
func TestParseAllTestFiles(t *testing.T) {
	allFiles := []struct {
		name  string
		array bool
	}{
		{"apache_builds", false},
		{"canada", false},
		{"citm_catalog", false},
		{"github_events", true},
		{"gsoc-2018", false},
		{"instruments", false},
		{"marine_ik", false},
		{"mesh", false},
		{"mesh.pretty", false},
		{"numbers", true},
		{"twitterescaped", false},
		{"twitter", false},
		{"random", false},
		{"update-center", false},
	}
	for _, tt := range allFiles {
		t.Run(tt.name, func(t *testing.T) {
			data := loadTestFile(t, tt.name)

			// Parse with simdjson
			pj, err := Parse(data, nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			defer pj.Close()

			// Verify root type
			if tt.array {
				if pj.RootType() != TypeArray {
					t.Fatalf("expected array root, got %v", pj.RootType())
				}
			} else {
				if pj.RootType() != TypeObject {
					t.Fatalf("expected object root, got %v", pj.RootType())
				}
			}

			// Verify encoding/json also accepts it
			var v interface{}
			if err := json.Unmarshal(data, &v); err != nil {
				t.Fatalf("encoding/json rejected file that simdjson accepted: %v", err)
			}
		})
	}
}

// TestParserPoolConcurrency validates pool safety under concurrent use
// (adapted from minio/simdjson-go TestExchange).
func TestParserPoolConcurrency(t *testing.T) {
	inputs := []string{
		`{"a":1}`,
		`[1,2,3]`,
		`"hello"`,
		`42`,
		`true`,
		demo_json,
	}
	t.Parallel()
	for _, input := range inputs {
		input := input
		t.Run(input[:min(len(input), 20)], func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 100; i++ {
				pj := GetParser()
				pj, err := Parse([]byte(input), pj)
				if err != nil {
					t.Fatal(err)
				}
				_ = pj.RootType()
				PutParser(pj)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestInterfaceMatchesStdlib validates that Interface() produces the same
// output as encoding/json for all test fixtures
// (adapted from minio/simdjson-go FuzzCorrect).
func TestInterfaceMatchesStdlib(t *testing.T) {
	allFiles := []string{
		"apache_builds", "canada", "citm_catalog", "github_events",
		"gsoc-2018", "instruments", "marine_ik", "mesh", "mesh.pretty",
		"numbers", "twitterescaped", "twitter", "random", "update-center",
	}
	for _, name := range allFiles {
		t.Run(name, func(t *testing.T) {
			data := loadTestFile(t, name)

			// Verify encoding/json also accepts it
			var stdlibVal interface{}
			if err := json.Unmarshal(data, &stdlibVal); err != nil {
				t.Fatal(err)
			}

			// simdjson Interface() round-trip
			pj, err := Parse(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			simdjsonVal, err := iter.Interface()
			if err != nil {
				t.Fatal(err)
			}
			simdjsonJSON, err := json.Marshal(simdjsonVal)
			if err != nil {
				t.Fatal(err)
			}

			// Normalize: round-trip both through json.Unmarshal→json.Marshal
			// to equalize number types (int64 vs float64).
			// Note: simdjson parses -0 as int64(0), encoding/json as float64(-0).
			// We normalize by replacing "-0" with "0" in the stdlib output.
			var simdjsonNorm interface{}
			_ = json.Unmarshal(simdjsonJSON, &simdjsonNorm)
			simdjsonRe, _ := json.Marshal(simdjsonNorm)

			var stdlibNorm interface{}
			_ = json.Unmarshal(simdjsonRe, &stdlibNorm)
			normalizedStdlib, _ := json.Marshal(stdlibNorm)

			if string(normalizedStdlib) != string(simdjsonRe) {
				// Show first divergence point
				a, b := string(normalizedStdlib), string(simdjsonRe)
				maxLen := len(a)
				if len(b) < maxLen {
					maxLen = len(b)
				}
				for i := 0; i < maxLen; i++ {
					if a[i] != b[i] {
						start := i - 20
						if start < 0 {
							start = 0
						}
						end := i + 20
						if end > maxLen {
							end = maxLen
						}
						t.Fatalf("mismatch at byte %d:\n  stdlib:  ...%s...\n  simdjson:...%s...",
							i, a[start:end], b[start:end])
						break
					}
				}
				t.Fatalf("length mismatch: stdlib=%d simdjson=%d", len(a), len(b))
			}
		})
	}
}

// BenchmarkInterface benchmarks Interface() on test fixtures.
func BenchmarkInterface(b *testing.B) {
	for _, name := range benchmarkFiles {
		data := loadTestFileB(b, name)
		b.Run(name, func(b *testing.B) {
			pj := GetParser()
			defer PutParser(pj)
			pj, _ = Parse(data, pj)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				iter, _ := pj.Iter()
				_, _ = iter.Interface()
			}
		})
	}
}

// TestFindPathCompat is adapted from minio/simdjson-go TestObject_FindPath.
func TestFindPathCompat(t *testing.T) {
	input := `{
    "Image":
    {
        "Animated": false,
        "Height": 600,
        "IDs": [116, 943, 234, 38793],
        "Thumbnail":
        {
            "Height": 125,
            "Url": "http://www.example.com/image/481989943",
            "Width": 100
        },
        "Title": "View from 15th Floor",
        "Width": 800
    },
    "Alt": "Image of city"
}`
	tests := []struct {
		name     string
		path     []string
		wantType Type
		wantErr  bool
	}{
		{"top", []string{"Alt"}, TypeString, false},
		{"nested-1", []string{"Image", "Animated"}, TypeBool, false},
		{"nested-2", []string{"Image", "Thumbnail", "Url"}, TypeString, false},
		{"int", []string{"Image", "Height"}, TypeInt64, false},
		{"obj", []string{"Image", "Thumbnail"}, TypeObject, false},
		{"array", []string{"Image", "IDs"}, TypeArray, false},
		{"404", []string{"Image", "NonEx"}, TypeNull, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			iter, _ := pj.Iter()
			obj, _ := iter.Object(nil)
			elem, err := obj.FindPath(nil, tt.path...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if elem.Iter.Type() != tt.wantType {
				t.Errorf("want type %v, got %v", tt.wantType, elem.Iter.Type())
			}
		})
	}

	// Value checks
	pj, _ := Parse([]byte(input), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)

	// String value
	elem, _ := obj.FindPath(nil, "Alt")
	v, _ := elem.Iter.String()
	if v != "Image of city" {
		t.Errorf("Alt: want 'Image of city', got %q", v)
	}

	// Bool value
	elem, _ = obj.FindPath(nil, "Image", "Animated")
	b, _ := elem.Iter.Bool()
	if b != false {
		t.Errorf("Animated: want false, got %v", b)
	}

	// Int value
	elem, _ = obj.FindPath(nil, "Image", "Height")
	n, _ := elem.Iter.Int()
	if n != 600 {
		t.Errorf("Height: want 600, got %d", n)
	}

	// Nested string
	elem, _ = obj.FindPath(nil, "Image", "Thumbnail", "Url")
	url, _ := elem.Iter.String()
	if url != "http://www.example.com/image/481989943" {
		t.Errorf("Url: want example.com URL, got %q", url)
	}

	// Array values
	elem, _ = obj.FindPath(nil, "Image", "IDs")
	arr, _ := elem.Iter.Array(nil)
	ids, _ := arr.AsInteger()
	expected := []int64{116, 943, 234, 38793}
	if len(ids) != len(expected) {
		t.Fatalf("IDs: want %v, got %v", expected, ids)
	}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("IDs[%d]: want %d, got %d", i, id, ids[i])
		}
	}
}

func TestParsedJsonForEach(t *testing.T) {
	pj, err := Parse([]byte(`{"a":1}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	called := 0
	err = pj.ForEach(func(i Iter) error {
		called++
		if i.Type() != TypeObject {
			t.Fatalf("expected object, got %v", i.Type())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
}

func TestParsedJsonReset(t *testing.T) {
	pj, _ := Parse([]byte(`[1,2,3]`), nil)
	defer pj.Close()

	pj.Reset()
	_, err := pj.Iter()
	if err == nil {
		t.Fatal("expected error after Reset")
	}

	// Re-parse should work
	pj2, err := Parse([]byte(`{"x":true}`), pj)
	if err != nil {
		t.Fatal(err)
	}
	if pj2.RootType() != TypeObject {
		t.Fatalf("expected object after re-parse, got %v", pj2.RootType())
	}
}

func TestParsedJsonClone(t *testing.T) {
	pj, _ := Parse([]byte(`{"key":"value"}`), nil)
	defer pj.Close()

	clone := pj.Clone(nil)
	// Close original — clone should still work
	pj.Close()

	iter, err := clone.Iter()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := iter.Object(nil)
	if err != nil {
		t.Fatal(err)
	}
	elem := obj.FindKey("key", nil)
	if elem == nil {
		t.Fatal("key not found in clone")
	}
	s, _ := elem.Iter.String()
	if s != "value" {
		t.Fatalf("expected 'value', got %q", s)
	}
}

// TestNumberIsValid validates number parsing edge cases.
// (adapted from minio/simdjson-go TestNumberIsValid)
func TestNumberIsValid(t *testing.T) {
	validTests := []string{
		"0", "-0", "1", "-1", "0.1", "-0.1",
		"1234", "-1234", "12.34", "-12.34",
		"12E0", "12E1", "12e34", "12E-0", "12e+1", "12e-34",
		"-12E0", "-12E1", "-12e34", "-12E-0", "-12e+1", "-12e-34",
		"1.2E0", "1.2E1", "1.2e34", "1.2E-0", "1.2e+1", "1.2e-34",
		"-1.2E0", "-1.2E1", "-1.2e34", "-1.2E-0", "-1.2e+1", "-1.2e-34",
		"0E0", "0E1", "0e34", "0E-0", "0e+1", "0e-34",
		"-0E0", "-0E1", "-0e34", "-0E-0", "-0e+1", "-0e-34",
	}
	for _, num := range validTests {
		pj, err := Parse([]byte(num), nil)
		if err != nil {
			t.Errorf("%s should be valid, got: %v", num, err)
			continue
		}
		pj.Close()
	}

	invalidTests := []string{
		"", "invalid", "1.0.1", "1..1", "-1-2",
		"012a42", "01.2", "012", "12E12.12", "1e2e3",
		"1e+-2", "1e--23", "1e", "e1", "1e+",
		"1ea", "1a", "1.a", "1.", "01", "1.e1",
	}
	for _, num := range invalidTests {
		pj, err := Parse([]byte(num), nil)
		if err == nil {
			pj.Close()
			t.Errorf("%s should be invalid", num)
		}
	}
}

func TestIterAdvance(t *testing.T) {
	pj, _ := Parse([]byte(`[1,"two",true]`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	ai := arr.tarr.Iter()
	it := Iter{tape: ai.tape, tapeIdx: ai.idx, copyStrings: true}

	if it.Type() != TypeInt64 {
		t.Fatalf("expected int64, got %v", it.Type())
	}
	if it.Advance() != TypeString {
		t.Fatal("expected string")
	}
	if it.Advance() != TypeBool {
		t.Fatal("expected bool")
	}
}

func TestIterAdvanceIter(t *testing.T) {
	pj, _ := Parse([]byte(`[10,20]`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	ai := arr.tarr.Iter()
	src := Iter{tape: ai.tape, tapeIdx: ai.idx, copyStrings: true}

	var dst Iter
	typ, err := src.AdvanceIter(&dst)
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeInt64 {
		t.Fatalf("expected int64, got %v", typ)
	}
}

func TestIterPeekNextTag(t *testing.T) {
	pj, _ := Parse([]byte(`[1,"hi"]`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()
	arr, _ := iter.Array(nil)
	ai := arr.tarr.Iter()
	it := Iter{tape: ai.tape, tapeIdx: ai.idx, copyStrings: true}

	tag := it.PeekNextTag()
	if tag != TagString {
		t.Fatalf("expected TagString, got %v", tag)
	}
	// Should not have advanced
	if it.Type() != TypeInt64 {
		t.Fatal("should still be at int64")
	}
}

func TestIterRoot(t *testing.T) {
	pj, _ := Parse([]byte(`{"a":1}`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()

	var dst Iter
	typ, got, err := iter.Root(&dst)
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeObject {
		t.Fatalf("expected object, got %v", typ)
	}
	if got == nil {
		t.Fatal("dst is nil")
	}
}

func TestIterFloatFlags(t *testing.T) {
	pj, _ := Parse([]byte(`3.14`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()

	v, flags, err := iter.FloatFlags()
	if err != nil {
		t.Fatal(err)
	}
	if v != 3.14 {
		t.Fatalf("expected 3.14, got %v", v)
	}
	if flags.Contains(FloatOverflowedInteger) {
		t.Fatal("should not have overflow flag")
	}
}

func TestIterFloatFlagsFromInt(t *testing.T) {
	pj, _ := Parse([]byte(`42`), nil)
	defer pj.Close()
	iter, _ := pj.Iter()

	v, _, err := iter.FloatFlags()
	if err != nil {
		t.Fatal(err)
	}
	if v != 42.0 {
		t.Fatalf("expected 42, got %v", v)
	}
}

func TestTagType(t *testing.T) {
	if TagString.Type() != TypeString {
		t.Fatal("TagString.Type() mismatch")
	}
	if TagBoolFalse.Type() != TypeBool {
		t.Fatal("TagBoolFalse.Type() mismatch")
	}
	if TagEnd.Type() != Type(-1) {
		t.Fatal("TagEnd.Type() mismatch")
	}
}
