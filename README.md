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

    fmt.Println("Root type:", pj.RootType())
    fmt.Println("SIMD backend:", simdjson.ActiveImplementation())

    name, _ := pj.FindString("name")
    fmt.Println("Name:", name)
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

## Tape Navigation (Recommended)

The tape API provides pure Go navigation with zero CGo overhead per element:

```go
pj, _ := simdjson.Parse(data, nil)
defer pj.Close()

tape, _ := pj.GetTape()
ti := tape.Iter()

// Navigate objects
obj, _ := ti.Object()
val := obj.FindKey("status")
s, _ := val.String()

// Cursor-style iteration
arr, _ := ti.Array()
ai := arr.Iter()
for ai.Type() != simdjson.Type(-1) {
    v, _ := ai.Int()
    fmt.Println(v)
    ai.Advance()
}

// Path-based lookup
elem := ti.FindElement("Image", "Width")
w, _ := elem.Int()
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

### Parse and Pool

```go
func Parse(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error)
func GetParser() *ParsedJson
func PutParser(pj *ParsedJson)
func SupportedCPU() bool
func ActiveImplementation() string
func WithCopyStrings(copy bool) ParserOption
func UseNumber() ParserOption
```

### ParsedJson

```go
func (pj *ParsedJson) Iter() (Iter, error)
func (pj *ParsedJson) GetTape() (*Tape, error)
func (pj *ParsedJson) RootType() Type
func (pj *ParsedJson) FindString(key string) (string, error)
func (pj *ParsedJson) ForEach(fn func(i Iter) error) error
func (pj *ParsedJson) Clone(dst *ParsedJson) *ParsedJson
func (pj *ParsedJson) Reset()
func (pj *ParsedJson) Close()
func (pj *ParsedJson) RootString/RootInt64/RootUint64/RootDouble/RootBool/RootCount
```

### Iter

```go
func (i *Iter) Type() Type
func (i *Iter) String() (string, error)
func (i *Iter) StringBytes() ([]byte, error)
func (i *Iter) StringCvt() (string, error)
func (i *Iter) Int() (int64, error)
func (i *Iter) Uint() (uint64, error)
func (i *Iter) Float() (float64, error)
func (i *Iter) FloatFlags() (float64, FloatFlags, error)
func (i *Iter) Bool() (bool, error)
func (i *Iter) Object(reuse *Object) (*Object, error)
func (i *Iter) Array(reuse *Array) (*Array, error)
func (i *Iter) Interface() (interface{}, error)
func (i *Iter) Advance() Type
func (i *Iter) AdvanceIter(dst *Iter) (Type, error)
func (i *Iter) PeekNext() Type
func (i *Iter) PeekNextTag() Tag
func (i *Iter) Root(dst *Iter) (Type, *Iter, error)
func (i *Iter) FindElement(reuse *Element, path ...string) (*Element, error)
```

### Object

```go
func (o *Object) FindKey(key string, reuse *Element) *Element
func (o *Object) FindPath(reuse *Element, path ...string) (*Element, error)
func (o *Object) ForEach(fn func(key string, i Iter) error) error
func (o *Object) Map(dst map[string]interface{}) (map[string]interface{}, error)
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error)
func (o *Object) NextElementBytes(dst *Iter) (name []byte, t Type, err error)
func (o *Object) Parse(dst *Elements) (*Elements, error)
func (o *Object) Count() (int, error)
```

### Array

```go
func (a *Array) ForEach(fn func(i Iter) error) error
func (a *Array) Interface() ([]interface{}, error)
func (a *Array) AsFloat() ([]float64, error)
func (a *Array) AsInteger() ([]int64, error)
func (a *Array) AsUint64() ([]uint64, error)
func (a *Array) AsString() ([]string, error)
func (a *Array) AsStringCvt() ([]string, error)
func (a *Array) Count() (int, error)
```

### Elements

```go
func (e Elements) Lookup(key string) *Element
```

### Tape (Pure Go, Zero CGo)

```go
func (t *Tape) Iter() TapeIter
func (t *Tape) RootType() Type
func (t *Tape) Interface() (interface{}, error)
func (t *Tape) Clone() *Tape

// TapeIter — cursor-style navigation
func (ti *TapeIter) Type/String/Int/Uint/Float/Bool/Object/Array/Interface
func (ti *TapeIter) Advance() Type
func (ti *TapeIter) PeekNext() Type
func (ti *TapeIter) AdvanceInto() Type
func (ti *TapeIter) FindElement(path ...string) *TapeIter
func (ti *TapeIter) StringCvt() (string, error)

// TapeObject
func (o *TapeObject) FindKey/FindPath/ForEach/Map/Count/Iter

// TapeArray
func (a *TapeArray) ForEach/AsFloat/AsInteger/AsString/Count/Interface/FirstType/Iter
```

## Building and Testing

```bash
# Quick check (lint + test)
make

# Full CI-equivalent (lint + build + race tests + benchmarks)
make release

# Individual targets
make build     # compile only
make test      # tests without race
make race      # tests with race detector
make lint      # golangci-lint
make bench     # benchmarks
make clean     # clear test cache
```

## Architecture

go-simdjson uses a two-phase approach:

1. **Parse (CGo)**: A single CGo call invokes C++ simdjson which SIMD-parses the JSON into a tape (flat `[]uint64` array) and string buffer, copied into Go memory. This is the only CGo call.
2. **Navigate (pure Go)**: All navigation — `Type()`, `String()`, `FindKey()`, `ForEach()`, `Interface()` — is pure Go pointer arithmetic on the tape. Zero CGo overhead per element.

```
┌─────────────────────────────────────────────────────┐
│                    Go Application                   │
│                                                     │
│  Parse(json) ──CGo──► C++ simdjson (SIMD parse)     │
│       │                     │                       │
│       ▼                     ▼                       │
│  ┌─────────┐  memcpy  ┌──────────┐                  │
│  │  Tape   │◄─────────│ C++ tape │                  │
│  │ []uint64│          └──────────┘                  │
│  └────┬────┘                                        │
│       │ pure Go                                     │
│       ▼                                             │
│  Iter/Object/Array/TapeIter (zero CGo)              │
└─────────────────────────────────────────────────────┘
```

## Benchmarks

Measured on Intel Xeon Platinum 8488C (x86_64, AVX2).

### Parse: go-simdjson vs encoding/json

| File | go-simdjson | encoding/json | Speedup |
|------|------------|---------------|---------|
| twitter.json (632KB) | 378µs, 1671 MB/s, 2 allocs | 5.2ms, 121 MB/s, 32K allocs | **14x** |
| canada.json (2.3MB) | 2.7ms, 849 MB/s, 2 allocs | 26.3ms, 86 MB/s, 393K allocs | **10x** |
| citm_catalog.json (1.7MB) | 765µs, 2259 MB/s, 2 allocs | 11.9ms, 145 MB/s, 96K allocs | **16x** |
| github_events.json (65KB) | 35µs, 1846 MB/s, 2 allocs | 460µs, 142 MB/s, 3K allocs | **13x** |
| mesh.json (724KB) | 855µs, 846 MB/s, 2 allocs | 10.8ms, 67 MB/s, 150K allocs | **13x** |
| numbers.json (150KB) | 161µs, 932 MB/s, 1 alloc | 1.5ms, 102 MB/s, 20K allocs | **9x** |

### Interface(): go-simdjson vs encoding/json

Full materialization to `map[string]interface{}` / `[]interface{}`:

| File | go-simdjson | encoding/json | Speedup |
|------|------------|---------------|---------|
| twitter.json | 1.35ms, 468 MB/s, 28K allocs | 5.2ms, 121 MB/s, 32K allocs | **3.9x** |
| canada.json | 4.2ms, 530 MB/s, 223K allocs | 26.3ms, 86 MB/s, 393K allocs | **6.2x** |
| citm_catalog.json | 3.1ms, 565 MB/s, 76K allocs | 11.9ms, 145 MB/s, 96K allocs | **3.9x** |
| mesh.json | 1.4ms, 515 MB/s, 74K allocs | 10.8ms, 67 MB/s, 150K allocs | **7.7x** |
| numbers.json | 156µs, 960 MB/s, 10K allocs | 1.5ms, 102 MB/s, 20K allocs | **9.4x** |

### WithCopyStrings(false)

Zero-copy string access eliminates string allocations (strings point into Go-owned tape memory):

| File | Copy (default) | NoCopy | Speedup | Alloc reduction |
|------|---------------|--------|---------|-----------------|
| twitter.json | 1.35ms, 28K allocs | 881µs, 10K allocs | **1.5x** | 64% |
| citm_catalog.json | 3.1ms, 76K allocs | 2.6ms, 50K allocs | **1.2x** | 35% |
| github_events.json | 121µs, 3.2K allocs | 71µs, 1.3K allocs | **1.7x** | 60% |
| apache_builds.json | 359µs, 9.7K allocs | 228µs, 4.4K allocs | **1.6x** | 55% |
| instruments.json | 482µs, 9.6K allocs | 361µs, 3.1K allocs | **1.3x** | 68% |
| random.json | 2.3ms, 59K allocs | 1.7ms, 26K allocs | **1.4x** | 56% |
| update-center.json | 2.1ms, 48K allocs | 1.3ms, 21K allocs | **1.6x** | 56% |

NoCopy has no effect on numeric-heavy files (canada, mesh, numbers) since they have few strings.

### Targeted Access Benchmarks

These show the cost of individual API calls (twitter.json, 632KB, pre-parsed):

| Operation | Time | Allocs | Bytes |
|-----------|------|--------|-------|
| `Elements.Lookup` | 18ns | 0 | 0 |
| `TapeIter.Advance` (tape cursor) | 42ns | 1 | 24 |
| `NextElementBytes` (key as `[]byte`) | 56ns | 2 | 48 |
| `NextElement` (key as `string`) | 62ns | 2 | 48 |
| `Object.ForEach` | 92ns | 4 | 72 |
| `Object.FindKey` | 114ns | 5 | 120 |
| `Object.FindPath` (2 levels) | 399ns | 17 | 320 |
| `Array.ForEach` (243 elements) | 1.1µs | 11 | 224 |
| `AsFloat` (numbers.json) | 68µs | 3 | 82KB |
| `AsInteger` (10K ints) | 67µs | 3 | 82KB |
| `Clone` (full document) | 172µs | 2 | 713KB |

Use `reuse` parameters to eliminate `Object`/`Array`/`Element` heap allocations in hot loops:

```go
var obj *simdjson.Object
for {
    obj, _ = iter.Object(obj) // reuses obj, zero alloc
    // ...
}
```

### Allocation Profile

| Operation | Allocations | Notes |
|-----------|------------|-------|
| Parse | 2 | tape `[]uint64` + string buffer `[]byte` |
| Interface() | O(elements) | Unavoidable: `interface{}` boxing, map/slice creation |
| Interface() NoCopy | ~64% fewer | Eliminates string copies |
| Targeted access | 0–5 per call | Use `reuse` params to minimize |
| Elements.Lookup | 0 | Zero-alloc after initial `Object.Parse` |
| AsFloat/AsInteger | 3 | Single slice allocation for result |

## Roadmap

- [ ] `ParseND` — newline-delimited JSON (ndjson) support
- [ ] `MarshalJSON` — re-serialize parsed elements
- [ ] Mutation APIs (`Set*`, `Delete*`)
- [ ] Binary serialization (`Serialize`/`Deserialize`)

## License

Apache 2.0 — same as simdjson.

simdjson is created by Daniel Lemire and Geoff Langdale. See [simdjson LICENSE](https://github.com/simdjson/simdjson/blob/master/LICENSE).
