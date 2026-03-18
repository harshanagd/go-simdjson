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
	elem        C.simdjson_element
	copyStrings bool
}

// Object represents a JSON object for key-value access.
type Object struct {
	elem        C.simdjson_element
	copyStrings bool
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
	return Iter{elem: elem, copyStrings: pj.copyStrings}, nil
}

// Type returns the JSON type of the current element.
func (i *Iter) Type() Type {
	return Type(C.simdjson_element_type(i.elem))
}

// String extracts a string value from the current element.
// If WithCopyStrings(false) was set, the string points into parser-owned
// memory and is only valid until the next Parse call or Close.
func (i *Iter) String() (string, error) {
	var out *C.char
	var outLen C.size_t
	rc := C.simdjson_element_get_string(i.elem, &out, &outLen)
	if rc != 0 {
		return "", fmt.Errorf("element is not a string")
	}
	if i.copyStrings {
		return C.GoStringN(out, C.int(outLen)), nil
	}
	return unsafe.String((*byte)(unsafe.Pointer(out)), int(outLen)), nil
}

// StringRef extracts a string value without copying.
// The returned string points into parser-owned memory and is only valid
// until the next Parse call or Close.
func (i *Iter) StringRef() (string, error) {
	var out *C.char
	var outLen C.size_t
	rc := C.simdjson_element_get_string(i.elem, &out, &outLen)
	if rc != 0 {
		return "", fmt.Errorf("element is not a string")
	}
	return unsafe.String((*byte)(unsafe.Pointer(out)), int(outLen)), nil
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
		reuse.copyStrings = i.copyStrings
		return reuse, nil
	}
	return &Object{elem: i.elem, copyStrings: i.copyStrings}, nil
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
		reuse.Iter.copyStrings = o.copyStrings
		return reuse
	}
	return &Element{Iter: Iter{elem: out, copyStrings: o.copyStrings}}
}

// ForEach iterates over all key-value pairs in O(n) time.
func (o *Object) ForEach(fn func(key string, i Iter) error) error {
	var it C.simdjson_obj_iter
	rc := C.simdjson_object_iter_begin(o.elem, &it)
	if rc != 0 {
		return fmt.Errorf("element is not an object")
	}
	var outKey *C.char
	var outKeyLen C.size_t
	var outVal C.simdjson_element
	for {
		rc = C.simdjson_object_iter_next(&it, &outKey, &outKeyLen, &outVal)
		if rc == 1 {
			return nil
		}
		if rc != 0 {
			return fmt.Errorf("iteration error")
		}
		key := unsafe.String((*byte)(unsafe.Pointer(outKey)), int(outKeyLen))
		if o.copyStrings {
			key = C.GoStringN(outKey, C.int(outKeyLen))
		}
		if err := fn(key, Iter{elem: outVal, copyStrings: o.copyStrings}); err != nil {
			return err
		}
	}
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

// Array represents a JSON array for element access.
type Array struct {
	elem        C.simdjson_element
	copyStrings bool
}

// Array returns the element as an Array.
func (i *Iter) Array(reuse *Array) (*Array, error) {
	if i.Type() != TypeArray {
		return nil, fmt.Errorf("element is not an array")
	}
	if reuse != nil {
		reuse.elem = i.elem
		reuse.copyStrings = i.copyStrings
		return reuse, nil
	}
	return &Array{elem: i.elem, copyStrings: i.copyStrings}, nil
}

// ForEach iterates over all elements in O(n) time.
func (a *Array) ForEach(fn func(i Iter) error) error {
	var it C.simdjson_arr_iter
	rc := C.simdjson_array_iter_begin(a.elem, &it)
	if rc != 0 {
		return fmt.Errorf("element is not an array")
	}
	var outVal C.simdjson_element
	for {
		rc = C.simdjson_array_iter_next(&it, &outVal)
		if rc == 1 {
			return nil
		}
		if rc != 0 {
			return fmt.Errorf("iteration error")
		}
		if err := fn(Iter{elem: outVal, copyStrings: a.copyStrings}); err != nil {
			return err
		}
	}
}

// Count returns the number of elements in the array.
func (a *Array) Count() (int, error) {
	var out C.size_t
	rc := C.simdjson_array_get_count(a.elem, &out)
	if rc != 0 {
		return 0, fmt.Errorf("element is not an array")
	}
	return int(out), nil
}

// Interface converts the element to its Go native equivalent:
// object → map[string]interface{}, array → []interface{},
// string → string, int64/uint64 → int64/uint64, double → float64,
// bool → bool, null → nil.
func (i *Iter) Interface() (interface{}, error) {
	switch i.Type() {
	case TypeObject:
		var obj Object
		obj.elem = i.elem
		obj.copyStrings = i.copyStrings
		return obj.Map(nil)
	case TypeArray:
		var arr Array
		arr.elem = i.elem
		arr.copyStrings = i.copyStrings
		var result []interface{}
		err := arr.ForEach(func(elem Iter) error {
			v, err := elem.Interface()
			if err != nil {
				return err
			}
			result = append(result, v)
			return nil
		})
		return result, err
	case TypeString:
		return i.String()
	case TypeInt64:
		return i.Int()
	case TypeUint64:
		return i.Uint()
	case TypeDouble:
		return i.Float()
	case TypeBool:
		return i.Bool()
	case TypeNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown type %v", i.Type())
	}
}

// Map converts the object to a map[string]interface{}.
// If dst is non-nil it is reused (cleared first).
func (o *Object) Map(dst map[string]interface{}) (map[string]interface{}, error) {
	if dst == nil {
		n, _ := o.Count()
		dst = make(map[string]interface{}, n)
	}
	err := o.ForEach(func(key string, val Iter) error {
		v, err := val.Interface()
		if err != nil {
			return err
		}
		dst[key] = v
		return nil
	})
	return dst, err
}
