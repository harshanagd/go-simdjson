// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"fmt"
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
	s, err := i.String()
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
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
	if o.iterPos >= o.tobj.endIdx {
		return "", TypeNull, nil // done
	}
	keyEntry := o.tobj.tape.data[o.iterPos]
	if byte(keyEntry>>56) != tagString {
		return "", TypeNull, nil
	}
	name, _ = o.tobj.tape.readString(keyEntry & payloadMask)
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
	return name, t, nil
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
