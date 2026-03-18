// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

const (
	tagRoot   = 'r'
	tagObject = '{'
	tagObjEnd = '}'
	tagArray  = '['
	tagArrEnd = ']'
	tagString = '"'
	tagInt64  = 'l'
	tagUint64 = 'u'
	tagDouble = 'd'
	tagTrue   = 't'
	tagFalse  = 'f'
	tagNull   = 'n'

	payloadMask = 0x00ffffffffffffff
)

// Tape holds the raw tape and string buffer from a parsed JSON document.
//
// The C++ simdjson parser produces two internal buffers during parsing:
//
//   - Tape ([]uint64): a flat array where each JSON element is 1-2 entries.
//     The upper 8 bits of each entry encode the type tag ('{', '[', '"', 'l',
//     'u', 'd', 't', 'f', 'n', 'r'), and the lower 56 bits encode the payload
//     (string buffer offset, container end index, etc.). Numbers (int64, uint64,
//     double) use two entries: the tag entry followed by the raw 64-bit value.
//
//   - String buffer ([]byte): all parsed string values, copied from the input
//     with JSON escape sequences resolved (e.g. \" → ", \n → newline). Each
//     string is stored as a 4-byte little-endian length prefix followed by the
//     UTF-8 bytes and a null terminator. This is NOT the original input buffer.
//
// Both buffers are owned by the C++ parser and are overwritten on the next
// Parse() call or freed on Close(). The Tape struct points directly into
// this C memory — no copies are made.
type Tape struct {
	data    []uint64
	strings []byte
}

// GetTape extracts the raw tape from a ParsedJson via a single CGo call.
// Valid until the next Parse or Close on pj.
func (pj *ParsedJson) GetTape() (*Tape, error) {
	tape, tapeLen, sbuf, sbufLen, err := getTapeRaw(pj)
	if err != nil {
		return nil, err
	}
	return &Tape{
		data:    ptrToUint64Slice(tape, tapeLen),
		strings: ptrToByteSlice(sbuf, sbufLen),
	}, nil
}

// TapeInterface converts the entire document to Go native types via pure Go
// tape walking. Zero CGo calls — significantly faster than DOM-based Interface().
func (pj *ParsedJson) TapeInterface() (interface{}, error) {
	t, err := pj.GetTape()
	if err != nil {
		return nil, err
	}
	return t.Interface()
}

// TapeInterfaceUseNumber is like TapeInterface but returns json.Number for numerics.
func (pj *ParsedJson) TapeInterfaceUseNumber() (interface{}, error) {
	t, err := pj.GetTape()
	if err != nil {
		return nil, err
	}
	return t.InterfaceUseNumber()
}

// RootType returns the type of the root element.
func (t *Tape) RootType() Type {
	if len(t.data) < 2 {
		return Type(-1)
	}
	return Type(t.data[1] >> 56)
}

// Interface converts the entire document to Go native types.
func (t *Tape) Interface() (interface{}, error) {
	if len(t.data) < 2 {
		return nil, fmt.Errorf("empty tape")
	}
	val, _, err := t.readValue(1)
	return val, err
}

// InterfaceUseNumber is like Interface but returns json.Number for numeric values.
func (t *Tape) InterfaceUseNumber() (interface{}, error) {
	if len(t.data) < 2 {
		return nil, fmt.Errorf("empty tape")
	}
	val, _, err := t.readValueNum(1)
	return val, err
}

func (t *Tape) readValue(idx int) (interface{}, int, error) {
	if idx >= len(t.data) {
		return nil, idx, fmt.Errorf("tape index %d out of bounds", idx)
	}
	entry := t.data[idx]
	tag := byte(entry >> 56)
	payload := entry & payloadMask

	switch tag {
	case tagString:
		s, err := t.readString(payload)
		return s, idx + 1, err
	case tagInt64:
		return int64(t.data[idx+1]), idx + 2, nil
	case tagUint64:
		return t.data[idx+1], idx + 2, nil
	case tagDouble:
		return math.Float64frombits(t.data[idx+1]), idx + 2, nil
	case tagTrue:
		return true, idx + 1, nil
	case tagFalse:
		return false, idx + 1, nil
	case tagNull:
		return nil, idx + 1, nil
	case tagObject:
		return t.readObject(idx)
	case tagArray:
		return t.readArray(idx)
	default:
		return nil, idx + 1, fmt.Errorf("unknown tag '%c' at index %d", tag, idx)
	}
}

func (t *Tape) readObject(idx int) (map[string]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & 0xffffffff)
	count := int((entry >> 32) & 0xffffff)
	result := make(map[string]interface{}, count)
	pos := idx + 1
	for pos < endIdx-1 {
		keyEntry := t.data[pos]
		if byte(keyEntry>>56) != tagString {
			return nil, pos, fmt.Errorf("expected string key at %d", pos)
		}
		key, err := t.readString(keyEntry & payloadMask)
		if err != nil {
			return nil, pos, err
		}
		pos++
		val, nextPos, err := t.readValue(pos)
		if err != nil {
			return nil, pos, err
		}
		result[key] = val
		pos = nextPos
	}
	return result, endIdx, nil
}

func (t *Tape) readArray(idx int) ([]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & 0xffffffff)
	count := int((entry >> 32) & 0xffffff)
	result := make([]interface{}, 0, count)
	pos := idx + 1
	for pos < endIdx-1 {
		val, nextPos, err := t.readValue(pos)
		if err != nil {
			return nil, pos, err
		}
		result = append(result, val)
		pos = nextPos
	}
	return result, endIdx, nil
}

func (t *Tape) readString(offset uint64) (string, error) {
	off := int(offset)
	if off+4 > len(t.strings) {
		return "", fmt.Errorf("string offset %d out of bounds", off)
	}
	slen := int(binary.LittleEndian.Uint32(t.strings[off : off+4]))
	start := off + 4
	if start+slen > len(t.strings) {
		return "", fmt.Errorf("string length %d at offset %d out of bounds", slen, off)
	}
	return string(t.strings[start : start+slen]), nil
}

func (t *Tape) readValueNum(idx int) (interface{}, int, error) {
	if idx >= len(t.data) {
		return nil, idx, fmt.Errorf("tape index %d out of bounds", idx)
	}
	entry := t.data[idx]
	tag := byte(entry >> 56)
	payload := entry & payloadMask

	switch tag {
	case tagString:
		s, err := t.readString(payload)
		return s, idx + 1, err
	case tagInt64:
		return json.Number(fmt.Sprintf("%d", int64(t.data[idx+1]))), idx + 2, nil
	case tagUint64:
		return json.Number(fmt.Sprintf("%d", t.data[idx+1])), idx + 2, nil
	case tagDouble:
		return json.Number(fmt.Sprintf("%v", math.Float64frombits(t.data[idx+1]))), idx + 2, nil
	case tagTrue:
		return true, idx + 1, nil
	case tagFalse:
		return false, idx + 1, nil
	case tagNull:
		return nil, idx + 1, nil
	case tagObject:
		return t.readObjectNum(idx)
	case tagArray:
		return t.readArrayNum(idx)
	default:
		return nil, idx + 1, fmt.Errorf("unknown tag '%c' at index %d", tag, idx)
	}
}

func (t *Tape) readObjectNum(idx int) (map[string]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & 0xffffffff)
	count := int((entry >> 32) & 0xffffff)
	result := make(map[string]interface{}, count)
	pos := idx + 1
	for pos < endIdx-1 {
		keyEntry := t.data[pos]
		if byte(keyEntry>>56) != tagString {
			return nil, pos, fmt.Errorf("expected string key at %d", pos)
		}
		key, err := t.readString(keyEntry & payloadMask)
		if err != nil {
			return nil, pos, err
		}
		pos++
		val, nextPos, err := t.readValueNum(pos)
		if err != nil {
			return nil, pos, err
		}
		result[key] = val
		pos = nextPos
	}
	return result, endIdx, nil
}

func (t *Tape) readArrayNum(idx int) ([]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & 0xffffffff)
	count := int((entry >> 32) & 0xffffff)
	result := make([]interface{}, 0, count)
	pos := idx + 1
	for pos < endIdx-1 {
		val, nextPos, err := t.readValueNum(pos)
		if err != nil {
			return nil, pos, err
		}
		result = append(result, val)
		pos = nextPos
	}
	return result, endIdx, nil
}
