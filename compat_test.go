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
