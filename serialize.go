// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/binary"
	"fmt"
)

// Serializer allows serializing a parsed JSON document to a compact binary
// format and reading it back. A Serializer can be reused but not used
// concurrently.
//
// Binary format:
//   - Version byte (1)
//   - Tape length (uint64 LE)
//   - Tape data ([]uint64 as LE bytes)
//   - Strings length (uint64 LE)
//   - Strings data (raw bytes)
type Serializer struct{}

const serializerVersion = 1

// NewSerializer creates a new Serializer.
func NewSerializer() *Serializer {
	return &Serializer{}
}

// Serialize writes the ParsedJson tape and string buffer to dst.
// Returns the extended dst slice.
func (s *Serializer) Serialize(dst []byte, pj ParsedJson) []byte {
	if !pj.hasTape {
		return dst
	}
	tapeLen := len(pj.tape.data)
	strLen := len(pj.tape.strings)

	// Version
	dst = append(dst, serializerVersion)

	// Tape
	var tmp [8]byte
	binary.NativeEndian.PutUint64(tmp[:], uint64(tapeLen))
	dst = append(dst, tmp[:]...)
	for _, v := range pj.tape.data {
		binary.NativeEndian.PutUint64(tmp[:], v)
		dst = append(dst, tmp[:]...)
	}

	// Strings
	binary.NativeEndian.PutUint64(tmp[:], uint64(strLen))
	dst = append(dst, tmp[:]...)
	dst = append(dst, pj.tape.strings...)

	return dst
}

// Deserialize reads a previously serialized ParsedJson from src.
// An optional dst can be provided for reuse.
func (s *Serializer) Deserialize(src []byte, dst *ParsedJson) (*ParsedJson, error) {
	if len(src) < 1 {
		return nil, fmt.Errorf("empty input")
	}
	if src[0] != serializerVersion {
		return nil, fmt.Errorf("unsupported version %d", src[0])
	}
	off := 1

	// Tape
	if off+8 > len(src) {
		return nil, fmt.Errorf("truncated tape length")
	}
	tapeLen := int(binary.NativeEndian.Uint64(src[off:]))
	off += 8

	tapeBytes := tapeLen * 8
	if off+tapeBytes > len(src) {
		return nil, fmt.Errorf("truncated tape data")
	}
	tapeData := make([]uint64, tapeLen)
	for i := range tapeData {
		tapeData[i] = binary.NativeEndian.Uint64(src[off:])
		off += 8
	}

	// Strings
	if off+8 > len(src) {
		return nil, fmt.Errorf("truncated strings length")
	}
	strLen := int(binary.NativeEndian.Uint64(src[off:]))
	off += 8

	if off+strLen > len(src) {
		return nil, fmt.Errorf("truncated strings data")
	}
	strings := make([]byte, strLen)
	copy(strings, src[off:off+strLen])

	if dst == nil {
		dst = newParsedJson()
	}
	dst.tape = Tape{
		data:        tapeData,
		strings:     strings,
		copyStrings: true,
	}
	dst.hasTape = true
	return dst, nil
}
