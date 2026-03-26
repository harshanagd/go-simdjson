// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// FloatFlags records metadata about parsed floats.
type FloatFlags uint64

// FloatFlag is a single flag recorded when parsing floats.
type FloatFlag uint64

const (
	// FloatOverflowedInteger is set when a JSON integer overflowed int64/uint64
	// and was parsed as float instead.
	FloatOverflowedInteger FloatFlag = 1 << iota
)

// Contains returns whether f contains the specified flag.
func (f FloatFlags) Contains(flag FloatFlag) bool {
	return FloatFlag(f)&flag == flag
}

// Iter represents a position in the parsed JSON document.
type Iter struct {
	tape        *Tape
	tapeIdx     int
	copyStrings bool
	useNumber   bool
}

// Object represents a JSON object for key-value access.
type Object struct {
	tobj        *TapeObject
	iterPos     int // current position for NextElement
	copyStrings bool
	useNumber   bool
}

// Element is a key-value pair result from object lookup.
type Element struct {
	Name string
	Type Type
	Iter Iter
}

// Iter returns an Iter positioned at the root of the parsed document.
func (pj *ParsedJson) Iter() (Iter, error) {
	if !pj.hasTape {
		return Iter{}, fmt.Errorf("no parsed document")
	}
	return Iter{tape: &pj.tape, tapeIdx: 1, copyStrings: pj.copyStrings, useNumber: pj.useNumber}, nil
}

// Type returns the JSON type of the current element.
func (i *Iter) Type() Type {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.Type()
}

// String extracts a string value from the current element.
// If WithCopyStrings(false) was set, the string points into parser-owned
// memory and is only valid until the next Parse call or Close.
func (i *Iter) String() (string, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.String()
}

// StringRef extracts a string value without copying.
// The returned string points into parser-owned memory and is only valid
// until the next Parse call or Close.
func (i *Iter) StringRef() (string, error) {
	return i.String() // tape strings already point into C memory
}

// StringCvt converts any scalar value to its JSON string representation
// using simdjson's native serialization — no float precision loss.
func (i *Iter) StringCvt() (string, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	switch ti.Type() {
	case TypeObject, TypeArray:
		return "", fmt.Errorf("cannot convert %v to string", ti.Type())
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
		if v {
			return "true", nil
		}
		return "false", nil
	case TypeNull:
		return "null", nil
	default:
		return "", fmt.Errorf("unknown type %v", ti.Type())
	}
}

// Int returns the element value as int64.
func (i *Iter) Int() (int64, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.Int()
}

// Uint returns the element value as uint64.
func (i *Iter) Uint() (uint64, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.Uint()
}

// Float returns the element value as float64.
func (i *Iter) Float() (float64, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.Float()
}

// Bool returns the element value as bool.
func (i *Iter) Bool() (bool, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.Bool()
}

// Object returns the element as an Object for key-value access.
func (i *Iter) Object(reuse *Object) (*Object, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tobj, err := ti.Object()
	if err != nil {
		return nil, err
	}
	if reuse != nil {
		reuse.tobj = tobj
		reuse.iterPos = tobj.startIdx
		reuse.copyStrings = i.copyStrings
		reuse.useNumber = i.useNumber
		return reuse, nil
	}
	return &Object{tobj: tobj, iterPos: tobj.startIdx, copyStrings: i.copyStrings, useNumber: i.useNumber}, nil
}

// FindKey finds a key in the object and returns an Element.
// Returns nil if the key is not found.
func (o *Object) FindKey(key string, reuse *Element) *Element {
	ti := o.tobj.FindKey(key)
	if ti == nil {
		return nil
	}
	iter := Iter{tape: ti.tape, tapeIdx: ti.idx, copyStrings: o.copyStrings, useNumber: o.useNumber}
	if reuse != nil {
		reuse.Iter = iter
		return reuse
	}
	return &Element{Iter: iter}
}

// ForEach iterates over all key-value pairs in O(n) time.
func (o *Object) ForEach(fn func(key string, i Iter) error) error {
	return o.tobj.ForEach(func(key string, val TapeIter) error {
		return fn(key, Iter{tape: val.tape, tapeIdx: val.idx, copyStrings: o.copyStrings, useNumber: o.useNumber})
	})
}

// Count returns the number of key-value pairs in the object.
func (o *Object) Count() (int, error) {
	return o.tobj.Count(), nil
}

// Array represents a JSON array for element access.
type Array struct {
	tarr        *TapeArray
	copyStrings bool
	useNumber   bool
}

// Array returns the element as an Array.
func (i *Iter) Array(reuse *Array) (*Array, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tarr, err := ti.Array()
	if err != nil {
		return nil, err
	}
	if reuse != nil {
		reuse.tarr = tarr
		reuse.copyStrings = i.copyStrings
		reuse.useNumber = i.useNumber
		return reuse, nil
	}
	return &Array{tarr: tarr, copyStrings: i.copyStrings, useNumber: i.useNumber}, nil
}

// ForEach iterates over all elements in O(n) time.
func (a *Array) ForEach(fn func(i Iter) error) error {
	return a.tarr.ForEach(func(val TapeIter) error {
		return fn(Iter{tape: val.tape, tapeIdx: val.idx, copyStrings: a.copyStrings, useNumber: a.useNumber})
	})
}

// Count returns the number of elements in the array.
func (a *Array) Count() (int, error) {
	return a.tarr.Count(), nil
}

// Interface converts the element to its Go native equivalent:
// object → map[string]interface{}, array → []interface{},
// string → string, int64/uint64 → int64/uint64, double → float64,
// bool → bool, null → nil.
// Interface converts the element to its Go native equivalent.
// Uses the tape walker for performance (pure Go, zero CGo per element).
// Interface converts the element to its Go native equivalent.
// Uses the tape walker (pure Go, zero CGo per element).
func (i *Iter) Interface() (interface{}, error) {
	if i.useNumber {
		val, _, err := i.tape.readValueNum(i.tapeIdx)
		return val, err
	}
	val, _, err := i.tape.readValue(i.tapeIdx)
	return val, err
}

// Map converts the object to a map[string]interface{}.
func (o *Object) Map(dst map[string]interface{}) (map[string]interface{}, error) {
	return o.tobj.Map(dst)
}

// StringBytes extracts a string value as []byte.
// Respects WithCopyStrings setting.
func (i *Iter) StringBytes() ([]byte, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.tape.readStringBytes(ti.payload())
}

// FindPath navigates a dot-separated path of object keys from the current element.
func (o *Object) FindPath(reuse *Element, path ...string) (*Element, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	elem := o.FindKey(path[0], reuse)
	if elem == nil {
		return nil, fmt.Errorf("key %q not found", path[0])
	}
	for _, key := range path[1:] {
		obj, err := elem.Iter.Object(nil)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		elem = obj.FindKey(key, reuse)
		if elem == nil {
			return nil, fmt.Errorf("key %q not found", key)
		}
	}
	return elem, nil
}

// FindElement navigates a path of object keys from the root.
func (i *Iter) FindElement(reuse *Element, path ...string) (*Element, error) {
	obj, err := i.Object(nil)
	if err != nil {
		return nil, err
	}
	return obj.FindPath(reuse, path...)
}

// Advance moves to the next sibling element and returns its type.
// Returns Type(-1) at end.
func (i *Iter) Advance() Type {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	t := ti.Advance()
	i.tapeIdx = ti.idx
	return t
}

// AdvanceIter advances and copies the current element into dst.
// Returns the type of the element and an error if past end.
func (i *Iter) AdvanceIter(dst *Iter) (Type, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	t := ti.Advance()
	if int(t) == -1 {
		return Type(-1), nil
	}
	i.tapeIdx = ti.idx
	if dst != i {
		*dst = *i
	}
	// Position dst at the element we just advanced to
	dst.tapeIdx = ti.idx
	return t, nil
}

// PeekNext returns the type of the next sibling without advancing.
func (i *Iter) PeekNext() Type {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.PeekNext()
}

// PeekNextTag returns the raw Tag of the next sibling without advancing.
// Returns TagEnd at end.
func (i *Iter) PeekNextTag() Tag {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	next := i.tape.skipValue(ti.idx)
	if next >= len(i.tape.data) {
		return TagEnd
	}
	return Tag(i.tape.data[next] >> 56)
}

// Root returns an Iter positioned at the root element's value.
func (i *Iter) Root(dst *Iter) (Type, *Iter, error) {
	if dst == nil {
		c := *i
		dst = &c
	} else {
		*dst = *i
	}
	// Position at the first element after root tag (index 1)
	dst.tapeIdx = 1
	return dst.Type(), dst, nil
}

// FloatFlags returns the float value and associated flags.
// Also accepts int64/uint64 values (returns them as float64 with no flags).
func (i *Iter) FloatFlags() (float64, FloatFlags, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagDouble:
		v, err := ti.Float()
		return v, 0, err
	case tagInt64:
		v, err := ti.Int()
		return float64(v), 0, err
	case tagUint64:
		v, err := ti.Uint()
		return float64(v), 0, err
	default:
		return 0, 0, fmt.Errorf("cannot convert %v to float", ti.Type())
	}
}

// NextElement returns the next key-value pair. Initialize the iterator by
// calling Object() first. Returns empty name when done.
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error) {
	n, t, err := o.NextElementBytes(dst)
	return string(n), t, err
}

// NextElementBytes is like NextElement but returns the key as []byte,
// avoiding a string allocation. Returns nil name when done.
func (o *Object) NextElementBytes(dst *Iter) (name []byte, t Type, err error) {
	if o.iterPos >= o.tobj.endIdx {
		return nil, TypeNull, nil
	}
	keyEntry := o.tobj.tape.data[o.iterPos]
	if byte(keyEntry>>56) != tagString {
		return nil, TypeNull, nil
	}
	s, _ := o.tobj.tape.readStringBytes(keyEntry & payloadMask)
	valIdx := o.iterPos + 1
	if dst != nil {
		dst.tape = o.tobj.tape
		dst.tapeIdx = valIdx
		dst.copyStrings = o.copyStrings
		dst.useNumber = o.useNumber
	}
	t = Type(o.tobj.tape.data[valIdx] >> 56)
	if byte(t) == tagFalse {
		t = TypeBool
	}
	o.iterPos = o.tobj.tape.skipValue(valIdx)
	return s, t, nil
}

// Elements contains all key-value pairs of an object, kept in original order.
type Elements struct {
	Elements []Element
	Index    map[string]int
}

// Lookup returns the element with the given key, or nil if not found.
func (e Elements) Lookup(key string) *Element {
	idx, ok := e.Index[key]
	if !ok {
		return nil
	}
	return &e.Elements[idx]
}

// Parse collects all key-value pairs into an Elements collection.
// If dst is non-nil it is reused.
func (o *Object) Parse(dst *Elements) (*Elements, error) {
	if dst == nil {
		dst = &Elements{
			Elements: make([]Element, 0, o.tobj.Count()),
			Index:    make(map[string]int, o.tobj.Count()),
		}
	} else {
		dst.Elements = dst.Elements[:0]
		for k := range dst.Index {
			delete(dst.Index, k)
		}
	}
	// Reset iteration position to start of object
	o.iterPos = o.tobj.startIdx
	var tmp Iter
	for {
		name, t, err := o.NextElement(&tmp)
		if err != nil {
			return dst, err
		}
		if name == "" && t == TypeNull {
			break
		}
		dst.Index[name] = len(dst.Elements)
		dst.Elements = append(dst.Elements, Element{
			Name: name,
			Type: t,
			Iter: tmp,
		})
	}
	return dst, nil
}

// Interface returns the array as []interface{}.
func (a *Array) Interface() ([]interface{}, error) {
	var result []interface{}
	err := a.ForEach(func(elem Iter) error {
		v, err := elem.Interface()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsFloat returns all elements as []float64.
func (a *Array) AsFloat() ([]float64, error) {
	n, _ := a.Count()
	result := make([]float64, 0, n)
	err := a.ForEach(func(elem Iter) error {
		v, err := elem.Float()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsInteger returns all elements as []int64.
func (a *Array) AsInteger() ([]int64, error) {
	n, _ := a.Count()
	result := make([]int64, 0, n)
	err := a.ForEach(func(elem Iter) error {
		v, err := elem.Int()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsUint64 returns all elements as []uint64.
func (a *Array) AsUint64() ([]uint64, error) {
	n, _ := a.Count()
	result := make([]uint64, 0, n)
	err := a.ForEach(func(elem Iter) error {
		v, err := elem.Uint()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsString returns all elements as []string.
func (a *Array) AsString() ([]string, error) {
	n, _ := a.Count()
	result := make([]string, 0, n)
	err := a.ForEach(func(elem Iter) error {
		v, err := elem.String()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// AsStringCvt returns all elements converted to strings via StringCvt.
func (a *Array) AsStringCvt() ([]string, error) {
	n, _ := a.Count()
	result := make([]string, 0, n)
	err := a.ForEach(func(elem Iter) error {
		v, err := elem.StringCvt()
		if err != nil {
			return err
		}
		result = append(result, v)
		return nil
	})
	return result, err
}

// TagNop is a no-operation tape entry used to fill gaps after mutations.
// The payload stores the skip distance for Advance().
const TagNop = Tag('N')

const tagNop = byte(TagNop)

// --- Tape write helpers ---
// These abstract the tape entry format: [tag:8 | payload:56].

// tapeEntry builds a tape entry from a tag byte and payload.
func tapeEntry(tag byte, payload uint64) uint64 {
	return (uint64(tag) << 56) | (payload & payloadMask)
}

// tapeSetTag writes a tag-only entry (no payload) at the given index.
func (t *Tape) tapeSetTag(idx int, tag byte) {
	t.data[idx] = tapeEntry(tag, 0)
}

// tapeSetTagPayload writes a tag + payload entry at the given index.
func (t *Tape) tapeSetTagPayload(idx int, tag byte, payload uint64) {
	t.data[idx] = tapeEntry(tag, payload)
}

// tapeSetNop writes a NOP entry at idx. Advance() will skip forward by `skip` entries.
func (t *Tape) tapeSetNop(idx int, skip uint64) {
	t.data[idx] = tapeEntry(tagNop, skip)
}

// tapeTagAt returns the tag byte at the given tape index.
func (t *Tape) tapeTagAt(idx int) byte {
	return byte(t.data[idx] >> 56)
}

// tapePayloadAt returns the 56-bit payload at the given tape index.
func (t *Tape) tapePayloadAt(idx int) uint64 {
	return t.data[idx] & payloadMask
}

// tapeSkipNop advances past a NOP entry, returning the next index.
func (t *Tape) tapeSkipNop(idx int) int {
	skip := int(t.tapePayloadAt(idx))
	if skip == 0 {
		skip = 1
	}
	return idx + skip
}

// tapeNopRange fills tape[start:end] with NOP entries, each pointing to end.
func (t *Tape) tapeNopRange(start, end int) {
	for j := start; j < end; j++ {
		t.tapeSetNop(j, uint64(end-j))
	}
}

// tapeAppendString appends a string to the string buffer and returns the offset.
// Format: [4-byte LE length][UTF-8 bytes][null terminator].
// The buffer is already well-sized from parse; append handles growth if needed.
func (t *Tape) tapeAppendString(v []byte) uint64 {
	off := len(t.strings)
	t.strings = append(t.strings, 0, 0, 0, 0)
	binary.NativeEndian.PutUint32(t.strings[off:], uint32(len(v)))
	t.strings = append(t.strings, v...)
	t.strings = append(t.strings, 0)
	return uint64(off)
}

// --- Mutation methods ---

// SetFloat changes the current value to a float64.
// Works on float, int, and uint elements (all use 2 tape entries).
func (i *Iter) SetFloat(v float64) error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagDouble, tagInt64, tagUint64:
		i.tape.tapeSetTag(i.tapeIdx, tagDouble)
		i.tape.data[i.tapeIdx+1] = math.Float64bits(v)
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to float", tag)
}

// SetInt changes the current value to an int64.
// Works on float, int, and uint elements (all use 2 tape entries).
func (i *Iter) SetInt(v int64) error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagDouble, tagInt64, tagUint64:
		i.tape.tapeSetTag(i.tapeIdx, tagInt64)
		i.tape.data[i.tapeIdx+1] = uint64(v)
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to int", tag)
}

// SetUInt changes the current value to a uint64.
// Works on float, int, and uint elements (all use 2 tape entries).
func (i *Iter) SetUInt(v uint64) error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagDouble, tagInt64, tagUint64:
		i.tape.tapeSetTag(i.tapeIdx, tagUint64)
		i.tape.data[i.tapeIdx+1] = v
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to uint", tag)
}

// SetStringBytes changes the current value to a string.
// Works on string, float, int, and uint elements (all use 2 tape entries).
// The new string is appended to the string buffer; the old value is orphaned.
func (i *Iter) SetStringBytes(v []byte) error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagString:
		// String → string: 1-entry type, just update the offset.
		off := i.tape.tapeAppendString(v)
		i.tape.tapeSetTagPayload(i.tapeIdx, tagString, off)
		return nil
	case tagDouble, tagInt64, tagUint64:
		// Number → string: 2-entry type shrinks to 1 entry.
		// First entry becomes the string, second becomes NOP.
		off := i.tape.tapeAppendString(v)
		i.tape.tapeSetTagPayload(i.tapeIdx, tagString, off)
		i.tape.tapeSetNop(i.tapeIdx+1, 1)
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to string", tag)
}

// SetString changes the current value to a string.
// Works on string, float, int, and uint elements (all use 2 tape entries).
func (i *Iter) SetString(v string) error {
	return i.SetStringBytes([]byte(v))
}

// SetBool changes the current value to a bool.
// Works on bool and null elements (all use 1 tape entry).
func (i *Iter) SetBool(v bool) error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagTrue, tagFalse, tagNull:
		if v {
			i.tape.tapeSetTag(i.tapeIdx, tagTrue)
		} else {
			i.tape.tapeSetTag(i.tapeIdx, tagFalse)
		}
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to bool", tag)
}

// SetNull changes the current value to null.
// Works on bool, string, number, object, and array elements.
// For 2-entry types (string, number), the second entry becomes a NOP.
// For containers (object, array), all entries through the closing tag become NOPs.
func (i *Iter) SetNull() error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagTrue, tagFalse, tagNull, tagString:
		// 1-entry types: just overwrite the tag.
		i.tape.tapeSetTag(i.tapeIdx, tagNull)
		return nil
	case tagDouble, tagInt64, tagUint64:
		// 2-entry types: first entry becomes null, second becomes NOP(skip=1).
		i.tape.tapeSetTag(i.tapeIdx, tagNull)
		i.tape.tapeSetNop(i.tapeIdx+1, 1)
		return nil
	case tagObject, tagArray:
		// Container: first entry becomes null, everything through closing tag becomes NOP.
		endIdx := int(i.tape.data[i.tapeIdx] & 0xffffffff)
		i.tape.tapeSetTag(i.tapeIdx, tagNull)
		i.tape.tapeNopRange(i.tapeIdx+1, endIdx+1)
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to null", tag)
}

// DeleteElems removes key-value pairs from the object where fn returns true.
// If onlyKeys is non-nil, only keys in the set are considered.
// Deleted entries are replaced with NOP entries in the tape.
func (o *Object) DeleteElems(fn func(key []byte, i Iter) bool, onlyKeys map[string]struct{}) error {
	if o.tobj == nil {
		return fmt.Errorf("nil object")
	}
	t := o.tobj.tape
	pos := o.tobj.startIdx
	n := 0
	for pos < o.tobj.endIdx {
		tag := t.tapeTagAt(pos)
		if tag == tagNop {
			pos = t.tapeSkipNop(pos)
			continue
		}
		if tag != tagString {
			break
		}
		startPos := pos
		keyBytes, err := t.readStringBytes(t.tapePayloadAt(pos))
		if err != nil {
			return err
		}
		pos++ // past key entry (string tag is 1 entry in DOM tape; length is next)

		if len(onlyKeys) > 0 {
			if _, ok := onlyKeys[string(keyBytes)]; !ok {
				pos = t.skipValue(pos)
				continue
			}
		}

		valueEnd := t.skipValue(pos)
		if fn == nil || fn(keyBytes, Iter{tape: t, tapeIdx: pos, copyStrings: o.copyStrings, useNumber: o.useNumber}) {
			// NOP-fill from key through value (inclusive).
			t.tapeNopRange(startPos, valueEnd)
		}
		pos = valueEnd
		n++
		if len(onlyKeys) > 0 && n == len(onlyKeys) {
			return nil
		}
	}
	return nil
}

// DeleteElems removes elements from the array where fn returns true.
// Deleted entries are replaced with NOP entries in the tape.
func (a *Array) DeleteElems(fn func(i Iter) bool) {
	t := a.tarr.tape
	pos := a.tarr.startIdx
	endIdx := a.tarr.endIdx
	for pos < endIdx {
		tag := t.tapeTagAt(pos)
		if tag == tagNop {
			pos = t.tapeSkipNop(pos)
			continue
		}
		valueEnd := t.skipValue(pos)
		if fn(Iter{tape: t, tapeIdx: pos, copyStrings: a.copyStrings}) {
			// NOP-fill the entire element.
			t.tapeNopRange(pos, valueEnd)
		}
		pos = valueEnd
	}
}

// AdvanceInto steps into a container (object/array) or advances to the next
// element, skipping NOP entries. Returns the Tag of the current element.
func (i *Iter) AdvanceInto() Tag {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	t := ti.AdvanceInto()
	i.tapeIdx = ti.idx
	if int(t) == -1 {
		return TagEnd
	}
	return Tag(t)
}

// MarshalJSON serializes the current element and its children to JSON bytes.
func (i *Iter) MarshalJSON() ([]byte, error) {
	return i.MarshalJSONBuffer(nil)
}

// MarshalJSONBuffer serializes the current element to JSON, using dst as
// the output buffer to reduce allocations.
func (i *Iter) MarshalJSONBuffer(dst []byte) ([]byte, error) {
	return marshalTape(i.tape, i.tapeIdx, dst)
}

// MarshalJSON serializes the array to JSON bytes.
func (a *Array) MarshalJSON() ([]byte, error) {
	return a.MarshalJSONBuffer(nil)
}

// MarshalJSONBuffer serializes the array to JSON, using dst as the output buffer.
func (a *Array) MarshalJSONBuffer(dst []byte) ([]byte, error) {
	// Array's TapeArray starts after the '[' tag, so back up one.
	return marshalTape(a.tarr.tape, a.tarr.startIdx-1, dst)
}

// MarshalJSON serializes the elements to JSON as an object.
func (e Elements) MarshalJSON() ([]byte, error) {
	return e.MarshalJSONBuffer(nil)
}

// MarshalJSONBuffer serializes the elements to JSON as an object.
func (e Elements) MarshalJSONBuffer(dst []byte) ([]byte, error) {
	dst = append(dst, '{')
	for i, elem := range e.Elements {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, '"')
		dst = appendEscaped(dst, elem.Name)
		dst = append(dst, '"', ':')
		var err error
		dst, err = marshalTape(elem.Iter.tape, elem.Iter.tapeIdx, dst)
		if err != nil {
			return nil, err
		}
	}
	dst = append(dst, '}')
	return dst, nil
}

// marshalTape walks the tape from idx and emits JSON bytes.
func marshalTape(t *Tape, idx int, dst []byte) ([]byte, error) {
	if idx >= len(t.data) {
		return dst, fmt.Errorf("tape index %d out of bounds", idx)
	}
	tag := t.tapeTagAt(idx)

	switch tag {
	case tagRoot:
		// Walk into root
		return marshalTape(t, idx+1, dst)

	case tagString:
		s, err := t.readString(t.tapePayloadAt(idx))
		if err != nil {
			return nil, err
		}
		dst = append(dst, '"')
		dst = appendEscaped(dst, s)
		dst = append(dst, '"')
		return dst, nil

	case tagInt64:
		v := int64(t.data[idx+1])
		return strconv.AppendInt(dst, v, 10), nil

	case tagUint64:
		v := t.data[idx+1]
		return strconv.AppendUint(dst, v, 10), nil

	case tagDouble:
		v := math.Float64frombits(t.data[idx+1])
		return appendJSONFloat(dst, v)

	case tagTrue:
		return append(dst, "true"...), nil
	case tagFalse:
		return append(dst, "false"...), nil
	case tagNull:
		return append(dst, "null"...), nil

	case tagObject:
		endIdx := int(t.data[idx] & 0xffffffff)
		dst = append(dst, '{')
		first := true
		pos := idx + 1
		for pos < endIdx-1 {
			ptag := t.tapeTagAt(pos)
			if ptag == tagNop {
				pos = t.tapeSkipNop(pos)
				continue
			}
			if ptag != tagString {
				break
			}
			if !first {
				dst = append(dst, ',')
			}
			first = false
			// Key
			key, err := t.readString(t.tapePayloadAt(pos))
			if err != nil {
				return nil, err
			}
			dst = append(dst, '"')
			dst = appendEscaped(dst, key)
			dst = append(dst, '"', ':')
			pos++
			// Value
			dst, err = marshalTape(t, pos, dst)
			if err != nil {
				return nil, err
			}
			pos = t.skipValue(pos)
		}
		dst = append(dst, '}')
		return dst, nil

	case tagArray:
		endIdx := int(t.data[idx] & 0xffffffff)
		dst = append(dst, '[')
		first := true
		pos := idx + 1
		for pos < endIdx-1 {
			ptag := t.tapeTagAt(pos)
			if ptag == tagNop {
				pos = t.tapeSkipNop(pos)
				continue
			}
			if !first {
				dst = append(dst, ',')
			}
			first = false
			var err error
			dst, err = marshalTape(t, pos, dst)
			if err != nil {
				return nil, err
			}
			pos = t.skipValue(pos)
		}
		dst = append(dst, ']')
		return dst, nil

	case tagNop:
		// Skip NOP entries
		return dst, nil

	case tagBigint:
		s, err := t.readString(t.tapePayloadAt(idx))
		if err != nil {
			return nil, err
		}
		return append(dst, s...), nil

	default:
		return nil, fmt.Errorf("unknown tag '%c' at index %d", tag, idx)
	}
}

// appendEscaped appends a JSON-escaped string to dst.
func appendEscaped(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			dst = append(dst, '\\', '"')
		case c == '\\':
			dst = append(dst, '\\', '\\')
		case c < 0x20:
			switch c {
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			default:
				dst = append(dst, '\\', 'u', '0', '0',
					"0123456789abcdef"[c>>4],
					"0123456789abcdef"[c&0xf])
			}
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

// appendJSONFloat appends a float64 as JSON to dst.
func appendJSONFloat(dst []byte, v float64) ([]byte, error) {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return nil, fmt.Errorf("INF or NaN number found")
	}
	// Use 'f' format if it round-trips, otherwise 'e'.
	abs := math.Abs(v)
	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = byte('e')
	}
	dst = strconv.AppendFloat(dst, v, format, -1, 64)
	return dst, nil
}
