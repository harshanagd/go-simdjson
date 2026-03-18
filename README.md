# go-simdjson

Cross-architecture SIMD JSON parser for Go, powered by [simdjson](https://github.com/simdjson/simdjson). Supports x86_64 (AVX2/SSE4.2) and ARM64 (NEON) with automatic runtime detection.

## Why?

The existing Go simdjson port ([minio/simdjson-go](https://github.com/minio/simdjson-go)) only supports x86_64. This library wraps the official C++ simdjson via CGo, giving you SIMD-accelerated JSON parsing on **every architecture** that simdjson supports — including ARM64 (AWS Graviton, Apple Silicon).

## Requirements

- Go 1.21+
- C++17 compiler (GCC 8+, Clang 7+, or Xcode 11+)
- CGo enabled (`CGO_ENABLED=1`, the default for native builds)

## Installation

```bash
go get github.com/harshanagd/go-simdjson
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    simdjson "github.com/harshanagd/go-simdjson"
)

func main() {
    data := []byte(`{"name": "simdjson", "fast": true}`)

    pj, err := simdjson.Parse(data, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer pj.Close()

    fmt.Println("Root type:", pj.RootType()) // object

    name, _ := pj.FindString("name")
    fmt.Println("Name:", name) // simdjson
}
```

## Parser Reuse

For best performance, reuse parsers via the built-in pool:

```go
pj := simdjson.GetParser()
defer simdjson.PutParser(pj)

pj, err := simdjson.Parse(data1, pj)
// use pj...

pj, err = simdjson.Parse(data2, pj) // reuses internal buffers
// use pj...
```

## Supported Platforms

| Architecture | SIMD Backend | Status |
|-------------|-------------|--------|
| x86_64 | AVX2, SSE4.2 | ✅ |
| ARM64 | NEON | ✅ |
| ppc64le | ALTIVEC | ✅ |
| Fallback | Portable | ✅ |

Runtime detection is automatic — no build flags needed.

## API

```go
// Parse JSON bytes. Reuse a ParsedJson for zero-allocation parsing.
func Parse(b []byte, reuse *ParsedJson) (*ParsedJson, error)

// Pool management.
func GetParser() *ParsedJson
func PutParser(pj *ParsedJson)

// Root element inspection.
func (pj *ParsedJson) RootType() Type
func (pj *ParsedJson) RootCount() (int, error)

// Value extraction from root.
func (pj *ParsedJson) FindString(key string) (string, error)
func (pj *ParsedJson) RootString() (string, error)
func (pj *ParsedJson) RootInt64() (int64, error)
func (pj *ParsedJson) RootUint64() (uint64, error)
func (pj *ParsedJson) RootDouble() (float64, error)
func (pj *ParsedJson) RootBool() (bool, error)

// Cleanup.
func (pj *ParsedJson) Close()
```

## Roadmap

- [ ] `Iter` — iterator for walking nested objects and arrays at any depth
- [ ] `Object.FindKey` — typed key lookup returning an `Iter`
- [ ] `ForEach` — iterate over root elements (ndjson support)
- [ ] `Interface()` — deserialize to `interface{}`
- [ ] `MarshalJSON()` — re-serialize parsed elements

## License

Apache 2.0 — same as simdjson.

simdjson is created by Daniel Lemire and Geoff Langdale. See [simdjson LICENSE](https://github.com/simdjson/simdjson/blob/master/LICENSE).
