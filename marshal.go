// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

import (
	"fmt"
	"math"
	"strconv"
)

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
		s, err := t.readStringBytes(t.tapePayloadAt(idx))
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
		return strconv.AppendInt(dst, int64(w), 10), nil //nolint:gosec // tagInt64 means the word holds an int64's raw bits

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
		endIdx := int(t.data[idx] & containerEndMask)
		dst = append(dst, '{')
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
			// Key
			key, err := t.readStringBytes(t.tapePayloadAt(pos))
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
		endIdx := int(t.data[idx] & containerEndMask)
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
		s, err := t.readStringBytes(t.tapePayloadAt(idx))
		if err != nil {
			return nil, err
		}
		return append(dst, s...), nil

	default:
		return nil, fmt.Errorf("unknown tag '%c' at index %d", tag, idx)
	}
}

// appendEscaped appends JSON-escaped text to dst. Taking either representation
// lets a tape walker pass the string bytes without materialising a string.
func appendEscaped[T ~string | ~[]byte](dst []byte, s T) []byte {
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
