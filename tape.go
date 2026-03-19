// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"unsafe"
)

// Tag is the raw tape tag byte. Compatible with simdjson-go's Tag type.
type Tag uint8

const (
	TagString      = Tag('"')
	TagInteger     = Tag('l')
	TagUint        = Tag('u')
	TagFloat       = Tag('d')
	TagNull        = Tag('n')
	TagBoolTrue    = Tag('t')
	TagBoolFalse   = Tag('f')
	TagObjectStart = Tag('{')
	TagObjectEnd   = Tag('}')
	TagArrayStart  = Tag('[')
	TagArrayEnd    = Tag(']')
	TagRoot        = Tag('r')
	TagEnd         = Tag(0)

	payloadMask = 0x00ffffffffffffff

	// Internal byte aliases for tape walking.
	tagRoot   = byte(TagRoot)
	tagObject = byte(TagObjectStart)
	tagObjEnd = byte(TagObjectEnd)
	tagArray  = byte(TagArrayStart)
	tagArrEnd = byte(TagArrayEnd)
	tagString = byte(TagString)
	tagInt64  = byte(TagInteger)
	tagUint64 = byte(TagUint)
	tagDouble = byte(TagFloat)
	tagTrue   = byte(TagBoolTrue)
	tagFalse  = byte(TagBoolFalse)
	tagNull   = byte(TagNull)
)

// Type converts a Tag to its corresponding Type.
func (tag Tag) Type() Type {
	switch tag {
	case TagBoolFalse:
		return TypeBool
	case TagEnd:
		return Type(-1)
	default:
		return Type(tag)
	}
}

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
	data        []uint64
	strings     []byte
	copyStrings bool
}

// GetTape returns the tape extracted during Parse. Zero-cost after parse.
func (pj *ParsedJson) GetTape() (*Tape, error) {
	if !pj.hasTape {
		return nil, fmt.Errorf("no parsed document")
	}
	return &pj.tape, nil
}

// Clone returns a deep copy of the tape and string buffer.
func (t *Tape) Clone() *Tape {
	d := make([]uint64, len(t.data))
	copy(d, t.data)
	s := make([]byte, len(t.strings))
	copy(s, t.strings)
	return &Tape{data: d, strings: s, copyStrings: t.copyStrings}
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

// Iter returns a TapeIter positioned at the root element.
func (t *Tape) Iter() TapeIter {
	return TapeIter{tape: t, idx: 1}
}

// TapeIter navigates the tape. Pure Go, zero CGo calls.
type TapeIter struct {
	tape *Tape
	idx  int
}

// Type returns the JSON type at the current position.
func (ti *TapeIter) Type() Type {
	if ti.idx >= len(ti.tape.data) {
		return Type(-1)
	}
	tag := byte(ti.tape.data[ti.idx] >> 56)
	// Normalize: 'f' (false) → 't' (bool)
	if tag == tagFalse {
		return TypeBool
	}
	return Type(tag)
}

// String returns the string value at the current position.
func (ti *TapeIter) String() (string, error) {
	if ti.tag() != tagString {
		return "", fmt.Errorf("element is not a string")
	}
	return ti.tape.readString(ti.payload())
}

// Int returns the int64 value at the current position.
func (ti *TapeIter) Int() (int64, error) {
	if ti.tag() != tagInt64 {
		return 0, fmt.Errorf("element is not an int64")
	}
	return int64(ti.tape.data[ti.idx+1]), nil
}

// Uint returns the uint64 value at the current position.
func (ti *TapeIter) Uint() (uint64, error) {
	tag := ti.tag()
	if tag == tagUint64 {
		return ti.tape.data[ti.idx+1], nil
	}
	if tag == tagInt64 {
		v := int64(ti.tape.data[ti.idx+1])
		if v >= 0 {
			return uint64(v), nil
		}
	}
	return 0, fmt.Errorf("element is not a uint64")
}

// Float returns the float64 value at the current position.
func (ti *TapeIter) Float() (float64, error) {
	tag := ti.tag()
	if tag == tagDouble {
		return math.Float64frombits(ti.tape.data[ti.idx+1]), nil
	}
	if tag == tagInt64 {
		return float64(int64(ti.tape.data[ti.idx+1])), nil
	}
	if tag == tagUint64 {
		return float64(ti.tape.data[ti.idx+1]), nil
	}
	return 0, fmt.Errorf("element is not a double")
}

// Bool returns the bool value at the current position.
func (ti *TapeIter) Bool() (bool, error) {
	tag := ti.tag()
	if tag == tagTrue {
		return true, nil
	}
	if tag == tagFalse {
		return false, nil
	}
	return false, fmt.Errorf("element is not a bool")
}

// Object returns a TapeObject for key-value access.
func (ti *TapeIter) Object() (*TapeObject, error) {
	if ti.tag() != tagObject {
		return nil, fmt.Errorf("element is not an object")
	}
	entry := ti.tape.data[ti.idx]
	endIdx := int(entry & 0xffffffff)
	return &TapeObject{tape: ti.tape, startIdx: ti.idx + 1, endIdx: endIdx - 1}, nil
}

// Array returns a TapeArray for element access.
func (ti *TapeIter) Array() (*TapeArray, error) {
	if ti.tag() != tagArray {
		return nil, fmt.Errorf("element is not an array")
	}
	entry := ti.tape.data[ti.idx]
	endIdx := int(entry & 0xffffffff)
	return &TapeArray{tape: ti.tape, startIdx: ti.idx + 1, endIdx: endIdx - 1}, nil
}

// Interface converts the element to its Go native equivalent.
func (ti *TapeIter) Interface() (interface{}, error) {
	val, _, err := ti.tape.readValue(ti.idx)
	return val, err
}

func (ti *TapeIter) tag() byte   { return byte(ti.tape.data[ti.idx] >> 56) }
func (ti *TapeIter) payload() uint64 { return ti.tape.data[ti.idx] & payloadMask }

// skipValue returns the tape index after the value at idx.
func (t *Tape) skipValue(idx int) int {
	tag := byte(t.data[idx] >> 56)
	switch tag {
	case tagObject, tagArray:
		return int(t.data[idx] & 0xffffffff) // end index (past closing tag)
	case tagInt64, tagUint64, tagDouble:
		return idx + 2
	default:
		return idx + 1
	}
}

// TapeObject provides key-value access over the tape.
type TapeObject struct {
	tape     *Tape
	startIdx int // first key index
	endIdx   int // index of closing '}'
}

// FindKey finds a key in the object. Returns nil if not found.
func (o *TapeObject) FindKey(key string) *TapeIter {
	pos := o.startIdx
	for pos < o.endIdx {
		keyEntry := o.tape.data[pos]
		if byte(keyEntry>>56) != tagString {
			break
		}
		k, _ := o.tape.readString(keyEntry & payloadMask)
		valIdx := pos + 1
		if k == key {
			return &TapeIter{tape: o.tape, idx: valIdx}
		}
		pos = o.tape.skipValue(valIdx)
	}
	return nil
}

// ForEach iterates over all key-value pairs.
func (o *TapeObject) ForEach(fn func(key string, val TapeIter) error) error {
	pos := o.startIdx
	for pos < o.endIdx {
		keyEntry := o.tape.data[pos]
		if byte(keyEntry>>56) != tagString {
			break
		}
		key, _ := o.tape.readString(keyEntry & payloadMask)
		valIdx := pos + 1
		if err := fn(key, TapeIter{tape: o.tape, idx: valIdx}); err != nil {
			return err
		}
		pos = o.tape.skipValue(valIdx)
	}
	return nil
}

// Map converts the object to map[string]interface{}.
func (o *TapeObject) Map(dst map[string]interface{}) (map[string]interface{}, error) {
	if dst == nil {
		dst = make(map[string]interface{}, 8)
	}
	pos := o.startIdx
	for pos < o.endIdx {
		keyEntry := o.tape.data[pos]
		if byte(keyEntry>>56) != tagString {
			break
		}
		key, _ := o.tape.readString(keyEntry & payloadMask)
		val, nextPos, err := o.tape.readValue(pos + 1)
		if err != nil {
			return nil, err
		}
		dst[key] = val
		pos = nextPos
	}
	return dst, nil
}

// Count returns the number of key-value pairs.
func (o *TapeObject) Count() int {
	return int((o.tape.data[o.startIdx-1] >> 32) & 0xffffff)
}

// FindPath navigates a path of nested keys.
func (o *TapeObject) FindPath(path ...string) *TapeIter {
	if len(path) == 0 {
		return nil
	}
	iter := o.FindKey(path[0])
	if iter == nil {
		return nil
	}
	for _, key := range path[1:] {
		obj, err := iter.Object()
		if err != nil {
			return nil
		}
		iter = obj.FindKey(key)
		if iter == nil {
			return nil
		}
	}
	return iter
}

// TapeArray provides element access over the tape.
type TapeArray struct {
	tape     *Tape
	startIdx int // first element index
	endIdx   int // index of closing ']'
}

// ForEach iterates over all elements.
func (a *TapeArray) ForEach(fn func(val TapeIter) error) error {
	pos := a.startIdx
	for pos < a.endIdx {
		if err := fn(TapeIter{tape: a.tape, idx: pos}); err != nil {
			return err
		}
		pos = a.tape.skipValue(pos)
	}
	return nil
}

// Count returns the number of elements.
func (a *TapeArray) Count() int {
	return int((a.tape.data[a.startIdx-1] >> 32) & 0xffffff)
}

// AsInteger returns all elements as []int64.
func (a *TapeArray) AsInteger() ([]int64, error) {
	result := make([]int64, 0, a.Count())
	err := a.ForEach(func(val TapeIter) error {
		v, err := val.Int()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsFloat returns all elements as []float64.
func (a *TapeArray) AsFloat() ([]float64, error) {
	result := make([]float64, 0, a.Count())
	err := a.ForEach(func(val TapeIter) error {
		v, err := val.Float()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsString returns all elements as []string.
func (a *TapeArray) AsString() ([]string, error) {
	result := make([]string, 0, a.Count())
	err := a.ForEach(func(val TapeIter) error {
		v, err := val.String()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
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
	b, err := t.readStringBytes(offset)
	if err != nil {
		return "", err
	}
	if t.copyStrings {
		return string(b), nil
	}
	return unsafe.String(&b[0], len(b)), nil
}

func (t *Tape) readStringBytes(offset uint64) ([]byte, error) {
	off := int(offset)
	if off+4 > len(t.strings) {
		return nil, fmt.Errorf("string offset %d out of bounds", off)
	}
	slen := int(binary.LittleEndian.Uint32(t.strings[off : off+4]))
	start := off + 4
	if start+slen > len(t.strings) {
		return nil, fmt.Errorf("string length %d at offset %d out of bounds", slen, off)
	}
	return t.strings[start : start+slen], nil
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
		return json.Number(strconv.FormatInt(int64(t.data[idx+1]), 10)), idx + 2, nil
	case tagUint64:
		return json.Number(strconv.FormatUint(t.data[idx+1], 10)), idx + 2, nil
	case tagDouble:
		return json.Number(strconv.FormatFloat(math.Float64frombits(t.data[idx+1]), 'g', -1, 64)), idx + 2, nil
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

// Advance moves the iterator to the next sibling element.
func (ti *TapeIter) Advance() Type {
	ti.idx = ti.tape.skipValue(ti.idx)
	if ti.idx >= len(ti.tape.data) {
		return Type(-1)
	}
	return ti.Type()
}

// PeekNext returns the type of the next sibling without advancing.
func (ti *TapeIter) PeekNext() Type {
	next := ti.tape.skipValue(ti.idx)
	if next >= len(ti.tape.data) {
		return Type(-1)
	}
	tag := byte(ti.tape.data[next] >> 56)
	if tag == tagFalse {
		return TypeBool
	}
	return Type(tag)
}

// AdvanceInto steps into a container (object/array), positioning at the first child.
func (ti *TapeIter) AdvanceInto() Type {
	tag := ti.tag()
	if tag != tagObject && tag != tagArray {
		return Type(-1)
	}
	ti.idx++
	if ti.idx >= len(ti.tape.data) {
		return Type(-1)
	}
	return ti.Type()
}

// FindElement navigates a path of object keys from the current element.
func (ti *TapeIter) FindElement(path ...string) *TapeIter {
	if len(path) == 0 {
		return nil
	}
	obj, err := ti.Object()
	if err != nil {
		return nil
	}
	return obj.FindPath(path...)
}

// StringCvt converts any scalar to its string representation.
func (ti *TapeIter) StringCvt() (string, error) {
	switch ti.Type() {
	case TypeString:
		return ti.String()
	case TypeInt64:
		v, _ := ti.Int()
		return strconv.FormatInt(v, 10), nil
	case TypeUint64:
		v, _ := ti.Uint()
		return strconv.FormatUint(v, 10), nil
	case TypeDouble:
		v, _ := ti.Float()
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case TypeBool:
		v, _ := ti.Bool()
		return strconv.FormatBool(v), nil
	case TypeNull:
		return "null", nil
	default:
		return "", fmt.Errorf("cannot convert %v to string", ti.Type())
	}
}

// Iter returns a TapeIter at the first element of the array.
func (a *TapeArray) Iter() TapeIter {
	return TapeIter{tape: a.tape, idx: a.startIdx}
}

// FirstType returns the type of the first element, or Type(-1) if empty.
func (a *TapeArray) FirstType() Type {
	if a.startIdx >= a.endIdx {
		return Type(-1)
	}
	tag := byte(a.tape.data[a.startIdx] >> 56)
	if tag == tagFalse {
		return TypeBool
	}
	return Type(tag)
}

// Interface returns the array as []interface{}.
func (a *TapeArray) Interface() ([]interface{}, error) {
	result := make([]interface{}, 0, a.Count())
	pos := a.startIdx
	for pos < a.endIdx {
		val, nextPos, err := a.tape.readValue(pos)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
		pos = nextPos
	}
	return result, nil
}

// Iter returns a TapeIter at the first key of the object.
func (o *TapeObject) Iter() TapeIter {
	return TapeIter{tape: o.tape, idx: o.startIdx}
}
