// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

//go:build !simdjson_static_cxx

package simdjson

// Default link mode: the C++ runtime stays a shared library.
//
// cgo links with the c++ driver, which supplies the runtime already, so this only
// names it. darwin must not name it: -lstdc++ there resolves to libc++ and
// duplicates the driver's own, warning on every consumer build.

// #cgo !darwin LDFLAGS: -lstdc++ -lm
// #cgo darwin LDFLAGS: -lm
import "C"

// staticCXX makes the two link files mutually exclusive and exhaustive; the
// reference in simdjson.go is what turns either mistake into a compile error.
const staticCXX = false
