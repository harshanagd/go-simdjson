// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// ParseND parses newline-delimited JSON (ndjson). Each line is a separate
// JSON value. The returned ParsedJson contains all values accessible via
// ForEach. Empty lines are skipped.
func ParseND(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	// For single-line input, just parse normally.
	if bytes.IndexByte(b, '\n') < 0 {
		return Parse(b, reuse, opts...)
	}

	// Multi-line: parse each line and concatenate tapes.
	// The result has multiple root entries in the tape.
	var combined Tape
	var copyStrings, useNumber bool
	if reuse != nil {
		copyStrings = reuse.copyStrings
		useNumber = reuse.useNumber
	} else {
		copyStrings = true
	}
	for _, opt := range opts {
		tmp := &ParsedJson{copyStrings: copyStrings, useNumber: useNumber}
		opt(tmp)
		copyStrings = tmp.copyStrings
		useNumber = tmp.useNumber
	}

	lines := bytes.Split(b, []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		pj, err := Parse(line, nil, opts...)
		if err != nil {
			return nil, fmt.Errorf("line %q: %w", truncate(line, 50), err)
		}
		// Append this line's tape (skip outer root entries)
		combined.data = append(combined.data, pj.tape.data...)
		// Adjust string buffer offsets — strings from this line start at
		// the current end of the combined string buffer.
		combined.strings = append(combined.strings, pj.tape.strings...)
		pj.Close()
	}

	combined.copyStrings = copyStrings

	if reuse == nil {
		reuse = newParsedJson()
	}
	for _, opt := range opts {
		opt(reuse)
	}
	reuse.tape = combined
	reuse.hasTape = true
	return reuse, nil
}

// Stream is used to stream back results from ParseNDStream.
type Stream struct {
	Value *ParsedJson
	Error error
}

// ParseNDStream parses newline-delimited JSON from an io.Reader, sending
// results to the res channel. An optional reuse channel can supply ParsedJson
// instances for reuse. The method returns immediately; parsing happens in a
// goroutine. The res channel is closed when parsing is complete.
func ParseNDStream(r io.Reader, res chan<- Stream, reuse <-chan *ParsedJson) {
	go func() {
		defer close(res)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 10<<20), 10<<20) // 10MB max line
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			// Try to get a reusable ParsedJson
			var pj *ParsedJson
			if reuse != nil {
				select {
				case pj = <-reuse:
				default:
				}
			}
			data := make([]byte, len(line))
			copy(data, line)
			var err error
			pj, err = Parse(data, pj)
			if err != nil {
				res <- Stream{Error: fmt.Errorf("line %q: %w", truncate(line, 50), err)}
				return
			}
			res <- Stream{Value: pj}
		}
		if err := scanner.Err(); err != nil {
			res <- Stream{Error: err}
		}
	}()
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
