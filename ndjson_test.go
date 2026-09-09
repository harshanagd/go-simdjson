// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseND(t *testing.T) {
	input := []byte("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n")
	pj, err := ParseND(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()
	if !pj.hasTape {
		t.Fatal("no tape")
	}
}

func TestParseNDSingleLine(t *testing.T) {
	input := []byte(`{"a":1}`)
	pj, err := ParseND(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()
	if pj.RootType() != TypeObject {
		t.Errorf("root type = %v, want object", pj.RootType())
	}
}

func TestParseNDEmpty(t *testing.T) {
	_, err := ParseND([]byte(""), nil)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseNDBlankLines(t *testing.T) {
	input := []byte("{\"a\":1}\n\n\n{\"b\":2}\n\n")
	pj, err := ParseND(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()
}

func TestParseNDInvalidLine(t *testing.T) {
	input := []byte("{\"a\":1}\n{invalid}\n")
	_, err := ParseND(input, nil)
	if err == nil {
		t.Error("expected error for invalid line")
	}
}

func TestParseNDStream(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"
	res := make(chan Stream, 10)
	ParseNDStream(strings.NewReader(input), res, nil)

	count := 0
	for s := range res {
		if s.Error != nil {
			t.Fatal(s.Error)
		}
		if s.Value.RootType() != TypeObject {
			t.Errorf("line %d: root type = %v", count, s.Value.RootType())
		}
		s.Value.Close()
		count++
	}
	if count != 3 {
		t.Errorf("got %d results, want 3", count)
	}
}

func TestParseNDStreamReuse(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n"
	res := make(chan Stream, 10)
	reuse := make(chan *ParsedJson, 2)
	ParseNDStream(strings.NewReader(input), res, reuse)

	for s := range res {
		if s.Error != nil {
			t.Fatal(s.Error)
		}
		// Return to reuse channel
		reuse <- s.Value
	}
}

func TestParseNDStreamEmpty(t *testing.T) {
	res := make(chan Stream, 10)
	ParseNDStream(strings.NewReader(""), res, nil)

	count := 0
	for s := range res {
		if s.Error != nil {
			t.Fatal(s.Error)
		}
		count++
	}
	if count != 0 {
		t.Errorf("got %d results, want 0", count)
	}
}

func TestParseNDStreamInvalid(t *testing.T) {
	input := "{\"a\":1}\n{bad}\n"
	res := make(chan Stream, 10)
	ParseNDStream(strings.NewReader(input), res, nil)

	gotError := false
	for s := range res {
		if s.Error != nil {
			gotError = true
		} else {
			s.Value.Close()
		}
	}
	if !gotError {
		t.Error("expected error for invalid line")
	}
}

func TestParseNDRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "multi_object",
			input: "{\"three\":true,\"two\":\"foo\",\"one\":-1}\n" +
				"{\"three\":false,\"two\":\"bar\",\"one\":null}\n" +
				"{\"three\":true,\"two\":\"baz\",\"one\":2.5}",
		},
		{
			name:  "single_object",
			input: `{"bimbam":12345465.447,"bumbum":true,"istrue":true,"isfalse":false,"aap":null}`,
		},
		{
			name:  "empty_object",
			input: `{}`,
		},
		{
			name:    "empty_input",
			input:   ``,
			wantErr: true,
		},
		{
			name: "escaped_strings",
			input: "{\"body\":\"say \\\"hello\\\"\"}\n" +
				"{\"body\":\"line1\\nline2\"}",
		},
		{
			name: "mixed_types",
			input: "{\"a\":1,\"b\":null,\"c\":true,\"d\":\"str\"}\n" +
				"[1,2,3]\n" +
				"{\"nested\":{\"x\":[4,5]}}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pj, err := ParseND([]byte(tc.input), nil)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer pj.Close()

			// Verify each line parses individually and matches
			lines := strings.Split(strings.TrimSpace(tc.input), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				lpj, err := Parse([]byte(line), nil)
				if err != nil {
					t.Fatalf("individual parse failed: %v", err)
				}
				iter, _ := lpj.Iter()
				val, _ := iter.Interface()
				if val == nil {
					t.Errorf("nil interface for line: %s", line)
				}
				lpj.Close()
			}
		})
	}
}

func TestParseNDStreamForEach(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"
	res := make(chan Stream, 10)
	ParseNDStream(strings.NewReader(input), res, nil)

	keys := []string{}
	for s := range res {
		if s.Error != nil {
			t.Fatal(s.Error)
		}
		iter, _ := s.Value.Iter()
		obj, _ := iter.Object(nil)
		var name string
		name, _, _ = obj.NextElement(nil)
		keys = append(keys, name)
		s.Value.Close()
	}
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("keys = %v, want [a b c]", keys)
	}
}

func TestParseNDStreamLargeLines(t *testing.T) {
	// Build a line with a large string value
	big := strings.Repeat("x", 10000)
	input := "{\"v\":\"" + big + "\"}\n{\"v\":\"small\"}\n"
	res := make(chan Stream, 10)
	ParseNDStream(strings.NewReader(input), res, nil)

	count := 0
	for s := range res {
		if s.Error != nil {
			t.Fatal(s.Error)
		}
		s.Value.Close()
		count++
	}
	if count != 2 {
		t.Errorf("got %d, want 2", count)
	}
}

func BenchmarkParseND(b *testing.B) {
	// Build NDJSON from twitter.json — repeat the object 10 times
	line := loadTestFileB(b, "twitter")
	var buf []byte
	for i := 0; i < 10; i++ {
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pj, err := ParseND(buf, nil)
		if err != nil {
			b.Fatal(err)
		}
		pj.Close()
	}
}

// --- #6: ParseND returns the caller's parser on error and clears its tape ---

func TestParseNDErrorReturnsParser(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	got, err := ParseND(nil, pj)
	if err == nil {
		t.Fatal("expected an error for empty ndjson input")
	}
	if got == nil {
		t.Fatal("ParseND returned nil for empty input, discarding the caller's parser")
	}
	if got.hasTape {
		t.Fatal("empty input left a tape in place")
	}

	// Seed a good parse, then fail on the same parser: the tape must be cleared
	// rather than left readable.
	if _, err = ParseND([]byte(`{"a":1}`+"\n"), pj); err != nil {
		t.Fatal(err)
	}
	got, err = ParseND([]byte("not json at all\n"), pj)
	if err == nil {
		t.Fatal("expected an error for malformed ndjson")
	}
	if got == nil {
		t.Fatal("ParseND returned nil on error, discarding the caller's parser")
	}
	if got.hasTape {
		t.Fatal("failed ParseND left hasTape set, exposing the previous document")
	}
}

// --- #7 / #8: ParseND resets options and threads useNumber into the tape ---

func TestParseNDOptionsDoNotLeakAcrossReuse(t *testing.T) {
	pj := GetParser()
	defer PutParser(pj)

	input := []byte(`{"n":42}` + "\n")

	pj, err := ParseND(input, pj, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	v, err := pj.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(map[string]interface{})["n"].(json.Number); !ok {
		t.Fatalf("UseNumber not applied by ParseND: got %T", v.(map[string]interface{})["n"])
	}

	// Second call passes no options; the first call's must not persist.
	pj, err = ParseND(input, pj)
	if err != nil {
		t.Fatal(err)
	}
	v, err = pj.TapeInterface()
	if err != nil {
		t.Fatal(err)
	}
	n := v.(map[string]interface{})["n"]
	if _, isNum := n.(json.Number); isNum {
		t.Fatal("UseNumber leaked into a later ParseND that passed no options")
	}
	if _, ok := n.(int64); !ok {
		t.Fatalf("expected int64 without UseNumber, got %T", n)
	}
}

func TestParseNDUseNumberAppliesToTapeAPI(t *testing.T) {
	pj, err := ParseND([]byte(`{"n":42}`+"\n"+`{"m":7}`+"\n"), nil, UseNumber())
	if err != nil {
		t.Fatal(err)
	}
	defer pj.Close()

	tape, err := pj.GetTape()
	if err != nil {
		t.Fatal(err)
	}

	ti := tape.Iter()
	v, err := ti.Interface()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected an object, got %T", v)
	}
	if _, ok := m["n"].(json.Number); !ok {
		t.Errorf("ParseND did not thread useNumber into the tape: got %T", m["n"])
	}
}
