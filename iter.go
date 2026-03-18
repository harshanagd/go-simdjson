// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package simdjson

// #include "bridge.h"
import "C"

import (
	"fmt"
	"unsafe"
)

// Iter represents a position in the parsed JSON document.
type Iter struct {
	elem C.simdjson_element
}

// Object represents a JSON object for key-value access.
type Object struct {
	elem C.simdjson_element
}

// Element is a key-value pair result from object lookup.
type Element struct {
	Iter Iter
}

// Iter returns an Iter positioned at the root of the parsed document.
func (pj *ParsedJson) Iter() (Iter, error) {
	var elem C.simdjson_element
	rc := C.simdjson_get_root(pj.parser, &elem)
	if rc != 0 {
		return Iter{}, fmt.Errorf("no parsed document")
	}
	return Iter{elem: elem}, nil
}

// Type returns the JSON type of the current element.
func (i *Iter) Type() Type {
	return Type(C.simdjson_element_type(i.elem))
}

// String extracts a string value from the current element.
func (i *Iter) String() (string, error) {
	var out *C.char
	var outLen C.size_t
	rc := C.simdjson_element_get_string(i.elem, &out, &outLen)
	if rc != 0 {
		return "", fmt.Errorf("element is not a string")
	}
	return C.GoStringN(out, C.int(outLen)), nil
}

// Int returns the element value as int64.
func (i *Iter) Int() (int64, error) {
	var out C.int64_t
	rc := C.simdjson_element_get_int64(i.elem, &out)
	if rc != 0 {
		return 0, fmt.Errorf("element is not an int64")
	}
	return int64(out), nil
}

// Uint returns the element value as uint64.
func (i *Iter) Uint() (uint64, error) {
	var out C.uint64_t
	rc := C.simdjson_element_get_uint64(i.elem, &out)
	if rc != 0 {
		return 0, fmt.Errorf("element is not a uint64")
	}
	return uint64(out), nil
}

// Float returns the element value as float64.
func (i *Iter) Float() (float64, error) {
	var out C.double
	rc := C.simdjson_element_get_double(i.elem, &out)
	if rc != 0 {
		return 0, fmt.Errorf("element is not a double")
	}
	return float64(out), nil
}

// Bool returns the element value as bool.
func (i *Iter) Bool() (bool, error) {
	var out C.int
	rc := C.simdjson_element_get_bool(i.elem, &out)
	if rc != 0 {
		return false, fmt.Errorf("element is not a bool")
	}
	return out != 0, nil
}

// Object returns the element as an Object for key-value access.
func (i *Iter) Object(reuse *Object) (*Object, error) {
	if i.Type() != TypeObject {
		return nil, fmt.Errorf("element is not an object")
	}
	if reuse != nil {
		reuse.elem = i.elem
		return reuse, nil
	}
	return &Object{elem: i.elem}, nil
}

// FindKey finds a key in the object and returns an Element.
// Returns nil if the key is not found.
func (o *Object) FindKey(key string, reuse *Element) *Element {
	var out C.simdjson_element
	rc := C.simdjson_object_find_key(o.elem,
		(*C.char)(unsafe.Pointer(unsafe.StringData(key))), C.size_t(len(key)),
		&out)
	if rc != 0 {
		return nil
	}
	if reuse != nil {
		reuse.Iter.elem = out
		return reuse
	}
	return &Element{Iter: Iter{elem: out}}
}

// NextElement iterates over object key-value pairs.
// Call with idx 0..Count()-1.
func (o *Object) NextElement(idx int, dst *Iter) (name string, t Type, err error) {
	var outKey *C.char
	var outKeyLen C.size_t
	var outVal C.simdjson_element
	rc := C.simdjson_object_iter(o.elem, C.size_t(idx), &outKey, &outKeyLen, &outVal)
	if rc != 0 {
		return "", TypeNull, fmt.Errorf("index %d out of range", idx)
	}
	if dst != nil {
		dst.elem = outVal
	}
	key := C.GoStringN(outKey, C.int(outKeyLen))
	typ := Type(C.simdjson_element_type(outVal))
	return key, typ, nil
}

// Count returns the number of key-value pairs in the object.
func (o *Object) Count() (int, error) {
	var out C.size_t
	rc := C.simdjson_object_get_count(o.elem, &out)
	if rc != 0 {
		return 0, fmt.Errorf("element is not an object")
	}
	return int(out), nil
}
