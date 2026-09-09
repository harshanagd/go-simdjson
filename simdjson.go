// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

// Package simdjson provides Go bindings for the simdjson C++ library via CGo.
// It supports x86_64 (AVX2/SSE4.2) and ARM64 (NEON) with automatic runtime detection.
package simdjson

// #cgo CXXFLAGS: -std=c++17 -O2 -DNDEBUG
// #cgo LDFLAGS: -lstdc++ -lm
// #include "bridge.h"
// #include <string.h>
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// ParsedJson holds a parsed JSON document. Safe to reuse via sync.Pool.
type ParsedJson struct {
	parser      C.simdjson_parser
	tape        Tape
	hasTape     bool
	copyStrings bool
	useNumber   bool
	bigInt      bool
}

// ParserOption configures parsing behavior.
type ParserOption func(*ParsedJson)

// WithCopyStrings controls whether each string value is copied out of the tape's
// string buffer when it is read.
//
// Note both settings are safe to retain: Parse copies the C++ string buffer into
// Go-managed memory, so a returned string is never invalidated by a later Parse or
// Close. The flag only controls whether each read allocates a fresh copy or
// aliases the tape's buffer, which matters for []byte results — see StringBytes.
// When true (the default), every string read allocates.
func WithCopyStrings(copy bool) ParserOption {
	return func(pj *ParsedJson) {
		pj.copyStrings = copy
	}
}

// UseNumber causes Interface() to return json.Number instead of int64/uint64/float64
// for numeric values, preserving the exact string representation.
// Big integers (exceeding uint64 range) are also returned as json.Number.
func UseNumber() ParserOption {
	return func(pj *ParsedJson) {
		pj.useNumber = true
		pj.bigInt = true
	}
}

// UseBigInt enables parsing of integers that exceed the 64-bit range.
// Big integers are returned as json.Number by Interface().
// Normal int64/uint64/float64 values are unaffected.
func UseBigInt() ParserOption {
	return func(pj *ParsedJson) {
		pj.bigInt = true
	}
}

func newParsedJson() *ParsedJson {
	pj := &ParsedJson{parser: C.simdjson_parser_new(), copyStrings: true}
	runtime.SetFinalizer(pj, func(p *ParsedJson) {
		p.Close()
	})
	return pj
}

// parserPool pools C++ parser instances to reduce allocations.
var parserPool = sync.Pool{
	New: func() interface{} {
		return newParsedJson()
	},
}

// GetParser returns a ParsedJson from the pool. Call PutParser when done.
func GetParser() *ParsedJson {
	return parserPool.Get().(*ParsedJson)
}

// PutParser returns a ParsedJson to the pool for reuse.
// The parser is reset first, so it does not carry its tape or its parser
// options over to the next GetParser caller.
func PutParser(pj *ParsedJson) {
	if pj != nil && pj.parser != nil {
		pj.Reset()
		parserPool.Put(pj)
	}
}

// Parse parses JSON bytes using the provided ParsedJson (or a new one if nil).
// The returned ParsedJson owns the parsed data until the next Parse call.
//
// Parser options apply only to this call: they are reset to their defaults
// before opts are applied, so an option passed for one document does not carry
// over to the next parse on the same ParsedJson.
//
// On error the returned ParsedJson is non-nil (it is the reuse argument when one
// was supplied) and holds no tape, so a failed parse never exposes the previous
// document.
func Parse(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error) {
	pj := reuse
	if pj == nil {
		pj = newParsedJson()
	}
	// Reset options to their defaults before applying opts. Without this they
	// accumulate across reuse of the same ParsedJson — including reuse via the
	// pool, where the next caller would inherit them.
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
	// Single CGo call: parse + extract tape pointers
	bigIntFlag := C.int(0)
	if pj.bigInt {
		bigIntFlag = 1
	}
	res := C.simdjson_parse_and_get_tape(pj.parser, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)), bigIntFlag)
	if res.result.ok == 0 {
		pj.hasTape = false
		pj.tape = Tape{}
		return pj, fmt.Errorf("%s", C.GoString(res.result.error_msg))
	}
	pj.tape = Tape{
		data:        copyUint64Slice(unsafe.Pointer(res.tape), int(res.tape_len)),
		strings:     copyByteSlice(unsafe.Pointer(res.sbuf), int(res.sbuf_len)),
		copyStrings: pj.copyStrings,
		useNumber:   pj.useNumber,
	}
	pj.hasTape = true
	return pj, nil
}

// Close frees the underlying C++ parser. The ParsedJson must not be used after Close.
func (pj *ParsedJson) Close() {
	if pj.parser != nil {
		C.simdjson_parser_free(pj.parser)
		pj.parser = nil
		pj.hasTape = false
	}
}

// Reset clears the parsed tape and restores the default parser options,
// allowing the ParsedJson to be reused without inheriting prior state.
func (pj *ParsedJson) Reset() {
	pj.hasTape = false
	pj.tape = Tape{}
	pj.copyStrings = true
	pj.useNumber = false
	pj.bigInt = false
}

// Clone returns a deep copy of the ParsedJson tape data.
// The clone has no C++ parser — it can only be used for reading, not re-parsing.
// If dst is non-nil, it is reused. The returned pointer is never nil: cloning a
// ParsedJson that holds no tape yields an empty but usable clone.
func (pj *ParsedJson) Clone(dst *ParsedJson) *ParsedJson {
	if dst == nil {
		dst = &ParsedJson{}
	}
	dst.copyStrings = pj.copyStrings
	dst.useNumber = pj.useNumber
	dst.bigInt = pj.bigInt
	if !pj.hasTape {
		// Clear any tape dst carried from a previous use, so reusing a clone
		// target cannot surface a stale document.
		dst.hasTape = false
		dst.tape = Tape{}
		return dst
	}
	dst.tape = *pj.tape.Clone()
	dst.hasTape = true
	return dst
}

// ForEach calls fn for each root element in the parsed document.
// For standard JSON this calls fn exactly once. Compatible with simdjson-go's
// ndjson ForEach pattern.
func (pj *ParsedJson) ForEach(fn func(i Iter) error) error {
	iter, err := pj.Iter()
	if err != nil {
		return err
	}
	return fn(iter)
}

// Type represents a JSON element type.
type Type int

const (
	TypeArray  Type = '['
	TypeObject Type = '{'
	TypeInt64  Type = 'l'
	TypeUint64 Type = 'u'
	TypeDouble Type = 'd'
	TypeString Type = '"'
	TypeBool   Type = 't'
	TypeNull   Type = 'n'
	TypeBigInt Type = 'Z'
)

// String returns the type name.
func (t Type) String() string {
	switch t {
	case TypeArray:
		return "array"
	case TypeObject:
		return "object"
	case TypeInt64:
		return "int64"
	case TypeUint64:
		return "uint64"
	case TypeDouble:
		return "double"
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	case TypeNull:
		return "null"
	case TypeBigInt:
		return "bigint"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

// RootType returns the type of the root JSON element.
func (pj *ParsedJson) RootType() Type {
	iter, err := pj.Iter()
	if err != nil {
		return Type(-1)
	}
	return iter.Type()
}

// FindString finds a string value by key in the root object.
// Convenience method — equivalent to pj.Iter() → Object() → FindKey() → String().
func (pj *ParsedJson) FindString(key string) (string, error) {
	iter, err := pj.Iter()
	if err != nil {
		return "", err
	}
	obj, err := iter.Object(nil)
	if err != nil {
		return "", fmt.Errorf("root is not an object")
	}
	elem := obj.FindKey(key, nil)
	if elem == nil {
		return "", fmt.Errorf("key %q not found", key)
	}
	return elem.Iter.String()
}

// RootString returns the root element as a string.
func (pj *ParsedJson) RootString() (string, error) {
	iter, err := pj.Iter()
	if err != nil {
		return "", err
	}
	return iter.String()
}

// RootInt64 returns the root element as an int64.
func (pj *ParsedJson) RootInt64() (int64, error) {
	iter, err := pj.Iter()
	if err != nil {
		return 0, err
	}
	return iter.Int()
}

// RootUint64 returns the root element as a uint64.
func (pj *ParsedJson) RootUint64() (uint64, error) {
	iter, err := pj.Iter()
	if err != nil {
		return 0, err
	}
	return iter.Uint()
}

// RootDouble returns the root element as a float64.
func (pj *ParsedJson) RootDouble() (float64, error) {
	iter, err := pj.Iter()
	if err != nil {
		return 0, err
	}
	return iter.Float()
}

// RootBool returns the root element as a bool.
func (pj *ParsedJson) RootBool() (bool, error) {
	iter, err := pj.Iter()
	if err != nil {
		return false, err
	}
	return iter.Bool()
}

// RootCount returns the number of elements in a root array or keys in a root object.
func (pj *ParsedJson) RootCount() (int, error) {
	iter, err := pj.Iter()
	if err != nil {
		return 0, err
	}
	switch iter.Type() {
	case TypeObject:
		obj, err := iter.Object(nil)
		if err != nil {
			return 0, err
		}
		return obj.Count()
	case TypeArray:
		arr, err := iter.Array(nil)
		if err != nil {
			return 0, err
		}
		return arr.Count()
	default:
		return 0, fmt.Errorf("root is not an array or object")
	}
}

// SupportedCPU returns true if the CPU supports SIMD-accelerated JSON parsing.
// C++ simdjson always has a fallback implementation, so this always returns true.
// Use ActiveImplementation() to check which SIMD backend is in use.
func SupportedCPU() bool {
	return true
}

// ActiveImplementation returns the name of the active SIMD implementation
// detected at runtime (e.g. "haswell", "westmere", "arm64", "fallback").
func ActiveImplementation() string {
	return C.GoString(C.simdjson_active_implementation())
}

func copyUint64Slice(ptr unsafe.Pointer, length int) []uint64 {
	if ptr == nil || length == 0 {
		return nil
	}
	dst := make([]uint64, length)
	C.memcpy(unsafe.Pointer(&dst[0]), ptr, C.size_t(length*8))
	return dst
}

func copyByteSlice(ptr unsafe.Pointer, length int) []byte {
	if ptr == nil || length == 0 {
		return nil
	}
	dst := make([]byte, length)
	C.memcpy(unsafe.Pointer(&dst[0]), ptr, C.size_t(length))
	return dst
}
