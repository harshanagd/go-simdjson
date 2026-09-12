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
	TagBigInt      = Tag('Z')

	payloadMask = 0x00ffffffffffffff

	// Container entry layout: [tag:8 | count:24 | endIdx:32]
	// endIdx = one past the closing '}' or ']' tape entry (exclusive end).
	// count  = number of elements (array) or key-value pairs (object); saturates at 0xffffff.
	containerEndMask   = 0xffffffff // lower 32 bits: end index
	containerCountMask = 0xffffff   // bits 32-55 (after >>32): element count

	// maxTapeDepth bounds container nesting. Tape.validate enforces it so the
	// recursive tape readers cannot be driven into a fatal stack overflow by a
	// deserialized tape. It is simdjson's DEFAULT_MAX_DEPTH; Parse realises at most
	// one level less than this, so the bound rejects nothing Parse can emit.
	maxTapeDepth = 1024

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
	tagBigint = byte(TagBigInt)
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
//     string is stored as a 4-byte native-endian length prefix followed by the
//     UTF-8 bytes and a null terminator. This is NOT the original input buffer.
//
// Both C++ buffers are owned by the parser and are overwritten on the next
// Parse() call or freed on Close(). The Tape does NOT point into them: Parse
// copies both into Go-managed slices, so a Tape and every string obtained from it
// remain valid for the lifetime of the Tape, independently of the C++ parser.
type Tape struct {
	data        []uint64
	strings     []byte
	copyStrings bool
	useNumber   bool
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
	return &Tape{data: d, strings: s, copyStrings: t.copyStrings, useNumber: t.useNumber}
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
// Returns Type(-1) when the tape holds no root document, or for a zero tag.
func (t *Tape) RootType() Type {
	if !t.hasRootDoc() {
		return Type(-1)
	}
	return Tag(t.tapeTagAt(1)).Type()
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
// Returns Type(-1) when the cursor is past the end of the tape.
func (ti *TapeIter) Type() Type {
	// Tag.Type() is the single source of truth for the tag-to-Type mapping: it
	// folds 'f' (false) into TypeBool and maps a zero tag to Type(-1). tag()
	// already yields a zero tag when the cursor is past end.
	return Tag(ti.tag()).Type()
}

// String returns the string value at the current position.
func (ti *TapeIter) String() (string, error) {
	if ti.tag() != tagString {
		return "", fmt.Errorf("element is not a string")
	}
	return ti.tape.readString(ti.payload())
}

// BigInt returns the big integer as a json.Number at the current position.
func (ti *TapeIter) BigInt() (json.Number, error) {
	if ti.tag() != tagBigint {
		return "", fmt.Errorf("element is not a big integer")
	}
	s, err := ti.tape.readString(ti.payload())
	return json.Number(s), err
}

// Int returns the int64 value at the current position.
func (ti *TapeIter) Int() (int64, error) {
	if ti.tag() != tagInt64 {
		return 0, fmt.Errorf("element is not an int64")
	}
	w, err := ti.tape.valueWord(ti.idx)
	if err != nil {
		return 0, err
	}
	return int64(w), nil
}

// Uint returns the uint64 value at the current position.
func (ti *TapeIter) Uint() (uint64, error) {
	tag := ti.tag()
	if tag != tagUint64 && tag != tagInt64 {
		return 0, fmt.Errorf("element is not a uint64")
	}
	w, err := ti.tape.valueWord(ti.idx)
	if err != nil {
		return 0, err
	}
	if tag == tagUint64 {
		return w, nil
	}
	if v := int64(w); v >= 0 {
		return uint64(v), nil
	}
	return 0, fmt.Errorf("element is not a uint64")
}

// Float returns the float64 value at the current position.
func (ti *TapeIter) Float() (float64, error) {
	tag := ti.tag()
	if tag != tagDouble && tag != tagInt64 && tag != tagUint64 {
		return 0, fmt.Errorf("element is not a double")
	}
	w, err := ti.tape.valueWord(ti.idx)
	if err != nil {
		return 0, err
	}
	switch tag {
	case tagDouble:
		return math.Float64frombits(w), nil
	case tagInt64:
		return float64(int64(w)), nil
	default:
		return float64(w), nil
	}
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
func (ti *TapeIter) Object() (obj TapeObject, err error) {
	if ti.pastEnd() {
		return obj, fmt.Errorf("iterator past end of tape")
	}
	if ti.tag() != tagObject {
		return obj, fmt.Errorf("element is not an object")
	}
	entry := ti.tape.data[ti.idx]
	endIdx := int(entry & containerEndMask)
	return TapeObject{tape: ti.tape, startIdx: ti.idx + 1, endIdx: endIdx - 1}, nil
}

// Array returns a TapeArray for element access.
func (ti *TapeIter) Array() (arr TapeArray, err error) {
	if ti.pastEnd() {
		return arr, fmt.Errorf("iterator past end of tape")
	}
	if ti.tag() != tagArray {
		return arr, fmt.Errorf("element is not an array")
	}
	entry := ti.tape.data[ti.idx]
	endIdx := int(entry & containerEndMask)
	return TapeArray{tape: ti.tape, startIdx: ti.idx + 1, endIdx: endIdx - 1}, nil
}

// validate checks the structural invariants every tape walker relies on.
//
// This is the package's trust boundary. Parse produces a tape from simdjson's own
// structural pass, so its output always satisfies these invariants and the walkers
// index the tape directly on that basis — no walker re-checks a bound. A tape rebuilt
// by Serializer.Deserialize is arbitrary bytes, so it must be validated here before
// any walker touches it.
//
// What it does NOT establish: that an object's entries alternate key then value. It
// classifies entries, and a string is just an entry, so a deserialized object can hold
// a bare value and still pass here. The object readers report that as
// "expected string key at N" — those checks are load-bearing, not redundant.
//
// The walk inspects every word rather than following NOP skips, and requires each skip
// to stay inside its own run, because the readers DO honour skips: the two must agree
// on where an entry begins. It never reads the word after a closing root — simdjson
// does not write that padding, so it holds whatever was previously in that memory, and
// reading it as a tag could reject a VALID tape. It is iterative rather than recursive
// because nesting depth here is untrusted (Deserialize bypasses simdjson's depth
// limit).
func (t *Tape) validate() error {
	if len(t.data) == 0 {
		return nil
	}
	idx := 0
	for idx < len(t.data) {
		if byte(t.data[idx]>>56) != tagRoot {
			return fmt.Errorf("tape index %d: expected a root marker", idx)
		}
		// The opening root's payload is one past its closing root.
		closeIdx := int(t.data[idx]&payloadMask) - 1
		if closeIdx <= idx || closeIdx >= len(t.data) {
			return fmt.Errorf("root at %d: invalid closing index %d", idx, closeIdx+1)
		}
		if byte(t.data[closeIdx]>>56) != tagRoot {
			return fmt.Errorf("root at %d: no closing root at %d", idx, closeIdx)
		}
		if int(t.data[closeIdx]&payloadMask) != idx {
			return fmt.Errorf("closing root at %d does not point back to %d", closeIdx, idx)
		}
		if err := t.validateEntries(idx+1, closeIdx); err != nil {
			return err
		} // Skip the closing root and its uninitialised padding word.
		idx = closeIdx + 2
	}
	return nil
}

// validateEntries checks every real entry in [start, end).
//
// It inspects every word rather than following NOP skips, and tracks open containers
// on an explicit stack so a container's declared end must be exactly where its
// matching closing tag sits. Both matter because the readers traverse by jumping to a
// container's end index, not by walking tags in order: honouring a NOP's skip here
// would let a hostile skip hide entries a reader still reaches, and checking a close
// tag only pointwise would let an end index point somewhere it does not close. The
// stack also gives the true nesting depth, which is the depth the recursive readers
// experience.
func (t *Tape) validateEntries(start, end int) error {
	// stack holds the end index of each open container, so the innermost container's
	// closing tag sits at stack[len-1]-1.
	stack := make([]int, 0, 16)
	pos := start
	for pos < end {
		// Entries must fall inside the innermost open container, or the block itself.
		limit := end
		if n := len(stack); n > 0 {
			limit = stack[n-1] - 1
		}

		switch tag := byte(t.data[pos] >> 56); tag {
		case tagNop:
			// Readers DO honour a NOP's skip (tapeSkipNop), so a skip must land where
			// this walk's next entry begins. Otherwise a reader can be steered onto a
			// word the walk consumed opaquely — a numeric's value word is 64 bits of
			// tape data that is never tag-checked, so landing there hands the reader an
			// entry of the attacker's choosing.
			//
			// The bound is the end of the contiguous NOP run rather than each skip
			// agreeing on a single target: two adjacent deletes merge into one run
			// whose skips point at different ends (measured: skips 3,2,1 then 3,2,1),
			// and that is a tape Parse plus DeleteElems really produces.
			runEnd := pos
			for runEnd < limit && byte(t.data[runEnd]>>56) == tagNop {
				runEnd++
			}
			for j := pos; j < runEnd; j++ {
				skip := int(t.data[j] & payloadMask)
				if skip == 0 {
					skip = 1 // tapeSkipNop floors a zero skip
				}
				// Compared as a distance so a 56-bit skip cannot overflow the addition
				// on a 32-bit platform.
				if skip > runEnd-j {
					return fmt.Errorf("NOP at %d: skip %d leaves its run, which ends at %d", j, skip, runEnd)
				}
			}
			pos = runEnd

		case tagObject, tagArray:
			endIdx := int(t.data[pos] & containerEndMask)
			// Smallest legal value is pos+2, an empty container whose closing tag sits
			// at pos+1. endIdx is exclusive, so endIdx == len(t.data) is legal.
			if endIdx > len(t.data) || endIdx < pos+2 {
				return fmt.Errorf("container at %d: invalid end index %d", pos, endIdx)
			}
			// A child must close before its parent does.
			if endIdx > limit {
				return fmt.Errorf("container at %d: end index %d escapes its parent", pos, endIdx)
			}
			want := tagObjEnd
			if tag == tagArray {
				want = tagArrEnd
			}
			if got := byte(t.data[endIdx-1] >> 56); got != want {
				return fmt.Errorf("container at %d: end index %d does not close it (found %q)", pos, endIdx, got)
			}
			// The count sizes result collections, so an inflated one buys a large
			// preallocation. Every element occupies at least one entry, so the
			// container's extent bounds it; the field saturates rather than wrapping,
			// so a real count never exceeds this.
			if count := int((t.data[pos] >> 32) & containerCountMask); count > endIdx-pos {
				return fmt.Errorf("container at %d: count %d exceeds its extent", pos, count)
			}
			// readValue, readObject, readArray and marshalTape recurse once per level,
			// and a stack overflow is fatal — recover cannot catch it. simdjson will not
			// parse deeper than this, so the bound rejects nothing Parse can emit.
			if len(stack) >= maxTapeDepth {
				return fmt.Errorf("container at %d: nesting deeper than %d", pos, maxTapeDepth)
			}
			stack = append(stack, endIdx)
			pos++

		case tagObjEnd, tagArrEnd:
			n := len(stack)
			if n == 0 {
				return fmt.Errorf("tape index %d: closing tag with no open container", pos)
			}
			if stack[n-1] != pos+1 {
				return fmt.Errorf("tape index %d: closing tag is not where end index %d points", pos, stack[n-1])
			}
			stack = stack[:n-1]
			pos++

		case tagInt64, tagUint64, tagDouble:
			// The value word must sit inside the same container, or a reader would take
			// a closing tag as the value.
			if pos+1 >= limit {
				return fmt.Errorf("numeric entry at %d has no value word", pos)
			}
			pos += 2

		case tagString, tagBigint:
			// String payloads are deliberately NOT validated here. readStringBytes is
			// the only path to the string buffer and must read the length prefix to
			// slice at all, so its bounds check is intrinsic rather than an extra
			// guard — validating offsets here would repeat it and cost 45% of this
			// walk. A bad offset surfaces as an error from the read instead.
			pos++

		case tagTrue, tagFalse, tagNull:
			pos++

		default:
			return fmt.Errorf("tape index %d: unknown tag %q", pos, tag)
		}
	}
	if n := len(stack); n > 0 {
		// A backstop, not a reachable branch on current logic: every container's end
		// index is bounded by its parent's close, and the numeric bound keeps the
		// cursor from straddling a closing tag, so the walk lands on each close and
		// pops it. Kept because it is one comparison and it fails closed if either of
		// those arguments stops holding.
		return fmt.Errorf("%d container(s) left unclosed at tape index %d", n, end)
	}
	return nil
}

// Interface converts the element to its Go native equivalent.
// Respects the UseNumber() option set during Parse — numeric values are
// returned as json.Number when it is enabled.
func (ti *TapeIter) Interface() (interface{}, error) {
	if ti.tape.useNumber {
		val, _, err := ti.tape.readValueNum(ti.idx)
		return val, err
	}
	val, _, err := ti.tape.readValue(ti.idx)
	return val, err
}

// pastEnd reports whether the cursor is outside the tape. Iter() on an empty or
// fully deleted container deliberately returns such a cursor, so any method that
// reads the tape before checking a bound must consult this first.
func (ti *TapeIter) pastEnd() bool {
	return ti.idx < 0 || ti.idx >= len(ti.tape.data)
}

// valueWord returns the second word of a two-word tape entry at idx — the raw
// numeric payload of a tagInt64, tagUint64 or tagDouble entry.
//
// A tape truncated between a numeric tag word and its value word cannot come out
// of Parse, but Serializer.Deserialize reconstructs a tape from arbitrary bytes
// with only length-prefix checks and no structural validation, so a corrupt
// payload can produce one. Every read of the second word goes through here so
// that case surfaces as an error rather than an index-out-of-range panic.
//
// The presence test cannot look at the word itself: the second word of a numeric
// entry is raw value bits, not a tape entry, so its top byte is not a tag —
// it is zero for any value below 2^56. Only the slice length can answer this.
func (t *Tape) valueWord(idx int) (uint64, error) {
	if !t.hasValueWord(idx) {
		return 0, fmt.Errorf("truncated tape: numeric entry at %d has no value word", idx)
	}
	return t.data[idx+1], nil
}

// hasValueWord reports whether the two-word entry at idx has its value word. This
// is the single bound test for the second word; valueWord is its error-returning
// form for readers, while the mutation setters call it directly because they
// overwrite that word rather than reading it.
func (t *Tape) hasValueWord(idx int) bool {
	return idx >= 0 && idx+1 < len(t.data)
}

// tag returns the tag byte at the cursor, or 0 (tagEnd) when the cursor is past
// the end of the tape. Returning tagEnd rather than panicking means every
// accessor's tag comparison simply fails, so an exhausted iterator — including
// one from Iter() on an empty or fully deleted container — yields an error
// instead of an index-out-of-range panic.
func (ti *TapeIter) tag() byte {
	if ti.pastEnd() {
		return byte(TagEnd)
	}
	return byte(ti.tape.data[ti.idx] >> 56)
}

func (ti *TapeIter) payload() uint64 { return ti.tape.data[ti.idx] & payloadMask }

// skipValue returns the tape index after the value at idx.
//
// A container's end index is taken from the tape and returned unchanged, with no
// progress check. That is safe only because Tape.validate has established that every
// reader-reachable container's end index is at least idx+2, so the cursor always
// advances. See Tape.validate.
func (t *Tape) skipValue(idx int) int {
	tag := t.tapeTagAt(idx)
	switch tag {
	case tagNop:
		return t.tapeSkipNop(idx)
	case tagObject, tagArray:
		return int(t.data[idx] & containerEndMask) // end index (past closing tag)
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

// FindKey finds a key in the object. Returns the zero TapeIter and false if not found.
// NOP padding left behind by DeleteElems is skipped.
//
// Unlike ForEach, a non-string tag where a key belongs is reported as "not found"
// rather than as an error, because this signature has nowhere to put one. That state is
// reachable: Tape.validate classifies entries and does not enforce that objects
// alternate key then value, so a deserialized object can hold a bare value. Use ForEach
// or Map if a malformed object must be distinguished from an absent key.
func (o *TapeObject) FindKey(key string) (iter TapeIter, ok bool) {
	pos := o.tape.skipNopsUntil(o.startIdx, o.endIdx)
	for pos < o.endIdx {
		if o.tape.tapeTagAt(pos) != tagString {
			break
		}
		// A key that fails to decode cannot match, so skip past it. FindKey has
		// no error return; use ForEach if you need the failure reported.
		k, err := o.tape.readString(o.tape.tapePayloadAt(pos))
		valIdx := pos + 1
		if err == nil && k == key {
			return TapeIter{tape: o.tape, idx: valIdx}, true
		}
		pos = o.tape.skipNopsUntil(o.tape.skipValue(valIdx), o.endIdx)
	}
	return iter, false
}

// ForEach iterates over all key-value pairs.
// NOP padding left behind by DeleteElems is skipped.
//
// A non-string tag where a key belongs is reported rather than treated as the end of
// iteration, matching Tape.readObject so the two layers agree. This is NOT a redundant
// guard: Tape.validate does not enforce that objects alternate key then value, so a
// deserialized tape can hold an object containing a bare value and still pass the
// boundary. Reporting it is what keeps that from becoming a silently short result.
//
// It is not needed for termination — the pos < endIdx bound handles that, and removing
// this check breaks no test that uses a well-formed tape. So once validation covers
// alternation, this check and its three siblings (FindKey, readObject, readObjectNum,
// plus marshalTape) should be DELETED rather than kept as backstops: the layers should
// agree by all trusting the tape, not by all re-checking it.
func (o *TapeObject) ForEach(fn func(key string, val TapeIter) error) error {
	pos := o.tape.skipNopsUntil(o.startIdx, o.endIdx)
	for pos < o.endIdx {
		if o.tape.tapeTagAt(pos) != tagString {
			return fmt.Errorf("expected string key at %d", pos)
		}
		key, err := o.tape.readString(o.tape.tapePayloadAt(pos))
		if err != nil {
			return err
		}
		valIdx := pos + 1
		if err := fn(key, TapeIter{tape: o.tape, idx: valIdx}); err != nil {
			return err
		}
		pos = o.tape.skipNopsUntil(o.tape.skipValue(valIdx), o.endIdx)
	}
	return nil
}

// Map converts the object to map[string]interface{}.
// Respects the UseNumber() option set during Parse.
// If dst is non-nil, entries are added to it: existing keys are overwritten but
// extra keys are not deleted. Pass nil for a fresh map.
// On error the returned map is the partially populated dst, since a caller-supplied
// dst has already been written to by the time a later entry fails.
func (o *TapeObject) Map(dst map[string]interface{}) (map[string]interface{}, error) {
	if dst == nil {
		dst = make(map[string]interface{}, o.Count())
	}
	// Delegate to ForEach rather than walking the range here: it skips the NOP
	// padding DeleteElems leaves behind (an open-coded walk stops at the first
	// deleted key), and val.Interface() honours UseNumber for free.
	err := o.ForEach(func(key string, val TapeIter) error {
		v, err := val.Interface()
		if err != nil {
			return err
		}
		dst[key] = v
		return nil
	})
	return dst, err
}

// Count returns the number of key-value pairs.
// This is the count recorded at parse time; it may be stale after tape mutations
// (deleted entries become NOPs but the header count is not decremented).
func (o *TapeObject) Count() int {
	return int((o.tape.data[o.startIdx-1] >> 32) & containerCountMask)
}

// findPath walks a path of object keys and returns the final value, naming the key
// that failed. FindPath and Object.FindPath share it so the descent exists once.
func (o *TapeObject) findPath(path []string) (iter TapeIter, err error) {
	if len(path) == 0 {
		return iter, fmt.Errorf("empty path")
	}
	cur := *o
	for _, key := range path[:len(path)-1] {
		ti, ok := cur.FindKey(key)
		if !ok {
			return iter, fmt.Errorf("key %q not found", key)
		}
		next, err := ti.Object()
		if err != nil {
			return iter, fmt.Errorf("key %q: %w", key, err)
		}
		cur = next
	}
	last := path[len(path)-1]
	ti, ok := cur.FindKey(last)
	if !ok {
		return iter, fmt.Errorf("key %q not found", last)
	}
	return ti, nil
}

// FindPath finds a nested value by a path of object keys.
// ok is false if any key is missing or an intermediate value is not an object.
func (o *TapeObject) FindPath(path ...string) (iter TapeIter, ok bool) {
	iter, err := o.findPath(path)
	if err != nil {
		return TapeIter{}, false
	}
	return iter, true
}

// TapeArray provides element access over the tape.
type TapeArray struct {
	tape     *Tape
	startIdx int // first element index
	endIdx   int // index of closing ']'
}

// ForEach iterates over all elements.
// NOP padding left behind by DeleteElems is skipped, both before the first
// element and after each advance, so fn is only ever invoked on real values.
func (a *TapeArray) ForEach(fn func(val TapeIter) error) error {
	pos := a.tape.skipNopsUntil(a.startIdx, a.endIdx)
	for pos < a.endIdx {
		if err := fn(TapeIter{tape: a.tape, idx: pos}); err != nil {
			return err
		}
		pos = a.tape.skipNopsUntil(a.tape.skipValue(pos), a.endIdx)
	}
	return nil
}

// Count returns the number of elements.
// This is the count recorded at parse time; it may be stale after tape mutations
// (deleted entries become NOPs but the header count is not decremented).
func (a *TapeArray) Count() int {
	return int((a.tape.data[a.startIdx-1] >> 32) & containerCountMask)
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
// Respects the UseNumber() option set during Parse.
// For an NDJSON tape this converts the FIRST document only — use
// ParsedJson.ForEach to reach the rest.
func (t *Tape) Interface() (interface{}, error) {
	if !t.hasRootDoc() {
		return nil, fmt.Errorf("tape holds no root document")
	}
	if t.useNumber {
		val, _, err := t.readValueNum(1)
		return val, err
	}
	val, _, err := t.readValue(1)
	return val, err
}

// InterfaceUseNumber is like Interface but returns json.Number for numeric values.
func (t *Tape) InterfaceUseNumber() (interface{}, error) {
	if !t.hasRootDoc() {
		return nil, fmt.Errorf("tape holds no root document")
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
	case tagNop:
		return nil, t.tapeSkipNop(idx), nil
	case tagString:
		s, err := t.readString(payload)
		return s, idx + 1, err
	case tagInt64:
		w, err := t.valueWord(idx)
		if err != nil {
			return nil, idx, err
		}
		return int64(w), idx + 2, nil
	case tagUint64:
		w, err := t.valueWord(idx)
		if err != nil {
			return nil, idx, err
		}
		return w, idx + 2, nil
	case tagDouble:
		w, err := t.valueWord(idx)
		if err != nil {
			return nil, idx, err
		}
		return math.Float64frombits(w), idx + 2, nil
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
	case tagBigint:
		s, err := t.readString(payload)
		return json.Number(s), idx + 1, err
	default:
		return nil, idx + 1, fmt.Errorf("unknown tag '%c' at index %d", tag, idx)
	}
}

func (t *Tape) readObject(idx int) (map[string]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & containerEndMask)
	count := int((entry >> 32) & containerCountMask)
	result := make(map[string]interface{}, count)
	pos := t.skipNopsUntil(idx+1, endIdx-1)
	for pos < endIdx-1 {
		if t.tapeTagAt(pos) != tagString {
			return nil, pos, fmt.Errorf("expected string key at %d", pos)
		}
		key, err := t.readString(t.tapePayloadAt(pos))
		if err != nil {
			return nil, pos, err
		}
		pos++
		val, nextPos, err := t.readValue(pos)
		if err != nil {
			return nil, pos, err
		}
		result[key] = val
		pos = t.skipNopsUntil(nextPos, endIdx-1)
	}
	return result, endIdx, nil
}

func (t *Tape) readArray(idx int) ([]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & containerEndMask)
	count := int((entry >> 32) & containerCountMask)
	result := make([]interface{}, 0, count)
	pos := t.skipNopsUntil(idx+1, endIdx-1)
	for pos < endIdx-1 {
		val, nextPos, err := t.readValue(pos)
		if err != nil {
			return nil, pos, err
		}
		result = append(result, val)
		pos = t.skipNopsUntil(nextPos, endIdx-1)
	}
	return result, endIdx, nil
}

func (t *Tape) readString(offset uint64) (string, error) {
	b, err := t.readStringBytes(offset)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", nil
	}
	if t.copyStrings {
		return string(b), nil
	}
	return unsafe.String(&b[0], len(b)), nil
}

// readStringBytes reads a string from the string buffer at the given offset.
// Format: [4-byte native-endian length][UTF-8 content bytes][null terminator].
//
// The returned slice has its capacity pinned to its length, so a caller's append
// reallocates instead of writing past the string into the adjacent entry's
// length prefix, which would corrupt every string after it in the buffer.
func (t *Tape) readStringBytes(offset uint64) ([]byte, error) {
	off := int(offset)
	if off+4 > len(t.strings) {
		return nil, fmt.Errorf("string offset %d out of bounds", off)
	}
	slen := int(binary.NativeEndian.Uint32(t.strings[off : off+4]))
	start := off + 4
	if start+slen > len(t.strings) {
		return nil, fmt.Errorf("string length %d at offset %d out of bounds", slen, off)
	}
	b := t.strings[start : start+slen]
	return b[:len(b):len(b)], nil
}

func (t *Tape) readValueNum(idx int) (interface{}, int, error) {
	if idx >= len(t.data) {
		return nil, idx, fmt.Errorf("tape index %d out of bounds", idx)
	}
	entry := t.data[idx]
	tag := byte(entry >> 56)
	payload := entry & payloadMask

	switch tag {
	case tagNop:
		return nil, t.tapeSkipNop(idx), nil
	case tagString:
		s, err := t.readString(payload)
		return s, idx + 1, err
	case tagInt64:
		w, err := t.valueWord(idx)
		if err != nil {
			return nil, idx, err
		}
		return json.Number(strconv.FormatInt(int64(w), 10)), idx + 2, nil
	case tagUint64:
		w, err := t.valueWord(idx)
		if err != nil {
			return nil, idx, err
		}
		return json.Number(strconv.FormatUint(w, 10)), idx + 2, nil
	case tagDouble:
		w, err := t.valueWord(idx)
		if err != nil {
			return nil, idx, err
		}
		return json.Number(strconv.FormatFloat(math.Float64frombits(w), 'g', -1, 64)), idx + 2, nil
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
	case tagBigint:
		s, err := t.readString(payload)
		return json.Number(s), idx + 1, err
	default:
		return nil, idx + 1, fmt.Errorf("unknown tag '%c' at index %d", tag, idx)
	}
}

func (t *Tape) readObjectNum(idx int) (map[string]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & containerEndMask)
	count := int((entry >> 32) & containerCountMask)
	result := make(map[string]interface{}, count)
	pos := t.skipNopsUntil(idx+1, endIdx-1)
	for pos < endIdx-1 {
		if t.tapeTagAt(pos) != tagString {
			return nil, pos, fmt.Errorf("expected string key at %d", pos)
		}
		key, err := t.readString(t.tapePayloadAt(pos))
		if err != nil {
			return nil, pos, err
		}
		pos++
		val, nextPos, err := t.readValueNum(pos)
		if err != nil {
			return nil, pos, err
		}
		result[key] = val
		pos = t.skipNopsUntil(nextPos, endIdx-1)
	}
	return result, endIdx, nil
}

func (t *Tape) readArrayNum(idx int) ([]interface{}, int, error) {
	entry := t.data[idx]
	endIdx := int(entry & containerEndMask)
	count := int((entry >> 32) & containerCountMask)
	result := make([]interface{}, 0, count)
	pos := t.skipNopsUntil(idx+1, endIdx-1)
	for pos < endIdx-1 {
		val, nextPos, err := t.readValueNum(pos)
		if err != nil {
			return nil, pos, err
		}
		result = append(result, val)
		pos = t.skipNopsUntil(nextPos, endIdx-1)
	}
	return result, endIdx, nil
}

// Advance moves the iterator to the next sibling element.
// NOP padding left behind by mutation is skipped, as is the root-marker
// scaffolding between NDJSON documents — so on a ParseND tape this steps from one
// document's value to the next rather than stopping on the boundary.
//
// It operates on the full tape with no container boundary — it will walk past
// closing brackets into subsequent entries. For bounded iteration within an
// object or array, use TapeObject.ForEach / TapeArray.ForEach instead.
func (ti *TapeIter) Advance() Type {
	// skipValue reads the tag at the cursor, so the bound must be checked on the
	// INPUT, not just the result. An exhausted cursor is reachable from Iter() on
	// an empty or fully deleted container.
	if ti.pastEnd() {
		return Type(-1)
	}
	next := ti.tape.skipNopsUntil(ti.tape.skipValue(ti.idx), len(ti.tape.data))
	ti.idx = ti.tape.skipRootBoundary(next)
	if ti.pastEnd() {
		return Type(-1)
	}
	return ti.Type()
}

// PeekNext returns the type of the next sibling without advancing.
// NOP padding left behind by mutation is skipped.
//
// Like Advance, it is unbounded — use TapeObject/TapeArray methods for
// container-scoped iteration.
func (ti *TapeIter) PeekNext() Type {
	next := *ti
	return next.Advance()
}

// AdvanceInto steps into a container (object/array), positioning at the first child.
// Returns Type(-1) if the element is not a container, the iterator is past end,
// or the container is empty (e.g. [] or {}) — including an container left empty
// by DeleteElems, whose children are NOP padding.
func (ti *TapeIter) AdvanceInto() Type {
	tag := ti.tag()
	if tag != tagObject && tag != tagArray {
		return Type(-1)
	}
	ti.idx = ti.tape.skipNopsUntil(ti.idx+1, len(ti.tape.data))
	if ti.pastEnd() {
		return Type(-1)
	}
	// A closing bracket here means the container has no remaining children.
	switch ti.tag() {
	case tagObjEnd, tagArrEnd, tagRoot:
		return Type(-1)
	}
	return ti.Type()
}

// FindElement navigates a path of object keys from the current element.
func (ti *TapeIter) FindElement(path ...string) (iter TapeIter, ok bool) {
	if len(path) == 0 {
		return iter, false
	}
	obj, err := ti.Object()
	if err != nil {
		return iter, false
	}
	return obj.FindPath(path...)
}

// StringCvt converts any scalar to its string representation.
func (ti *TapeIter) StringCvt() (string, error) {
	switch ti.Type() {
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
		return strconv.FormatBool(v), nil
	case TypeNull:
		return "null", nil
	case TypeBigInt:
		v, err := ti.BigInt()
		return string(v), err
	default:
		return "", fmt.Errorf("cannot convert %v to string", ti.Type())
	}
}

// Iter returns a TapeIter at the first element of the array.
// NOP padding left behind by DeleteElems is skipped. For an empty array — or one
// left empty by deletion — the returned iterator is positioned past end, so
// Type() returns Type(-1).
func (a *TapeArray) Iter() TapeIter {
	pos := a.tape.skipNopsUntil(a.startIdx, a.endIdx)
	if pos >= a.endIdx {
		return TapeIter{tape: a.tape, idx: len(a.tape.data)}
	}
	return TapeIter{tape: a.tape, idx: pos}
}

// FirstType returns the type of the first element, or Type(-1) if empty.
// NOP padding left behind by DeleteElems is skipped.
func (a *TapeArray) FirstType() Type {
	pos := a.tape.skipNopsUntil(a.startIdx, a.endIdx)
	if pos >= a.endIdx {
		return Type(-1)
	}
	return Tag(a.tape.tapeTagAt(pos)).Type()
}

// Interface returns the array as []interface{}.
// Respects the UseNumber() option set during Parse.
func (a *TapeArray) Interface() ([]interface{}, error) {
	// Delegate to the shared array walker, which skips the NOP padding that
	// DeleteElems leaves behind. Walking the range here instead would append a
	// nil per NOP, since readValue reports "nothing here" as a nil value.
	if a.tape.useNumber {
		result, _, err := a.tape.readArrayNum(a.startIdx - 1)
		return result, err
	}
	result, _, err := a.tape.readArray(a.startIdx - 1)
	return result, err
}

// Iter returns a TapeIter at the first key of the object.
// NOP padding left behind by DeleteElems is skipped. For an empty object — or one
// left empty by deletion — the returned iterator is positioned past end, so
// Type() returns Type(-1).
func (o *TapeObject) Iter() TapeIter {
	pos := o.tape.skipNopsUntil(o.startIdx, o.endIdx)
	if pos >= o.endIdx {
		return TapeIter{tape: o.tape, idx: len(o.tape.data)}
	}
	return TapeIter{tape: o.tape, idx: pos}
}
