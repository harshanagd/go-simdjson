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
// The result is safe to retain: tape strings live in Go-managed memory and remain
// valid for the lifetime of the Tape, regardless of WithCopyStrings.
func (i *Iter) String() (string, error) {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	return ti.String()
}

// StringRef extracts a string value without copying.
// Equivalent to String() — tape strings are stored in Go-managed memory and
// remain valid for the lifetime of the Tape.
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
	switch ti.Type() {
	case TypeObject, TypeArray:
		return "", fmt.Errorf("cannot convert %v to string", ti.Type())
	case TypeString:
		return ti.String()
	case TypeInt64:
		v, err := ti.Int()
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(v, 10), nil
	case TypeUint64:
		v, err := ti.Uint()
		if err != nil {
			return "", err
		}
		return strconv.FormatUint(v, 10), nil
	case TypeDouble:
		v, err := ti.Float()
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case TypeBool:
		v, err := ti.Bool()
		if err != nil {
			return "", err
		}
		if v {
			return "true", nil
		}
		return "false", nil
	case TypeNull:
		return "null", nil
	case TypeBigInt:
		v, err := ti.BigInt()
		return string(v), err
	default:
		return "", fmt.Errorf("cannot convert %v to string", ti.Type())
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
// When WithCopyStrings(true) (the default), the result is an independent copy.
// When false, it is a slice into the tape's string buffer whose capacity is
// pinned to its length, so appending to it reallocates rather than corrupting
// the adjacent string.
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
	if i.copyStrings {
		cp := make([]byte, len(b))
		copy(cp, b)
		return cp, nil
	}
	return b, nil
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
// Returns the type of the element, or Type(-1) with a nil error at end — the
// error return is reserved for future use and is never currently non-nil.
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
// NOP padding left behind by mutation is skipped, so this never reports TagNop.
// Returns TagEnd at end.
func (i *Iter) PeekNextTag() Tag {
	ti := TapeIter{tape: i.tape, idx: i.tapeIdx}
	if ti.Advance() == Type(-1) {
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
// Returns Type(-1) and an error when the cursor is not inside any document, in
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
		return Type(-1), dst, fmt.Errorf("iterator at tape index %d is not inside a root document", i.tapeIdx)
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
// calling Object() first. Returns empty name when done.
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error) {
	n, t, err := o.NextElementBytes(dst)
	return string(n), t, err
}

// NextElementBytes is like NextElement but returns the key as []byte,
// avoiding a string allocation. Returns nil name when done.
// The key is copied when WithCopyStrings(true) (the default); when false it is a
// cap-bounded slice into the tape's string buffer.
//
// NOP entries left behind by DeleteElems are skipped, so deleting a key does not
// end iteration early.
//
// Note: an object with an empty-string key ("") whose value is null yields an
// empty but non-nil name with TypeNull, which differs from the end-of-iteration
// sentinel only in that the name is non-nil. Callers that test len(name) == 0
// instead of name == nil will stop early on such an entry — use ForEach for
// objects that may contain one.
func (o *Object) NextElementBytes(dst *Iter) (name []byte, t Type, err error) {
	// Skip any NOP padding left by a prior delete before reading the key.
	o.iterPos = o.tobj.tape.skipNopsUntil(o.iterPos, o.tobj.endIdx)
	if o.iterPos >= o.tobj.endIdx {
		return nil, TypeNull, nil
	}
	if o.tobj.tape.tapeTagAt(o.iterPos) != tagString {
		return nil, TypeNull, nil
	}
	keyEntry := o.tobj.tape.data[o.iterPos]
	s, err := o.tobj.tape.readStringBytes(keyEntry & payloadMask)
	if err != nil {
		return nil, TypeNull, err
	}
	if o.copyStrings {
		cp := make([]byte, len(s))
		copy(cp, s)
		s = cp
	}
	valIdx := o.iterPos + 1
	if valIdx >= len(o.tobj.tape.data) {
		return nil, TypeNull, fmt.Errorf("truncated tape: key at %d has no value", o.iterPos)
	}
	if dst != nil {
		dst.tape = o.tobj.tape
		dst.tapeIdx = valIdx
		dst.copyStrings = o.copyStrings
		dst.useNumber = o.useNumber
	}
	// Tag.Type() is the single source of truth for tag-to-Type mapping: it folds
	// 'f' (false) into TypeBool and maps a zero tag to Type(-1).
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

// skipNopsUntil advances idx past any leading NOP entries, stopping before limit.
// Mutation (DeleteElems, and the setters when they shrink a value) leaves NOP
// padding on the tape; every walker must step over it before treating an index
// as a value. Bounded by limit so it is safe to call at a container's end.
func (t *Tape) skipNopsUntil(idx, limit int) int {
	for idx < limit && t.tapeTagAt(idx) == tagNop {
		idx = t.tapeSkipNop(idx)
	}
	return idx
}

// --- Root document blocks ---
//
// simdjson_parse_many lays out an NDJSON stream as one self-contained block per
// document, each of the form
//
//	[k]   tagRoot   payload = index one past the block's closing root
//	[k+1] the document's value (an object, array or scalar)
//	...
//	[e]   tagRoot   payload = k (points back to the opening root)
//	[e+1] NOT WRITTEN BY simdjson
//
// so the next document's opening root, if any, sits at payload+1. A tape from a
// single-document Parse is the same shape with exactly one block.
//
// The word at payload is the important trap: tape_len counts it, so Parse copies
// it, but simdjson never writes it and the C++ parser does not zero its tape
// between parses — it holds whatever a previous parse left there. Never infer
// structure from its contents; only the root payload chain is reliable.

// nextRootDoc returns the index of the opening root entry of the next READABLE
// root document after the one at rootIdx, or len(t.data) when there is none.
//
// "Readable" means the returned index is a root marker with a value entry after
// it, so callers may read at index+1 without a further bound check. It always
// makes progress, so it is safe to drive a loop.
func (t *Tape) nextRootDoc(rootIdx int) int {
	if !t.hasRootAt(rootIdx) {
		return len(t.data)
	}
	next := int(t.tapePayloadAt(rootIdx)) + 1
	if next <= rootIdx || !t.hasRootDocAt(next) {
		return len(t.data)
	}
	return next
}

// hasRootAt reports whether idx is in range and holds a root entry. Use this to
// recognise a root marker; use hasRootDocAt when you intend to read its value.
func (t *Tape) hasRootAt(idx int) bool {
	return idx >= 0 && idx < len(t.data) && t.tapeTagAt(idx) == tagRoot
}

// hasRootDocAt reports whether idx holds a readable root document: a root marker
// with a value entry following it.
//
// Both halves are load-bearing. A bare length check only confirms a value slot
// exists and never looks at the marker, so a tape with no root would pass it;
// hasRootAt alone confirms the marker but not that anything follows. Neither
// implies the other, and Serializer.Deserialize can produce either shape, since
// it validates length prefixes but not structure.
func (t *Tape) hasRootDocAt(idx int) bool {
	return t.hasRootAt(idx) && idx+1 < len(t.data)
}

// hasRootDoc reports whether the tape holds at least one readable root document.
// This is the guard for every entry point that reads from index 1 — RootType,
// Interface, InterfaceUseNumber and ForEach.
func (t *Tape) hasRootDoc() bool {
	return t.hasRootDocAt(0)
}

// rootDocContaining returns the index of the opening root entry of the document
// that contains idx, or -1 if idx is not inside any document.
//
// The block's extent is [root, payload): the opening root marker itself counts as
// inside, so a cursor already sitting on a root resolves to that document rather
// than being reported as outside one. The padding slot at payload does not count —
// nothing on the tape refers to it and simdjson never writes it.
//
// Root blocks are laid out in order and do not overlap, so this walks them from
// the start: O(documents before idx), and O(1) for a single-document tape. There
// is no back-pointer from an arbitrary entry to its enclosing root, so a scan is
// the only option — only the closing root points back, and a cursor deep inside a
// container cannot reach it cheaply.
func (t *Tape) rootDocContaining(idx int) int {
	for root := 0; t.hasRootDocAt(root); root = t.nextRootDoc(root) {
		if root > idx {
			break
		}
		if idx < int(t.tapePayloadAt(root)) {
			return root
		}
	}
	return -1
}

// skipRootBoundary advances a cursor that has landed on a root marker to the
// value of the next root document, or past the end of the tape when there is
// none. A cursor not on a root marker is returned unchanged.
//
// A closing root's payload points back to its opening root, whose payload in turn
// locates the end of the block — so the next document is found purely from the
// payload chain, without reading the uninitialised padding word.
func (t *Tape) skipRootBoundary(idx int) int {
	if !t.hasRootAt(idx) {
		return idx
	}
	payload := int(t.tapePayloadAt(idx))
	if payload > idx {
		// An opening root: its document's value is the next entry.
		return idx + 1
	}
	// A closing root: payload is its opening root.
	next := t.nextRootDoc(payload)
	if next >= len(t.data) {
		return len(t.data)
	}
	return next + 1
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

// AdvanceInto steps into a container (object/array), positioning at the first
// child and skipping any NOP padding left by mutation. Returns TagEnd if the
// element is not a container or the container has no remaining children.
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
		w, err := t.valueWord(idx)
		if err != nil {
			return dst, err
		}
		return strconv.AppendInt(dst, int64(w), 10), nil

	case tagUint64:
		w, err := t.valueWord(idx)
		if err != nil {
			return dst, err
		}
		return strconv.AppendUint(dst, w, 10), nil

	case tagDouble:
		w, err := t.valueWord(idx)
		if err != nil {
			return dst, err
		}
		return appendJSONFloat(dst, math.Float64frombits(w))

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
