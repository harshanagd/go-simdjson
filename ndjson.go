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
func ParseND(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	pj := reuse
	if pj == nil {
		pj = newParsedJson()
	}
	for _, opt := range opts {
		opt(pj)
	}

	res := C.simdjson_parse_many(pj.parser, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
	if res.result.ok == 0 {
		return nil, fmt.Errorf("%s", C.GoString(res.result.error_msg))
	}
	pj.tape = Tape{
		data:        copyUint64Slice(uintptr(unsafe.Pointer(res.tape)), int(res.tape_len)),
		strings:     copyByteSlice(uintptr(unsafe.Pointer(res.sbuf)), int(res.sbuf_len)),
		copyStrings: pj.copyStrings,
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
