// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

// Package simdjson provides Go bindings for the simdjson C++ library via CGo.
// It supports x86_64 (AVX2/SSE4.2) and ARM64 (NEON) with automatic runtime detection.
package simdjson

// #cgo CXXFLAGS: -std=c++17 -O2 -DNDEBUG
// #cgo LDFLAGS: -lstdc++ -lm
// #include "bridge.h"
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// ParsedJson holds a parsed JSON document. Safe to reuse via sync.Pool.
type ParsedJson struct {
	parser      C.simdjson_parser
	copyStrings bool
}

// ParserOption configures parsing behavior.
type ParserOption func(*ParsedJson)

// WithCopyStrings controls whether string values are copied from C memory.
// When true (default), strings are copied and safe to retain indefinitely.
// When false, strings point into parser-owned memory and are only valid
// until the next Parse call or Close — same semantics as simdjson-go.
func WithCopyStrings(copy bool) ParserOption {
	return func(pj *ParsedJson) {
		pj.copyStrings = copy
	}
}

// parserPool pools C++ parser instances to reduce allocations.
var parserPool = sync.Pool{
	New: func() interface{} {
		return &ParsedJson{parser: C.simdjson_parser_new(), copyStrings: true}
	},
}

// GetParser returns a ParsedJson from the pool. Call PutParser when done.
func GetParser() *ParsedJson {
	return parserPool.Get().(*ParsedJson)
}

// PutParser returns a ParsedJson to the pool for reuse.
func PutParser(pj *ParsedJson) {
	if pj != nil && pj.parser != nil {
		parserPool.Put(pj)
	}
}

// Parse parses JSON bytes using the provided ParsedJson (or a new one if nil).
// The returned ParsedJson owns the parsed data until the next Parse call.
func Parse(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error) {
	pj := reuse
	if pj == nil {
		pj = &ParsedJson{parser: C.simdjson_parser_new(), copyStrings: true}
	}
	for _, opt := range opts {
		opt(pj)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	res := C.simdjson_parse(pj.parser, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
	if res.ok == 0 {
		return nil, fmt.Errorf("%s", C.GoString(res.error_msg))
	}
	return pj, nil
}

// Close frees the underlying C++ parser. The ParsedJson must not be used after Close.
func (pj *ParsedJson) Close() {
	if pj.parser != nil {
		C.simdjson_parser_free(pj.parser)
		pj.parser = nil
	}
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
