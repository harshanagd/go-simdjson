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

## Benchmarks

Measured on Intel Xeon Platinum 8259CL @ 2.50GHz (x86_64, AVX2).
All go-simdjson benchmarks show **0 allocations**.

### Parse: go-simdjson vs encoding/json

| File | go-simdjson | encoding/json | Speedup | Allocs (stdlib) |
|------|------------|---------------|---------|-----------------|
| twitter.json (632KB) | 347µs (1822 MB/s) | 8.9ms (71 MB/s) | **25.7x** | 32,125 |
| canada.json (2.3MB) | 3.8ms (597 MB/s) | 46.1ms (49 MB/s) | **12.2x** | 392,515 |
| citm_catalog.json (1.7MB) | 878µs (1968 MB/s) | 21.3ms (81 MB/s) | **24.3x** | 95,865 |
| github_events.json (65KB) | 27µs (2445 MB/s) | 748µs (87 MB/s) | **28.1x** | 3,366 |
| apache_builds.json (127KB) | 62µs (2065 MB/s) | 1.6ms (82 MB/s) | **25.2x** | 9,720 |
| instruments.json (220KB) | 142µs (1547 MB/s) | 3.3ms (67 MB/s) | **23.1x** | 13,710 |
| mesh.json (724KB) | 1.3ms (550 MB/s) | 20.7ms (35 MB/s) | **15.7x** | 149,496 |
| numbers.json (150KB) | 247µs (609 MB/s) | 2.6ms (58 MB/s) | **10.5x** | 20,024 |
| random.json (510KB) | 456µs (1120 MB/s) | 9.8ms (52 MB/s) | **21.6x** | 68,020 |
| update-center.json (533KB) | 361µs (1476 MB/s) | 9.8ms (54 MB/s) | **27.2x** | 50,811 |

### Throughput Summary

| Metric | go-simdjson | encoding/json |
|--------|-----------|---------------|
| Peak throughput | 2.4 GB/s | 87 MB/s |
| Average speedup | **~20x** | baseline |
| Allocations | 0 | thousands |

### Interface() — Full Materialization

`Interface()` converts the entire document to Go native types (`map[string]interface{}`, etc.).
This crosses the CGo boundary per element, so the speedup is smaller than parse-only.
Use targeted navigation (`FindKey`, `ForEach`) for best performance.

| File | go-simdjson Interface() | encoding/json Unmarshal |
|------|------------------------|------------------------|
| twitter.json | 7.4ms (55K allocs) | 7.9ms (32K allocs) |
| github_events.json | 686µs (6K allocs) | 708µs (3K allocs) |
| mesh.json | 27.2ms (238K allocs) | 16.9ms (150K allocs) |

For full materialization, `encoding/json` is comparable or faster because `Interface()` incurs CGo overhead per element. The strength of go-simdjson is **parse + selective access** — parsing is 20x faster, and targeted key lookups avoid materializing the entire document.

## Roadmap

- [ ] `ParseND` — newline-delimited JSON (ndjson) support
- [ ] `MarshalJSON` — re-serialize parsed elements
- [ ] `Iter.FindElement` — path-based element lookup

## License

Apache 2.0 — same as simdjson.

simdjson is created by Daniel Lemire and Geoff Langdale. See [simdjson LICENSE](https://github.com/simdjson/simdjson/blob/master/LICENSE).
