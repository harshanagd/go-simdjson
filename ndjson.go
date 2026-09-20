// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

// #include "bridge.h"
import "C"

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"unsafe"
)

// ParseND parses newline-delimited JSON (ndjson) using C++ simdjson's
// parse_many for SIMD-accelerated batch processing. Each line is a separate
// JSON value. The returned ParsedJson contains all values in a combined tape.
//
// As with Parse, parser options apply only to this call, and on error the
// returned ParsedJson is non-nil and holds no tape.
func ParseND(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error) {
	b = bytes.TrimSpace(b)

	pj := reuse
	if pj == nil {
		pj = newParsedJson()
	}
	// Reset options to their defaults before applying opts — see Parse.
	pj.copyStrings = true
	pj.useNumber = false
	pj.bigInt = false
	for _, opt := range opts {
		opt(pj)
	}

	if len(b) == 0 {
		pj.hasTape = false
		pj.tape = Tape{}
		return pj, fmt.Errorf("empty input")
	}

	res := C.simdjson_parse_many(pj.parser, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
	if res.result.ok == 0 {
		pj.hasTape = false
		pj.tape = Tape{}
		return pj, fmt.Errorf("%s", C.GoString(res.result.error_msg))
	}
	// parse_many is a streaming API: an incomplete document at the end of the
	// buffer is held over for the next batch rather than reported as an error.
	// ParseND is given the whole input at once, so held-over bytes mean the input
	// is malformed or truncated — reporting success here would silently drop that
	// document. Errors ARE raised normally when a malformed document is followed
	// by a valid one; only the final one goes quiet.
	if truncated := int(res.truncated_bytes); truncated > 0 {
		pj.hasTape = false
		pj.tape = Tape{}
		return pj, fmt.Errorf("truncated NDJSON: the last %d byte(s) do not form a complete document", truncated)
	}
	pj.tape = Tape{
		data:        unsafe.Slice((*uint64)(unsafe.Pointer(res.tape)), int(res.tape_len)),
		strings:     unsafe.Slice((*byte)(unsafe.Pointer(res.sbuf)), int(res.sbuf_len)),
		copyStrings: pj.copyStrings,
		useNumber:   pj.useNumber,
		pj:          pj,
	}
	pj.hasTape = true
	return pj, nil
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
//
// Each Stream.Value holds a view into its own parser, so read a document before
// putting its ParsedJson on the reuse channel: reusing it re-parses over the memory
// the previous view pointed at. Clone to keep a document past that point.
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
