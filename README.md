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

    fmt.Println("Root type:", pj.RootType())           // object
    fmt.Println("SIMD backend:", simdjson.ActiveImplementation()) // e.g. haswell

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

## Tree Walking

Navigate nested JSON using Iter, Object, and Array:

```go
pj, _ := simdjson.Parse(data, nil)
defer pj.Close()

iter, _ := pj.Iter()
obj, _ := iter.Object(nil)

// Find a key
elem := obj.FindKey("users", nil)
arr, _ := elem.Iter.Array(nil)

// Iterate array
arr.ForEach(func(i simdjson.Iter) error {
    userObj, _ := i.Object(nil)
    name := userObj.FindKey("name", nil)
    fmt.Println(name.Iter.String())
    return nil
})

// Or convert everything to Go native types
iter, _ = pj.Iter()
v, _ := iter.Interface() // map[string]interface{}, []interface{}, etc.
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
// Parse and pool management
func Parse(b []byte, reuse *ParsedJson) (*ParsedJson, error)
func GetParser() *ParsedJson
func PutParser(pj *ParsedJson)
func SupportedCPU() bool
func ActiveImplementation() string

// ParsedJson — root access
func (pj *ParsedJson) Iter() (Iter, error)
func (pj *ParsedJson) RootType() Type
func (pj *ParsedJson) RootCount() (int, error)
func (pj *ParsedJson) FindString(key string) (string, error)
func (pj *ParsedJson) RootString() (string, error)
func (pj *ParsedJson) RootInt64() (int64, error)
func (pj *ParsedJson) RootUint64() (uint64, error)
func (pj *ParsedJson) RootDouble() (float64, error)
func (pj *ParsedJson) RootBool() (bool, error)
func (pj *ParsedJson) Close()

// Iter — element access
func (i *Iter) Type() Type
func (i *Iter) String() (string, error)
func (i *Iter) Int() (int64, error)
func (i *Iter) Uint() (uint64, error)
func (i *Iter) Float() (float64, error)
func (i *Iter) Bool() (bool, error)
func (i *Iter) Object(reuse *Object) (*Object, error)
func (i *Iter) Array(reuse *Array) (*Array, error)
func (i *Iter) Interface() (interface{}, error)

// Object — key-value access
func (o *Object) FindKey(key string, reuse *Element) *Element
func (o *Object) ForEach(fn func(key string, i Iter) error) error
func (o *Object) Map(dst map[string]interface{}) (map[string]interface{}, error)
func (o *Object) Count() (int, error)

// Array — element access
func (a *Array) ForEach(fn func(i Iter) error) error
func (a *Array) Count() (int, error)
```

## Building and Testing

```bash
# Clone
git clone https://github.com/harshanagd/go-simdjson.git
cd go-simdjson

# Build
go build ./...

# Run tests
go test -v ./...

# Run benchmarks
go test -bench=. -benchmem -benchtime=2s ./...

# Run specific benchmark
go test -bench=BenchmarkParseFiles -benchmem ./...

# Run parse vs stdlib comparison
go test -bench="BenchmarkParseFiles$|BenchmarkStdlibUnmarshal" -benchmem ./...

# Run parallel benchmark
go test -bench=BenchmarkParseFilesParallel -benchmem ./...

# Lint (requires golangci-lint)
golangci-lint run ./...
```

## Architecture

go-simdjson uses a two-phase approach:

1. **Parse (CGo)**: A single CGo call invokes C++ simdjson which SIMD-parses the JSON into a tape (flat `[]uint64` array) and string buffer. This is the only CGo call.
2. **Navigate (pure Go)**: All navigation — `Type()`, `String()`, `FindKey()`, `ForEach()`, `Interface()` — is pure Go pointer arithmetic on the tape. Zero CGo overhead per element.

## Benchmarks

Measured on Intel Xeon Platinum 8259CL @ 2.50GHz (x86_64, AVX2).

### Parse Only: go-simdjson vs encoding/json

Parse benchmarks show **4 allocations** (parser + tape struct) regardless of document size.

| File | go-simdjson | encoding/json | Speedup |
|------|------------|---------------|---------|
| twitter.json (632KB) | 411µs (1538 MB/s) | 7.8ms (81 MB/s) | **19x** |
| canada.json (2.3MB) | 4.0ms (569 MB/s) | 42.8ms (53 MB/s) | **10.8x** |
| citm_catalog.json (1.7MB) | 1.0ms (1701 MB/s) | 19.9ms (87 MB/s) | **19.6x** |
| github_events.json (65KB) | 32µs (2043 MB/s) | 704µs (92 MB/s) | **22.1x** |
| apache_builds.json (127KB) | 68µs (1866 MB/s) | 1.5ms (87 MB/s) | **21.4x** |
| mesh.json (724KB) | 1.5ms (486 MB/s) | 16.9ms (43 MB/s) | **11.3x** |

### Full Materialization: Interface() vs encoding/json Unmarshal

`Interface()` converts the entire document to Go native types (`map[string]interface{}`, etc.)
using the pure Go tape walker — no CGo per element.

| File | go-simdjson Interface() | encoding/json Unmarshal | Speedup |
|------|------------------------|------------------------|---------|
| twitter.json | 2.0ms, 28K allocs | 7.8ms, 32K allocs | **3.9x** |
| canada.json | 7.8ms, 223K allocs | 42.8ms, 393K allocs | **5.5x** |
| citm_catalog.json | 5.0ms, 76K allocs | 19.9ms, 96K allocs | **4.0x** |
| github_events.json | 190µs, 3K allocs | 704µs, 3K allocs | **3.7x** |
| mesh.json | 2.3ms, 74K allocs | 16.9ms, 150K allocs | **7.4x** |
| numbers.json | 215µs, 10K allocs | 2.4ms, 20K allocs | **11.1x** |

### Throughput Summary

| Metric | go-simdjson | encoding/json |
|--------|-----------|---------------|
| Peak parse throughput | 2.0 GB/s | 92 MB/s |
| Parse allocations | 4 | thousands |
| Interface() speedup | **3.7x–11x** | baseline |

## Roadmap

- [ ] `ParseND` — newline-delimited JSON (ndjson) support
- [ ] `MarshalJSON` — re-serialize parsed elements
- [ ] `Iter.FindElement` — path-based element lookup

## License

Apache 2.0 — same as simdjson.

simdjson is created by Daniel Lemire and Geoff Langdale. See [simdjson LICENSE](https://github.com/simdjson/simdjson/blob/master/LICENSE).
