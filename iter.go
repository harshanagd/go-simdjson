// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"encoding/json"
	"fmt"
	"math"
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

// Flags converts the flag to FloatFlags, merging any additional flags given.
func (f FloatFlag) Flags(more ...FloatFlag) FloatFlags {
	out := FloatFlags(f)
	for _, m := range more {
		out |= FloatFlags(m)
	}
	return out
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
	tobj        TapeObject
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

// Type returns the JSON type of the current element.
func (i *Iter) Type() Type {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.Type()
}

// String extracts a string value from the current element.
// The result is safe to retain: tape strings live in Go-managed memory and remain
// valid for the lifetime of the Tape, regardless of WithCopyStrings.
func (i *Iter) String() (string, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.String()
}

// StringRef extracts a string value without copying.
// Equivalent to String(), and kept for compatibility with simdjson-go where the
// two differ. Here a string read from a zero-copy tape is copied regardless, so
// the result is always safe to retain.
func (i *Iter) StringRef() (string, error) {
	return i.String()
}

// StringCvt converts any scalar value to its JSON string representation.
//
// Floats are formatted with strconv.FormatFloat(v, 'g', -1, 64), which round-trips
// the stored float64 exactly. That is not the same as round-tripping the source
// token: the tape holds the parsed float64, not the original text, so `1e2` comes
// back as "100", and a literal carrying more precision than a float64 can hold was
// already lossy before this call. Big integers are exempt — they are stored as
// their original digits and returned verbatim.
func (i *Iter) StringCvt() (string, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.StringCvt()
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

// BigInt returns a JSON integer too large for int64 or uint64 as a json.Number.
// Only valid on TypeBigInt elements, which appear when UseBigInt or UseNumber is set.
func (i *Iter) BigInt() (json.Number, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.BigInt()
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

// element wraps a tape position as an Element. This is the only place reuse is
// written, so a lookup that fails before reaching here leaves it untouched.
func (o *Object) element(name string, ti TapeIter, reuse *Element) *Element {
	t := ti.Type()
	iter := Iter{tape: ti.tape, tapeIdx: ti.idx, copyStrings: o.copyStrings, useNumber: o.useNumber}
	if reuse != nil {
		reuse.Name = name
		reuse.Type = t
		reuse.Iter = iter
		return reuse
	}
	return &Element{Name: name, Type: t, Iter: iter}
}

// FindKey finds a key in the object and returns an Element.
// Returns nil if the key is not found.
//
// A non-nil reuse is overwritten and returned, invalidating any Element already
// sharing that storage.
func (o *Object) FindKey(key string, reuse *Element) *Element {
	ti, ok := o.tobj.FindKey(key)
	if !ok {
		return nil
	}
	return o.element(key, ti, reuse)
}

// ForEach iterates over all key-value pairs in O(n) time, or only those named in
// onlyKeys when it is non-empty. The key is valid for the duration of the call.
//
// The callback cannot abort the walk; use NextElementBytes for that. The returned
// error is a failed key read, which a tape from Deserialize can still produce —
// Validate does not range-check string payload offsets.
func (o *Object) ForEach(fn func(key []byte, i Iter), onlyKeys map[string]struct{}) error {
	return o.tobj.ForEach(func(key []byte, val TapeIter) error {
		if len(onlyKeys) > 0 {
			if _, ok := onlyKeys[string(key)]; !ok {
				return nil
			}
		}
		fn(key, Iter{tape: val.tape, tapeIdx: val.idx, copyStrings: o.copyStrings, useNumber: o.useNumber})
		return nil
	})
}

// Count returns the number of key-value pairs in the object. DeleteElems keeps it
// current. The tape's count field saturates at 16777215, so for an object larger
// than that this is a lower bound and deletions do not lower it.
func (o *Object) Count() (int, error) {
	return o.tobj.Count(), nil
}

// Array represents a JSON array for element access.
type Array struct {
	tarr        TapeArray
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
//
// The callback cannot abort the walk and a read failure is not reported. Use the
// tape layer's TapeArray.Iter when you need either.
func (a *Array) ForEach(fn func(i Iter)) {
	_ = a.tarr.ForEach(func(val TapeIter) error {
		fn(Iter{tape: val.tape, tapeIdx: val.idx, copyStrings: a.copyStrings, useNumber: a.useNumber})
		return nil
	})
}

// Count returns the number of elements in the array. DeleteElems keeps it current.
// The tape's count field saturates at 16777215, so for an array larger than that
// this is a lower bound and deletions do not lower it.
func (a *Array) Count() (int, error) {
	return a.tarr.Count(), nil
}

// FirstType returns the type of the first element without consuming it, or TypeNone
// for an empty array. NOP padding left by DeleteElems is skipped.
func (a *Array) FirstType() Type {
	return a.tarr.FirstType()
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
// The result is an independent copy under WithCopyStrings(true) (the default),
// and also on a zero-copy tape whatever the flag says. Only a cloned tape with
// the flag off returns a slice into the string buffer, whose capacity is pinned
// to its length so appending reallocates rather than corrupting the next string.
func (i *Iter) StringBytes() ([]byte, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	if ti.pastEnd() {
		return nil, fmt.Errorf("iterator past end of tape")
	}
	if ti.tag() != tagString {
		return nil, fmt.Errorf("element is not a string")
	}
	b, err := ti.tape.readStringBytes(ti.payload())
	if err != nil {
		return nil, err
	}
	if i.copyStrings || i.tape.pj != nil {
		cp := make([]byte, len(b))
		copy(cp, b)
		return cp, nil
	}
	return b, nil
}

// FindPath navigates a path of object keys from the current element.
//
// A non-nil reuse is overwritten and returned, invalidating any Element already
// sharing that storage. Only the final result is written, so a failed lookup
// leaves reuse intact.
func (o *Object) FindPath(reuse *Element, path ...string) (*Element, error) {
	ti, err := o.tobj.findPath(path)
	if err != nil {
		return nil, err
	}
	return o.element(path[len(path)-1], ti, reuse), nil
}

// FindElement navigates a path of object keys from the root.
// reuse follows Object.FindPath's contract.
func (i *Iter) FindElement(reuse *Element, path ...string) (*Element, error) {
	obj, err := i.Object(nil)
	if err != nil {
		return nil, err
	}
	return obj.FindPath(reuse, path...)
}

// Advance moves to the next sibling element and returns its type.
// Returns TypeNone at end.
func (i *Iter) Advance() Type {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	t := ti.Advance()
	i.tapeIdx = ti.idx
	return t
}

// AdvanceIter advances and copies the current element into dst.
// Returns the type of the element, or TypeNone with a nil error at end — the
// error return is reserved for future use and is never currently non-nil.
func (i *Iter) AdvanceIter(dst *Iter) (Type, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	t := ti.Advance()
	if t == TypeNone {
		return TypeNone, nil
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
// NOP padding left behind by mutation is skipped, so this never reports TagNop.
// Returns TagEnd at end.
func (i *Iter) PeekNextTag() Tag {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	if ti.Advance() == TypeNone {
		return TagEnd
	}
	return Tag(ti.tag())
}

// Root returns an Iter positioned at the value of the root document that contains
// the cursor.
//
// For a tape from Parse there is one document, so this is that document's value.
// For a tape from ParseND it is the document the cursor is currently inside — it
// used to hardcode tape index 1 and so always returned the FIRST document, with a
// nil error, no matter where the cursor was.
//
// Returns TypeNone and an error when the cursor is not inside any document, in
// which case dst is positioned past the end of the tape. Cost is O(documents
// before the cursor), and O(1) for a single-document tape.
func (i *Iter) Root(dst *Iter) (Type, *Iter, error) {
	if dst == nil {
		c := *i
		dst = &c
	} else {
		*dst = *i
	}
	root := i.tape.rootDocContaining(i.tapeIdx)
	if root < 0 {
		// Keep dst usable rather than nil: this method never used to fail, so a
		// caller that ignores the error must not be handed a nil pointer.
		dst.tapeIdx = len(i.tape.data)
		return TypeNone, dst, fmt.Errorf("iterator at tape index %d is not inside a root document", i.tapeIdx)
	}
	dst.tapeIdx = root + 1
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
// calling Object() first. At end it returns TypeNone; the name is "" then, but so
// is a legitimate empty key, so test the type rather than the name.
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error) {
	n, t, err := o.NextElementBytes(dst)
	return string(n), t, err
}

// NextElementBytes is like NextElement but returns the key as []byte,
// avoiding a string allocation. At end it returns a nil name with TypeNone, the
// same end-of-tape sentinel Advance and TapeIter.Type use; no real entry can carry
// that type, so testing it is safe where testing len(name) == 0 is not.
// The key is copied when WithCopyStrings(true) (the default); when false it is a
// cap-bounded slice into the tape's string buffer.
//
// NOP entries left behind by DeleteElems are skipped, so deleting a key does not
// end iteration early.
func (o *Object) NextElementBytes(dst *Iter) (name []byte, t Type, err error) {
	// Skip any NOP padding left by a prior delete before reading the key.
	o.iterPos = o.tobj.tape.skipNopsUntil(o.iterPos, o.tobj.endIdx)
	if o.iterPos >= o.tobj.endIdx {
		return nil, TypeNone, nil
	}
	keyEntry := o.tobj.tape.data[o.iterPos]
	s, err := o.tobj.tape.readStringBytes(keyEntry & payloadMask)
	if err != nil {
		return nil, TypeNone, err
	}
	if o.copyStrings || o.tobj.tape.pj != nil {
		cp := make([]byte, len(s))
		copy(cp, s)
		s = cp
	}
	valIdx := o.iterPos + 1
	if valIdx >= len(o.tobj.tape.data) {
		return nil, TypeNone, fmt.Errorf("truncated tape: key at %d has no value", o.iterPos)
	}
	if dst != nil {
		dst.tape = o.tobj.tape
		dst.tapeIdx = valIdx
		dst.copyStrings = o.copyStrings
		dst.useNumber = o.useNumber
	}
	// Tag.Type() is the single source of truth for tag-to-Type mapping: it folds
	// 'f' (false) into TypeBool and maps a zero tag to TypeNone.
	t = Tag(o.tobj.tape.tapeTagAt(valIdx)).Type()
	o.iterPos = o.tobj.tape.skipValue(valIdx)
	return s, t, nil
}

// Elements contains all key-value pairs of an object, kept in original order.
type Elements struct {
	Elements []Element
	Index    map[string]int
}

// Lookup returns the element with the given key, or nil if not found.
//
// The result points into the Elements slice and is only valid until the next
// Object.Parse into that same Elements, which overwrites entries in place — the
// slice does not have to grow. Copy it (`e := *els.Lookup(k)`) to outlive that.
func (e Elements) Lookup(key string) *Element {
	idx, ok := e.Index[key]
	if !ok {
		return nil
	}
	return &e.Elements[idx]
}

// Parse collects all key-value pairs into an Elements collection.
// A non-nil dst is reused, invalidating every *Element from its Lookup.
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
		if t == TypeNone {
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
	return a.tarr.Interface()
}

// AsFloat returns all elements as []float64.
func (a *Array) AsFloat() ([]float64, error) {
	return a.tarr.AsFloat()
}

// AsInteger returns all elements as []int64.
func (a *Array) AsInteger() ([]int64, error) {
	return a.tarr.AsInteger()
}

// AsUint64 returns all elements as []uint64.
func (a *Array) AsUint64() ([]uint64, error) {
	return a.tarr.AsUint64()
}

// AsString returns all elements as []string.
func (a *Array) AsString() ([]string, error) {
	return a.tarr.AsString()
}

// AsStringCvt returns all elements converted to strings via StringCvt.
func (a *Array) AsStringCvt() ([]string, error) {
	return a.tarr.AsStringCvt()
}

// --- Mutation methods ---

// SetFloat changes the current value to a float64.
// Works on float, int, and uint elements (all use 2 tape entries).
func (i *Iter) SetFloat(v float64) error {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	tag := ti.tag()
	switch tag {
	case tagDouble, tagInt64, tagUint64:
		if !i.tape.hasValueWord(i.tapeIdx) {
			return fmt.Errorf("truncated tape: numeric entry at %d has no value word", i.tapeIdx)
		}
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
		if !i.tape.hasValueWord(i.tapeIdx) {
			return fmt.Errorf("truncated tape: numeric entry at %d has no value word", i.tapeIdx)
		}
		i.tape.tapeSetTag(i.tapeIdx, tagInt64)
		i.tape.data[i.tapeIdx+1] = uint64(v) //nolint:gosec // the tape stores an int64 as its raw bits; tagInt64 marks how to read it back
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
		if !i.tape.hasValueWord(i.tapeIdx) {
			return fmt.Errorf("truncated tape: numeric entry at %d has no value word", i.tapeIdx)
		}
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
	// The tape's string format prefixes the length as a uint32, so a longer string
	// is unrepresentable: appending it would wrap the prefix and read back truncated.
	if uint64(len(v)) > math.MaxUint32 {
		return fmt.Errorf("string of %d bytes exceeds the tape's %d-byte limit", len(v), uint64(math.MaxUint32))
	}
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
		if !i.tape.hasValueWord(i.tapeIdx) {
			return fmt.Errorf("truncated tape: numeric entry at %d has no value word", i.tapeIdx)
		}
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
		if !i.tape.hasValueWord(i.tapeIdx) {
			return fmt.Errorf("truncated tape: numeric entry at %d has no value word", i.tapeIdx)
		}
		i.tape.tapeSetTag(i.tapeIdx, tagNull)
		i.tape.tapeSetNop(i.tapeIdx+1, 1)
		return nil
	case tagObject, tagArray:
		// Container: first entry becomes null, everything through closing tag becomes NOP.
		endIdx := int(i.tape.data[i.tapeIdx] & containerEndMask)
		i.tape.tapeSetTag(i.tapeIdx, tagNull)
		// endIdx is one PAST the closing tag, so the exclusive bound is endIdx: an
		// extra entry here overwrites whatever follows the container, which destroys
		// the next key or element unless the container is its parent's last child.
		i.tape.tapeNopRange(i.tapeIdx+1, endIdx)
		return nil
	}
	return fmt.Errorf("cannot set tag '%c' to null", tag)
}

// DeleteElems removes key-value pairs from the object where fn returns true.
// If onlyKeys is non-nil, only keys in the set are considered.
// Deleted entries are replaced with NOP entries in the tape, and the object's
// element count is rewritten from the surviving pairs.
func (o *Object) DeleteElems(fn func(key []byte, i Iter) bool, onlyKeys map[string]struct{}) error {
	if o.tobj.tape == nil {
		return fmt.Errorf("nil object")
	}
	t := o.tobj.tape
	hdr := o.tobj.startIdx - 1
	count := int((t.data[hdr] >> 32) & containerCountMask)
	// A saturated header carries no usable total, so this object cannot take the O(1)
	// shortcut below: it declines the early exit and is walked in full, which restores
	// an exact count whenever one now fits.
	saturated := count == containerCountMask

	pos := o.tobj.startIdx
	n := 0
	deleted, survived, walkedAll := 0, 0, true
	for pos < o.tobj.endIdx {
		tag := t.tapeTagAt(pos)
		if tag == tagNop {
			pos = t.tapeSkipNop(pos)
			continue
		}
		startPos := pos
		keyBytes, err := t.readStringBytes(t.tapePayloadAt(pos))
		if err != nil {
			// A bad payload abandons the walk part-way. Counting the untouched tail is
			// O(tail), but it is exact even for a saturated header, and this path only
			// runs on a tape whose string offsets are already corrupt.
			t.tapeSetContainerCount(hdr, survived+t.tapeCountObjectEntries(pos, o.tobj.endIdx))
			return err
		}
		pos++ // past key entry (string tag is 1 entry in DOM tape; length is next)

		if len(onlyKeys) > 0 {
			if _, ok := onlyKeys[string(keyBytes)]; !ok {
				pos = t.skipValue(pos)
				survived++
				continue
			}
		}

		valueEnd := t.skipValue(pos)
		if fn == nil || fn(keyBytes, Iter{tape: t, tapeIdx: pos, copyStrings: o.copyStrings, useNumber: o.useNumber}) {
			// NOP-fill from key through value (inclusive).
			t.tapeNopRange(startPos, valueEnd)
			deleted++
		} else {
			survived++
		}
		pos = valueEnd
		n++
		if len(onlyKeys) > 0 && n == len(onlyKeys) && !saturated {
			walkedAll = false
			break
		}
	}
	if walkedAll {
		t.tapeSetContainerCount(hdr, survived)
	} else {
		// Both tape sources declare an exact count -- simdjson at parse time, Validate at
		// the deserialize boundary -- and each deletion consumed a distinct element, so
		// this cannot go negative. Saturated headers never reach here.
		t.tapeSetContainerCount(hdr, count-deleted)
	}
	return nil
}

// DeleteElems removes elements from the array where fn returns true.
// Deleted entries are replaced with NOP entries in the tape, and the array's
// element count is rewritten from the surviving elements.
func (a *Array) DeleteElems(fn func(i Iter) bool) {
	t := a.tarr.tape
	pos := a.tarr.startIdx
	endIdx := a.tarr.endIdx
	survived := 0
	for pos < endIdx {
		tag := t.tapeTagAt(pos)
		if tag == tagNop {
			pos = t.tapeSkipNop(pos)
			continue
		}
		valueEnd := t.skipValue(pos)
		if fn(Iter{tape: t, tapeIdx: pos, copyStrings: a.copyStrings, useNumber: a.useNumber}) {
			// NOP-fill the entire element.
			t.tapeNopRange(pos, valueEnd)
		} else {
			survived++
		}
		pos = valueEnd
	}
	t.tapeSetContainerCount(a.tarr.startIdx-1, survived)
}

// AdvanceInto steps into a container (object/array), positioning at the first
// child and skipping any NOP padding left by mutation. Returns TagEnd if the
// element is not a container or the container has no remaining children.
func (i *Iter) AdvanceInto() Tag {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	t := ti.AdvanceInto()
	i.tapeIdx = ti.idx
	if t == TypeNone {
		return TagEnd
	}
	return Tag(t) //nolint:gosec // a Type other than the TypeNone sentinel above holds a tag byte
}
