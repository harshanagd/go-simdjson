// Copyright 2026 harshanagd
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

//go:build simdjson_static_cxx

package simdjson

// Opt-in link mode (-tags simdjson_static_cxx): bind libstdc++ into the binary so
// it runs on hosts whose C++ runtime predates the build host's. glibc stays
// dynamic, so this is not a fully static build.
//
// Do not add -lstdc++ here. -static-libstdc++ only redirects the c++ driver's
// implicit runtime, so an explicit one would resolve dynamically and silently
// defeat the tag. On darwin the flag is a no-op and libc++ is always dynamic.

// #cgo !darwin LDFLAGS: -static-libstdc++ -lm
// #cgo darwin LDFLAGS: -lm
import "C"

// See link_dynamic.go.
const staticCXX = true
