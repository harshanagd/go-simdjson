# go-simdjson

Cross-architecture SIMD JSON parser for Go, powered by [simdjson](https://github.com/simdjson/simdjson). CI-tested on x86_64, ARM64, ppc64le, and s390x with automatic runtime detection of the fastest SIMD backend (AVX-512, AVX2, SSE4.2, NEON, VSX, or scalar fallback).

## Why?

The existing Go simdjson port ([minio/simdjson-go](https://github.com/minio/simdjson-go)) only supports x86_64. This library wraps the official C++ simdjson via CGo, giving you SIMD-accelerated JSON parsing on **every architecture** that simdjson supports — including ARM64 (AWS Graviton, Apple Silicon).

## Requirements

- Go 1.21+
- C++17 compiler (GCC 8+, Clang 7+, or Xcode 11+)
- CGo enabled (`CGO_ENABLED=1`, the default for native builds)

### C++ runtime linking

The C++ runtime is linked dynamically by default, so a binary needs a libstdc++ at
least as new as the one it was built against — an older host refuses to load it with
`version GLIBCXX_3.4.xx not found`. Build with the `simdjson_static_cxx` tag to bind
libstdc++ into the binary instead:

```bash
go build -tags simdjson_static_cxx
```

It is opt-in for two reasons. glibc stays dynamic either way, so the result is not a
fully static binary and `GLIBC_2.xx not found` remains possible. And libstdc++ is
GPLv3 with the GCC Runtime Library Exception, which permits static linking when the
build uses a GPL-compatible toolchain — ordinary GCC builds qualify, but the default
should not answer that question on your behalf.

The tag is a no-op on macOS, where the runtime is libc++ and always dynamic. CI
builds and tests both modes on Linux and macOS and asserts the produced binary's
linkage, since `-static-libstdc++` is silently ignored by a driver that does not
implement it — a successful build does not mean the tag took effect.

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

Reusing a parser re-parses over the memory the previous document's tape pointed at,
so finish with `data1` before parsing `data2`, or `Clone()` to keep it.

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
arr.ForEach(func(i simdjson.Iter) {
    userObj, _ := i.Object(nil)
    name := userObj.FindKey("name", nil)
    fmt.Println(name.Iter.String())
})

// Or convert everything to Go native types
iter, _ = pj.Iter()
v, _ := iter.Interface() // map[string]interface{}, []interface{}, etc.
```

## Tape Navigation

Both navigation APIs are pure Go with zero CGo per element — `Iter` is a thin
delegation to `TapeIter`, so the choice between them is not about CGo overhead.
The difference is allocation:

| | ns/op | B/op | allocs |
|---|---|---|---|
| `TapeObject.FindKey` | 15 | 8 | 1 |
| `Object.FindKey(key, nil)` | 53 | 104 | 3 |
| `Object.FindKey(key, &reuse)` | 22 | 8 | 1 |

The tape layer returns values and materialises no `Element`, so it allocates less;
passing a `reuse` brings `Iter` to parity for lookups, and `Advance` is already equal
on both (`TapeIter.Advance` and `Iter.Advance` are both zero-alloc).

Use `Iter` by default — it has the larger surface, including mutation, marshalling,
`Root` and `FindElement` with reuse. Reach for the tape layer in allocation-sensitive
hot loops where you do not need those.

```go
pj, _ := simdjson.Parse(data, nil)
defer pj.Close()

tape, _ := pj.GetTape()
ti := tape.Iter()

// Navigate objects
obj, _ := ti.Object()
val, ok := obj.FindKey("status")
if !ok {
    log.Fatal("no status key")
}
s, _ := val.String()

// Cursor-style iteration
arr, _ := ti.Array()
ai := arr.Iter()
for ai.Type() != simdjson.TypeNone {
    v, _ := ai.Int()
    fmt.Println(v)
    ai.Advance()
}

// Path-based lookup
elem, ok := ti.FindElement("Image", "Width")
if !ok {
    log.Fatal("no Image.Width")
}
w, _ := elem.Int()
```

## Big Integer Support

JSON integers that exceed the 64-bit range (e.g., blockchain IDs, large financial numbers) are supported via opt-in options:

```go
// Option 1: UseBigInt — only big integers become json.Number, normal numbers stay native
pj, err := simdjson.Parse(data, nil, simdjson.UseBigInt())
iter, _ := pj.Iter()
v, _ := iter.Interface()
// v["big"] is json.Number("123456789012345678901")
// v["small"] is int64(42)

// Option 2: UseNumber — all numbers become json.Number (like encoding/json)
pj, err = simdjson.Parse(data, nil, simdjson.UseNumber())
// v["big"] is json.Number("123456789012345678901")
// v["small"] is json.Number("42")
```

Without either option, big integers return a parse error (existing behavior).

## Supported Platforms

The best SIMD implementation is selected automatically at runtime — no build flags needed.

| Architecture | SIMD Backend | CI Tested |
|-------------|-------------|-----------|
| x86_64 | AVX-512, AVX2, SSE4.2 | ✅ Native |
| ARM64 | NEON | ✅ Native |
| ppc64le | VSX / Altivec | ✅ QEMU |
| s390x | Scalar fallback | ✅ QEMU |
| riscv64 | RISC-V Vector (RVV) | Supported, not CI tested |
| LoongArch64 | LSX / LASX | Supported, not CI tested |
| Any other | Scalar fallback | Supported via fallback |

Use `simdjson.ActiveImplementation()` to check which backend was selected at runtime.

## API

### Parse and Pool

```go
func Parse(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error)
func ParseND(b []byte, reuse *ParsedJson, opts ...ParserOption) (*ParsedJson, error)
func ParseNDStream(r io.Reader, res chan<- Stream, reuse <-chan *ParsedJson)
func GetParser() *ParsedJson
func PutParser(pj *ParsedJson)
func SupportedCPU() bool
func ActiveImplementation() string
func WithCopyStrings(copy bool) ParserOption
func UseNumber() ParserOption
func UseBigInt() ParserOption
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
func (i *Iter) BigInt() (json.Number, error)
func (i *Iter) Float() (float64, error)
func (i *Iter) FloatFlags() (float64, FloatFlags, error)
func (i *Iter) Bool() (bool, error)
func (i *Iter) Object(reuse *Object) (*Object, error)
func (i *Iter) Array(reuse *Array) (*Array, error)
func (i *Iter) Interface() (interface{}, error)
func (i *Iter) Advance() Type
func (i *Iter) AdvanceInto() Tag
func (i *Iter) AdvanceIter(dst *Iter) (Type, error)
func (i *Iter) PeekNext() Type
func (i *Iter) PeekNextTag() Tag
func (i *Iter) Root(dst *Iter) (Type, *Iter, error)
func (i *Iter) FindElement(reuse *Element, path ...string) (*Element, error)
func (i *Iter) MarshalJSON() ([]byte, error)
func (i *Iter) MarshalJSONBuffer(dst []byte) ([]byte, error)

// Mutation
func (i *Iter) SetFloat(v float64) error
func (i *Iter) SetInt(v int64) error
func (i *Iter) SetUInt(v uint64) error
func (i *Iter) SetString(v string) error
func (i *Iter) SetStringBytes(v []byte) error
func (i *Iter) SetBool(v bool) error
func (i *Iter) SetNull() error
```

### Object

```go
func (o *Object) FindKey(key string, reuse *Element) *Element
func (o *Object) FindPath(reuse *Element, path ...string) (*Element, error)
func (o *Object) ForEach(fn func(key []byte, i Iter), onlyKeys map[string]struct{}) error
func (o *Object) Map(dst map[string]interface{}) (map[string]interface{}, error)
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error)
func (o *Object) NextElementBytes(dst *Iter) (name []byte, t Type, err error)
func (o *Object) Parse(dst *Elements) (*Elements, error)
func (o *Object) Count() (int, error)
func (o *Object) DeleteElems(fn func(key []byte, i Iter) bool, onlyKeys map[string]struct{}) error
```

### Array

```go
func (a *Array) ForEach(fn func(i Iter))
func (a *Array) Interface() ([]interface{}, error)
func (a *Array) AsFloat() ([]float64, error)
func (a *Array) AsInteger() ([]int64, error)
func (a *Array) AsUint64() ([]uint64, error)
func (a *Array) AsString() ([]string, error)
func (a *Array) AsStringCvt() ([]string, error)
func (a *Array) Count() (int, error)
func (a *Array) FirstType() Type
func (a *Array) DeleteElems(fn func(i Iter) bool)
func (a *Array) MarshalJSON() ([]byte, error)
func (a *Array) MarshalJSONBuffer(dst []byte) ([]byte, error)
```

### Elements

```go
func (e Elements) Lookup(key string) *Element
func (e Elements) MarshalJSON() ([]byte, error)
func (e Elements) MarshalJSONBuffer(dst []byte) ([]byte, error)
```

### Serializer

```go
func NewSerializer() *Serializer
func (s *Serializer) Serialize(dst []byte, pj ParsedJson) []byte
func (s *Serializer) Deserialize(src []byte, dst *ParsedJson) (*ParsedJson, error)
```

### Tape (Pure Go, Zero CGo)

```go
func (t *Tape) Iter() TapeIter
func (t *Tape) RootType() Type
func (t *Tape) Interface() (interface{}, error)
func (t *Tape) Clone() *Tape

// TapeIter — cursor-style navigation. Containers and lookups are returned by
// value, so navigation does not allocate; lookups report absence with ok=false.
func (ti *TapeIter) Type/String/Int/Uint/Float/Bool/BigInt/Interface
func (ti *TapeIter) Object() (TapeObject, error)
func (ti *TapeIter) Array() (TapeArray, error)
func (ti *TapeIter) Advance() Type
func (ti *TapeIter) PeekNext() Type
func (ti *TapeIter) AdvanceInto() Type
func (ti *TapeIter) FindElement(path ...string) (TapeIter, bool)
func (ti *TapeIter) StringCvt() (string, error)

// TapeObject
func (o *TapeObject) FindKey(key string) (TapeIter, bool)
func (o *TapeObject) FindPath(path ...string) (TapeIter, bool)
func (o *TapeObject) ForEach/Map/Count/Iter

// TapeArray
func (a *TapeArray) ForEach/AsFloat/AsInteger/AsUint64/AsString/AsStringCvt/Count/Interface/FirstType/Iter
```

## Building and Testing

```bash
# Quick check (lint + test)
make

# Full CI-equivalent (lint + build + both link modes + race + benchmarks + fuzzing)
make release

# Individual targets
make build     # compile only
make test      # tests without race
make race      # tests with race detector
make lint      # golangci-lint
make static    # build and test with -tags simdjson_static_cxx
make bench     # benchmarks
make fuzz      # fuzz every target; make fuzz FUZZTIME=5m for longer
make clean     # clear test cache
```

`go test` alone only replays each fuzz target's stored seed corpus. `make fuzz`
mutates, which is what reaches the malformed-tape and serialized-envelope paths:

| target | covers |
|---|---|
| `FuzzParse` | arbitrary bytes through `Parse`, then `Iter`/`Interface` |
| `FuzzCorrect` | agreement with `encoding/json` on inputs it accepts |
| `FuzzDeserialize` | `Serializer.Deserialize`, including the length-prefix envelope |
| `FuzzTapeContract` | mutated tapes still satisfying `Tape.Validate` |

## Architecture

go-simdjson uses a two-phase approach:

1. **Parse (CGo)**: A single CGo call invokes C++ simdjson which SIMD-parses the JSON into a tape (flat `[]uint64` array) and string buffer. Go wraps both with `unsafe.Slice` — no copy — and the C++ parser retains ownership. This is the only CGo call.
2. **Navigate (pure Go)**: All navigation — `Type()`, `String()`, `FindKey()`, `ForEach()`, `Interface()` — is pure Go pointer arithmetic on the tape. Zero CGo overhead per element.

```
┌─────────────────────────────────────────────────────┐
│                    Go Application                   │
│                                                     │
│  Parse(json) ──CGo──► C++ simdjson (SIMD parse)     │
│       │                     │                       │
│       ▼                     ▼                       │
│  ┌─────────┐  view    ┌──────────┐                  │
│  │  Tape   │─ ─ ─ ─ ─►│ C++ tape │ (parser-owned)   │
│  │ []uint64│  no copy └──────────┘                  │
│  └────┬────┘                                        │
│       │ pure Go                                     │
│       ▼                                             │
│  Iter/Object/Array/TapeIter (zero CGo)              │
└─────────────────────────────────────────────────────┘
```

`Tape` holds an `unsafe.Slice` over the C++ parser's buffers, plus a back-reference
to the owning `ParsedJson` so the GC keeps the parser alive while any iterator
exists. The view is valid until the next `Parse` on that `ParsedJson` or until
`Close`; `Clone()` returns an independent Go-owned copy that outlives both.

## Benchmarks

Measured on Intel Xeon Platinum 8488C (x86_64, AVX2).

### Parse: go-simdjson vs encoding/json

| File | go-simdjson | encoding/json | Speedup |
|------|------------|---------------|---------|
| twitter.json (632KB) | 378µs, 1671 MB/s, 0 allocs | 5.2ms, 121 MB/s, 32K allocs | **14x** |
| canada.json (2.3MB) | 2.7ms, 849 MB/s, 0 allocs | 26.3ms, 86 MB/s, 393K allocs | **10x** |
| citm_catalog.json (1.7MB) | 765µs, 2259 MB/s, 0 allocs | 11.9ms, 145 MB/s, 96K allocs | **16x** |
| github_events.json (65KB) | 35µs, 1846 MB/s, 0 allocs | 460µs, 142 MB/s, 3K allocs | **13x** |
| mesh.json (724KB) | 855µs, 846 MB/s, 0 allocs | 10.8ms, 67 MB/s, 150K allocs | **13x** |
| numbers.json (150KB) | 161µs, 932 MB/s, 0 allocs | 1.5ms, 102 MB/s, 20K allocs | **9x** |

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

`WithCopyStrings(false)` lets a string read alias the tape's buffer instead of
allocating — but only on a **cloned** tape, which owns Go memory. A tape straight
from `Parse` is a view into the C++ parser, where an aliased string would have no
owner the GC can see, so those reads copy whatever the flag says. The fastest
iteration path is therefore `Clone()` followed by `WithCopyStrings(false)`:

```go
pj, _ := simdjson.Parse(data, nil, simdjson.WithCopyStrings(false))
cloned := pj.Clone(nil) // Go-owned; the no-copy path applies from here
pj.Close()
```

The measurements previously in this section were taken when `Parse` copied both
buffers into Go memory, so they described the parsed-tape path this change removed.
They are pending a re-run against the clone path on the reference machine. String-heavy
documents (twitter, github_events, apache_builds, update-center) benefited most, and
numeric-heavy ones (canada, mesh, numbers) never did, having few strings.

### Targeted Access Benchmarks

These show the cost of individual API calls (twitter.json, 632KB, pre-parsed):

| Operation | Time | Allocs | Bytes |
|-----------|------|--------|-------|
| `Elements.Lookup` | 18ns | 0 | 0 |
| `TapeIter.Advance` (tape cursor) | 42ns | 0 | 0 |
| `NextElementBytes` (key as `[]byte`) | 56ns | 2 | 48 |
| `NextElement` (key as `string`) | 62ns | 2 | 48 |
| `Object.ForEach` | 92ns | 1 | 48 |
| `Object.FindKey` | 114ns | 3 | 104 |
| `Object.FindPath` (2 levels) | 399ns | 11 | 192 |
| `Array.ForEach` (243 elements) | 1.1µs | 8 | 200 |
| `AsFloat` (numbers.json) | 68µs | 2 | 82KB |
| `AsInteger` (10K ints) | 67µs | 2 | 82KB |
| `Clone` (full document) | 172µs | 2 | 713KB |

Allocation counts for `TapeIter.Advance`, `FindKey` and `FindPath` reflect the by-value
tape navigation, and those for both `ForEach` methods reflect the simdjson-go
signatures (an object key is now `[]byte`, so it is no longer copied into a string).
The timings predate both changes and are pending a re-run on the reference machine,
so every one of those rows is faster than shown.

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
| Parse | 0 | zero-copy view over the C++ parser's buffers |
| Interface() | O(elements) | Unavoidable: `interface{}` boxing, map/slice creation |
| Interface() on a clone, NoCopy | fewer | Skips string copies; clone required, see above |
| Targeted access | 0–5 per call | Use `reuse` params to minimize |
| Elements.Lookup | 0 | Zero-alloc after initial `Object.Parse` |
| AsFloat/AsInteger | 2 | The result slice, plus the `*Array` unless you pass a `reuse` |

## simdjson-go compatibility

The exported surface aims to be drop-in compatible with
[minio/simdjson-go](https://github.com/minio/simdjson-go) — same names, same
signatures, so switching the import path is enough. It is not fully there, so this
section lists where it diverges instead of claiming parity.

### Matching

`Parse`, `ParseND`, `ParseNDStream`, `SupportedCPU`, `WithCopyStrings`, `Stream`,
`ParsedJson.Clone`/`ForEach`/`Reset`, all of `Iter` — including the `Set*`
mutations, `MarshalJSON`/`MarshalJSONBuffer`, `Root`, `FindElement` and
`FloatFlags` — all of `Object`, all of `Array` except `Iter()`, `Element`,
`Elements.Lookup`, `Serializer.Serialize`/`Deserialize`, every `Tag` constant,
`TagToType`, `ErrPathNotFound`, `FloatFlag.Flags`, and the `JSONTAGOFFSET` /
`JSONTAGMASK` / `JSONVALUEMASK` tape constants.

`TypeInt`, `TypeUint` and `TypeFloat` exist as simdjson-go's names for
`TypeInt64`, `TypeUint64` and `TypeDouble`. The values are identical, so a switch
written against either set compiles and matches.

### Diverging

| | simdjson-go | here | why |
|---|---|---|---|
| `ParsedJson.Iter()` | `Iter` | `(Iter, error)` | An unparsed `ParsedJson` is an error, not a past-end cursor. Dropping the error would make that failure surface later and quieter. |
| `ParsedJson.Message` | `[]byte` | absent | The tape views the C++ parser's buffers; there is no retained Go message buffer for payloads to index into. A nil field their code reads is worse than its absence. |
| `ParsedJson.Tape`, `.Strings` | exported fields | `GetTape() (*Tape, error)` | Same ownership reason: the buffers are parser-owned and valid only until the next `Parse` or `Close`, which a bare field cannot express. |
| `Array.Iter()` | `Iter`, first value ready after `Advance` | absent | Their cursor starts *before* the first element. `Advance` here moves by skipping the value at the cursor, so no index exists from which it lands on element 0 — a naive port would silently skip the first element. `TapeArray.Iter` is the equivalent, positioned *at* the first element. |
| `Serializer.CompressMode` | present | absent | Nothing here compresses, so it could only be accepted and ignored. The serialized formats are not interchangeable anyway. |
| `Type` underlying type | `uint8`, iota 0–9 | `int`, tag bytes | Our values are the tape's own tag bytes, which is what makes `Tag.Type()` a conversion rather than a lookup. |
| `Type.String()` | `int`, `uint`, `float` | `int64`, `uint64`, `double` | Names the width actually stored. |
| `STRINGBUFBIT`, `STRINGBUFMASK` | exported | absent | They mark a payload that points into the message buffer rather than the string buffer. This tape has no such bit; strings always live in the string buffer. |

### Additions

`Iter.StringRef`, `Iter.BigInt`, `Object.Count`, `Array.Count`, `GetParser`,
`PutParser`, `Close`, `ActiveImplementation`, `FindString`, the `Root*`
accessors, `UseNumber`, `UseBigInt`, `TypeBigInt`, `TagBigInt` and the whole
`Tape`/`TapeIter`/`TapeObject`/`TapeArray` layer have no simdjson-go equivalent,
so they cannot break a program written against it.

`compat_test.go` checks behaviour against shared fixtures; only
`TestSimdjsonGoTypeAliases` checks a signature. The tables above are maintained by
hand, so a rename upstream will not fail the build here.

## License

Apache 2.0 — same as simdjson.

simdjson is created by Daniel Lemire and Geoff Langdale. See [simdjson LICENSE](https://github.com/simdjson/simdjson/blob/master/LICENSE).
