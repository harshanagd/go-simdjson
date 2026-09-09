// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"errors"
	"reflect"
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

// --- Reading every document on an NDJSON tape ---
//
// ParseND builds one combined tape holding every document, but nothing could read
// past the first: ParsedJson.ForEach called fn exactly once (`return fn(iter)`)
// despite documenting "for each root element", and a manual Advance walk landed on
// the next document's root marker, where Interface() failed with
// "unknown tag 'r'". simdjson_parse_many gives each document its own block —
// opening root, value, closing root, trailing zero word — so both walks have to
// step over that scaffolding.

// --- Shared scaffolding for the root-document walk tests ---

// collectViaForEach gathers every document ParsedJson.ForEach yields.
func collectViaForEach(t *testing.T, pj *ParsedJson) []interface{} {
	t.Helper()
	var got []interface{}
	if err := pj.ForEach(func(i Iter) error {
		v, err := i.Interface()
		if err != nil {
			return err
		}
		got = append(got, v)
		return nil
	}); err != nil {
		t.Fatalf("ForEach: %v", err)
	}
	return got
}

// collectViaAdvance gathers every document reachable by stepping a cursor with
// Advance. This is the other route across a document boundary, and it exercises
// different code from ForEach — worth keeping both.
func collectViaAdvance(t *testing.T, pj *ParsedJson, maxSteps int) []interface{} {
	t.Helper()
	it, err := pj.Iter()
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	var got []interface{}
	for step := 0; step < maxSteps; step++ {
		v, err := it.Interface()
		if err != nil {
			t.Fatalf("manual step %d landed on a non-value: %v", step, err)
		}
		got = append(got, v)
		if it.Advance() == Type(-1) {
			break
		}
	}
	return got
}

// rootBlock builds one CLEAN root document block for a single-entry value, in the
// layout simdjson_parse_many emits:
//
//	[start]   tagRoot  payload = index of the padding word (one past the closing root)
//	[start+1] the value tag
//	[start+2] the value word
//	[start+3] tagRoot  payload = start
//	[start+4] padding — simdjson never writes this
//
// Only for blocks that are meant to be well formed. Tests that deliberately
// corrupt the padding or truncate a block build their tapes inline, because the
// whole point there is to deviate from this shape.
func rootBlock(start int, valueTag byte, valueWord uint64) []uint64 {
	return []uint64{
		uint64(tagRoot)<<56 | uint64(start+4),
		uint64(valueTag) << 56,
		valueWord,
		uint64(tagRoot)<<56 | uint64(start),
		0,
	}
}

func TestParseNDForEachVisitsEveryDocument(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []interface{}
	}{
		{
			name:  "objects",
			input: "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n",
			want: []interface{}{
				map[string]interface{}{"n": int64(1)},
				map[string]interface{}{"n": int64(2)},
				map[string]interface{}{"n": int64(3)},
			},
		},
		{
			// Scalars are the tighter case: their blocks are 5 words rather than 8,
			// so a hardcoded stride would pass the object case and fail here.
			name:  "scalars",
			input: "1\n2\n3\n",
			want:  []interface{}{int64(1), int64(2), int64(3)},
		},
		{
			name:  "arrays",
			input: "[1]\n[2]\n",
			want:  []interface{}{[]interface{}{int64(1)}, []interface{}{int64(2)}},
		},
		{
			name:  "single_document",
			input: "{\"n\":1}\n",
			want:  []interface{}{map[string]interface{}{"n": int64(1)}},
		},
		{
			name:  "mixed_shapes",
			input: "{\"n\":1}\n[2]\n3\n\"four\"\ntrue\nnull\n",
			want: []interface{}{
				map[string]interface{}{"n": int64(1)},
				[]interface{}{int64(2)},
				int64(3),
				"four",
				true,
				nil,
			},
		},
		{
			// Empty containers are the tightest blocks of all — the value occupies
			// a single entry pair with no children, so a boundary skip that is even
			// one entry out lands on the next document's root marker.
			name:  "empty_containers",
			input: "{}\n[]\n{}\n",
			want: []interface{}{
				map[string]interface{}{},
				[]interface{}{},
				map[string]interface{}{},
			},
		},
		{
			name:  "empty_then_populated",
			input: "[]\n[1,2]\n{}\n{\"k\":\"v\"}\n",
			want: []interface{}{
				[]interface{}{},
				[]interface{}{int64(1), int64(2)},
				map[string]interface{}{},
				map[string]interface{}{"k": "v"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pj, err := ParseND([]byte(tt.input), nil)
			if err != nil {
				t.Fatalf("ParseND: %v", err)
			}
			defer pj.Close()

			got := collectViaForEach(t, pj)
			if len(got) != len(tt.want) {
				t.Fatalf("ForEach visited %d documents (%#v), want %d", len(got), got, len(tt.want))
			}
			for i := range tt.want {
				if !reflect.DeepEqual(got[i], tt.want[i]) {
					t.Errorf("document %d = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestParseNDManualAdvanceWalk covers the other route across a document boundary:
// a cursor stepping with Advance. It previously landed on the next document's root
// marker, whose Type() is unknown(114) and which every accessor rejects.
func TestParseNDManualAdvanceWalk(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  []int64
	}{
		{"scalars", "1\n2\n3\n", []int64{1, 2, 3}},
		{"single", "7\n", []int64{7}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pj, err := ParseND([]byte(tt.input), nil)
			if err != nil {
				t.Fatalf("ParseND: %v", err)
			}
			defer pj.Close()

			it, err := pj.Iter()
			if err != nil {
				t.Fatalf("Iter: %v", err)
			}
			var got []int64
			for step := 0; step < 16; step++ {
				v, err := it.Int()
				if err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
				got = append(got, v)
				if it.Advance() == Type(-1) {
					break
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("manual walk got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("manual walk got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestParseNDForEachStopsOnError confirms fn's error aborts the walk rather than
// being swallowed or continuing to the remaining documents.
func TestParseNDForEachStopsOnError(t *testing.T) {
	pj, err := ParseND([]byte("1\n2\n3\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	want := errors.New("stop here")
	calls := 0
	got := pj.ForEach(func(i Iter) error {
		calls++
		if calls == 2 {
			return want
		}
		return nil
	})
	if !errors.Is(got, want) {
		t.Errorf("ForEach returned %v, want %v", got, want)
	}
	if calls != 2 {
		t.Errorf("fn called %d times, want 2 (iteration must stop at the error)", calls)
	}
}

// TestForEachWithoutTape covers the guard ForEach gained when it stopped
// delegating to Iter(), which previously produced the error for it.
func TestForEachWithoutTape(t *testing.T) {
	pj := GetParser()
	defer pj.Close()
	if err := pj.ForEach(func(i Iter) error {
		t.Error("fn was called for a parser holding no document")
		return nil
	}); err == nil {
		t.Error("ForEach on a parser with no tape returned no error")
	}
}

// TestNDJSONWalksIgnoreUninitialisedPadding is the regression test for the trap
// that made the walk above order-dependent.
//
// Each root document block ends with a word that tape_len counts — so Parse
// copies it — but that simdjson never writes. The C++ parser does not zero its
// tape between parses and the allocator hands a fresh parser previously-used
// memory, so that word holds arbitrary leftovers. Observed in practice:
//
//	fresh buffer:  [0]r:4 [1]l [2]=7 [3]r:0 [4]<zero>
//	reused buffer: [0]r:4 [1]l [2]=7 [3]r:0 [4]']' payload 1
//
// A boundary skip that inferred structure from that word therefore worked only on
// a fresh parser. The walk must use the root payload chain alone. This builds the
// dirty layout directly rather than trying to provoke the allocator.
func TestNDJSONWalksIgnoreUninitialisedPadding(t *testing.T) {
	// Two single-scalar documents, with padding words holding tags that a
	// content-sniffing skip would stop on.
	tape := &Tape{data: []uint64{
		uint64(tagRoot)<<56 | 4,   // [0] opening root, block ends at 4
		uint64(tagInt64) << 56,    // [1] value tag
		1,                         // [2] value word
		uint64(tagRoot) << 56,     // [3] closing root -> opening root 0
		uint64(tagArrEnd)<<56 | 1, // [4] PADDING: never written by simdjson
		uint64(tagRoot)<<56 | 9,   // [5] opening root, block ends at 9
		uint64(tagInt64) << 56,    // [6] value tag
		2,                         // [7] value word
		uint64(tagRoot)<<56 | 5,   // [8] closing root -> opening root 5
		uint64(tagObject)<<56 | 7, // [9] PADDING: never written by simdjson
	}}
	pj := &ParsedJson{tape: *tape, hasTape: true, copyStrings: true}

	// ForEach must visit both documents.
	want := []interface{}{int64(1), int64(2)}
	if got := collectViaForEach(t, pj); !reflect.DeepEqual(got, want) {
		t.Errorf("ForEach over a tape with dirty padding = %#v, want %#v", got, want)
	}
	// The manual Advance walk is the other route across the boundary.
	if got := collectViaAdvance(t, pj, 8); !reflect.DeepEqual(got, want) {
		t.Errorf("manual walk over a tape with dirty padding = %#v, want %#v", got, want)
	}
}

// TestParseNDWalkAfterOtherParses is the realistic reproduction: earlier parses
// dirty the heap the next parser's tape is allocated from. It cannot force the
// condition, so the deterministic cover is the test above — this guards the real
// path and would have caught the bug when run after the rest of the suite.
func TestParseNDWalkAfterOtherParses(t *testing.T) {
	for _, warm := range []string{
		"{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n",
		"[1]\n[2]\n",
		"{\"a\":{\"b\":[1,2,3]}}\n",
	} {
		pj, err := ParseND([]byte(warm), nil)
		if err != nil {
			t.Fatalf("warm-up ParseND: %v", err)
		}
		_ = pj.ForEach(func(i Iter) error { _, e := i.Interface(); return e })
		pj.Close()
	}

	pj, err := ParseND([]byte("1\n2\n3\n"), nil)
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	got := collectViaAdvance(t, pj, 8)
	want := []interface{}{int64(1), int64(2), int64(3)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("walk after warm-up parses = %#v, want %#v", got, want)
	}
}

// TestParseNDForEachPropagatesOptions confirms the Iter that ForEach hands to fn
// carries the parser's options. ForEach used to delegate to pj.Iter(), which set
// them; now it builds each Iter itself, so they have to be copied explicitly —
// easy to drop and silent if dropped, since the values would just come back as
// native types instead of json.Number.
func TestParseNDForEachPropagatesOptions(t *testing.T) {
	pj, err := ParseND([]byte("1\n2\n3\n"), nil, UseNumber())
	if err != nil {
		t.Fatalf("ParseND: %v", err)
	}
	defer pj.Close()

	docs := 0
	if err := pj.ForEach(func(i Iter) error {
		docs++
		v, err := i.Interface()
		if err != nil {
			return err
		}
		n, ok := v.(json.Number)
		if !ok {
			t.Errorf("document %d = %T (%#v), want json.Number — UseNumber not propagated", docs, v, v)
			return nil
		}
		if want := []string{"1", "2", "3"}[docs-1]; string(n) != want {
			t.Errorf("document %d = %q, want %q", docs, string(n), want)
		}
		return nil
	}); err != nil {
		t.Fatalf("ForEach: %v", err)
	}
	if docs != 3 {
		t.Errorf("ForEach visited %d documents, want 3", docs)
	}

	// WithCopyStrings(false) must reach fn's Iter too. Asserting cap==len would be
	// useless: BOTH modes satisfy it — the no-copy path returns a cap-pinned
	// sub-slice and the copy path returns a make()'d slice whose cap equals its
	// len. The discriminating property is ALIASING, so mutate the tape's string
	// buffer and see whether the returned slice observes the change.
	for _, copyStrings := range []bool{false, true} {
		name := "nocopy"
		if copyStrings {
			name = "copy"
		}
		t.Run(name, func(t *testing.T) {
			pj, err := ParseND([]byte("\"aaaa\"\n\"bbbb\"\n"), nil, WithCopyStrings(copyStrings))
			if err != nil {
				t.Fatalf("ParseND: %v", err)
			}
			defer pj.Close()

			var got [][]byte
			if err := pj.ForEach(func(i Iter) error {
				b, err := i.StringBytes()
				if err != nil {
					return err
				}
				got = append(got, b)
				return nil
			}); err != nil {
				t.Fatalf("ForEach: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("ForEach visited %d documents, want 2", len(got))
			}
			if string(got[0]) != "aaaa" || string(got[1]) != "bbbb" {
				t.Fatalf("read %q and %q, want \"aaaa\" and \"bbbb\"", got[0], got[1])
			}

			// Overwrite every 'a' in the tape's own buffer.
			tape, err := pj.GetTape()
			if err != nil {
				t.Fatalf("GetTape: %v", err)
			}
			for i := range tape.strings {
				if tape.strings[i] == 'a' {
					tape.strings[i] = 'z'
				}
			}

			aliased := string(got[0]) != "aaaa"
			if aliased == copyStrings {
				t.Errorf("WithCopyStrings(%v): slice aliases the tape buffer = %v; the option did not reach ForEach's Iter",
					copyStrings, aliased)
			}
		})
	}
}

// TestNDJSONForEachTruncatedTrailingBlock pins the behaviour for a tape whose
// last root marker has no value after it — a document truncated mid-block.
//
// This is the case ForEach used to handle with a mid-loop `if it.tapeIdx >=
// len(t.data) { break }`. That check is gone: nextRootDoc now only yields indices
// that have a value after them, so the invariant lives in one place instead of
// being re-checked inside the loop. The observable behaviour is identical, which
// is what this test locks — it is a refactor, not a fix.
//
// Note what that behaviour IS: the complete documents are delivered and the
// truncated trailing block is silently skipped. That is the same silent-truncation
// family as ParseND dropping a malformed final document, and is better addressed
// at parse time than here.
func TestNDJSONForEachTruncatedTrailingBlock(t *testing.T) {
	// One complete document, then a bare root marker with nothing after it.
	data := rootBlock(0, tagInt64, 1)
	data = append(data, uint64(tagRoot)<<56) // [5] TRUNCATED: marker, no value after
	tape := &Tape{data: data}
	pj := &ParsedJson{tape: *tape, hasTape: true, copyStrings: true}

	want := []interface{}{int64(1)}
	if got := collectViaForEach(t, pj); !reflect.DeepEqual(got, want) {
		t.Errorf("ForEach = %#v, want %#v (complete document only)", got, want)
	}

	// nextRootDoc must refuse the truncated block rather than returning index 5,
	// which is what makes the removed bound check unnecessary.
	if next := tape.nextRootDoc(0); next != len(tape.data) {
		t.Errorf("nextRootDoc(0) = %d, want %d (must not yield a valueless root)", next, len(tape.data))
	}
	if tape.hasRootAt(5) != true {
		t.Error("hasRootAt(5) should still recognise the marker")
	}
	if tape.hasRootDocAt(5) != false {
		t.Error("hasRootDocAt(5) should reject a marker with no value")
	}
}

// TestParseNDTruncatedFinalDocument covers silent data loss on a truncated stream.
//
// parse_many is a streaming API: an INCOMPLETE document at the end of the buffer
// is held over for the next batch rather than reported, on the assumption the rest
// arrives later. ParseND is handed the whole input at once, so held-over bytes
// mean malformed input — but it reported success and returned a tape missing that
// document. `{"a":1}\n{invalid\n` yielded one record and a nil error.
//
// Note this only affected the FINAL document. A malformed document followed by a
// valid one always errored, which is why TestParseNDInvalidLine passed: its
// `{invalid}` is a COMPLETE document that fails to parse, not a truncated one.
func TestParseNDTruncatedFinalDocument(t *testing.T) {
	t.Run("truncated_is_rejected", func(t *testing.T) {
		for _, input := range []string{
			"{\"a\":1}\n{invalid\n",  // valid document then a truncated one
			"{invalid\n",             // truncated only
			"1\n2\n{\"unclosed\":\n", // scalars then a truncated object
			"[1,2\n",                 // unclosed array
			"{\"a\":1}\n\"unterminated\n",
		} {
			pj, err := ParseND([]byte(input), nil)
			if err == nil {
				docs := 0
				_ = pj.ForEach(func(i Iter) error { docs++; return nil })
				t.Errorf("ParseND(%q) returned no error (yielded %d documents)", input, docs)
			}
			// Per the error contract, the parser comes back non-nil with no tape.
			if pj == nil {
				t.Errorf("ParseND(%q) returned a nil parser, discarding the caller's", input)
				continue
			}
			if _, iterErr := pj.Iter(); iterErr == nil {
				t.Errorf("ParseND(%q) left a readable tape after an error", input)
			}
			pj.Close()
		}
	})

	t.Run("complete_input_is_accepted", func(t *testing.T) {
		// The false-positive risk is the whole risk here: a check that is too eager
		// rejects valid input. These are the shapes most likely to trip it — in
		// particular a BARE TRAILING SCALAR, which unlike `}` or `]` has no
		// self-terminating character, so a naive "did the stream end mid-token"
		// test would flag it.
		for _, tc := range []struct {
			input string
			docs  int
		}{
			{"{\"a\":1}\n{\"b\":2}\n", 2},
			{"1\n2\n3\n", 3},
			{"[1,2]\n[3]\n", 2},
			{"{\"a\":1}", 1}, // no trailing newline
			{"{}\n[]\n", 2},
			// Bare trailing scalars, no terminator of any kind.
			{"42", 1},
			{"true", 1},
			{"false", 1},
			{"null", 1},
			{"3.14", 1},
			{"\"str\"", 1},
			{"1\n2", 2}, // last scalar unterminated
			// A review of this change predicted these would be false-positived,
			// on the reasoning that a trailing number has no self-terminating
			// character. They are not: simdjson::padded_string appends
			// SIMDJSON_PADDING zero bytes, which terminates the final token, so
			// truncated_bytes() is 0. Pinned so the prediction stays disproved.
			{"1\n2\n3", 3},
			{"-17", 1},
			{"1e5", 1},
			{"{\"a\":1}\n42", 2},
			{"[1]\n2", 2},
			{"18446744073709551615", 1},
			// Blank and whitespace-only lines survive the leading/trailing
			// TrimSpace and must not be read as truncation.
			{"{\"a\":1}\n\n{\"b\":2}\n", 2},
			{"{\"a\":1}\n   \n{\"b\":2}\n", 2},
			// CRLF line endings.
			{"{\"a\":1}\r\n{\"b\":2}\r\n", 2},
			// An escaped newline inside a string is not a record separator.
			{"{\"s\":\"has\\nescaped\"}\n{\"t\":2}\n", 2},
		} {
			pj, err := ParseND([]byte(tc.input), nil)
			if err != nil {
				t.Errorf("ParseND(%q) = %v, want nil", tc.input, err)
				if pj != nil {
					pj.Close()
				}
				continue
			}
			docs := 0
			if err := pj.ForEach(func(i Iter) error { docs++; return nil }); err != nil {
				t.Errorf("ParseND(%q) ForEach: %v", tc.input, err)
			}
			if docs != tc.docs {
				t.Errorf("ParseND(%q) yielded %d documents, want %d", tc.input, docs, tc.docs)
			}
			pj.Close()
		}
	})

	t.Run("malformed_mid_stream_still_errors_distinctly", func(t *testing.T) {
		// A complete-but-invalid document mid-stream is a parse error, not a
		// truncation — the two must not be conflated.
		pj, err := ParseND([]byte("{\"a\":1}\n{bad}\n{\"c\":3}\n"), nil)
		if err == nil {
			t.Fatal("ParseND accepted a malformed mid-stream document")
		}
		if strings.Contains(err.Error(), "truncated") {
			t.Errorf("mid-stream parse failure reported as truncation: %v", err)
		}
		pj.Close()
	})
}
