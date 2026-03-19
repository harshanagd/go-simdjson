package simdjson

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// loadTestFile decompresses a .zst test fixture and returns its contents.
func loadTestFile(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name+".json.zst")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("testdata/%s.json.zst not found: %v", name, err)
	}
	out, err := exec.Command("zstd", "-d", path, "--stdout").Output()
	if err != nil {
		t.Fatalf("decompress %s: %v", name, err)
	}
	return out
}

var benchmarkFiles = []string{
	"twitter",
	"canada",
	"citm_catalog",
	"github_events",
	"apache_builds",
	"instruments",
	"mesh",
	"numbers",
	"random",
	"update-center",
}

func TestParseTestFiles(t *testing.T) {
	for _, name := range benchmarkFiles {
		t.Run(name, func(t *testing.T) {
			data := loadTestFile(t, name)
			pj, err := Parse(data, nil)
			if err != nil {
				t.Fatalf("Parse(%s) failed: %v", name, err)
			}
			defer pj.Close()
			typ := pj.RootType()
			if typ != TypeObject && typ != TypeArray {
				t.Fatalf("%s: expected object or array root, got %v", name, typ)
			}
		})
	}
}

func BenchmarkParseFiles(b *testing.B) {
	for _, name := range benchmarkFiles {
		path := filepath.Join("testdata", name+".json.zst")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		data, err := exec.Command("zstd", "-d", path, "--stdout").Output()
		if err != nil {
			b.Fatalf("decompress %s: %v", name, err)
		}

		pj := GetParser()
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pj, _ = Parse(data, pj)
			}
		})
		PutParser(pj)
	}
}

// BenchmarkParseFilesParallel benchmarks parallel parsing
// (adapted from minio/simdjson-go benchmarkFromFile nocopy-par).
func BenchmarkParseFilesParallel(b *testing.B) {
	for _, name := range benchmarkFiles {
		data := loadTestFileB(b, name)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				pj := GetParser()
				defer PutParser(pj)
				for pb.Next() {
					pj, _ = Parse(data, pj)
				}
			})
		})
	}
}

// BenchmarkStdlibUnmarshal benchmarks encoding/json for comparison
// (adapted from minio/simdjson-go BenchmarkGoMarshalJSON).
func BenchmarkStdlibUnmarshal(b *testing.B) {
	for _, name := range benchmarkFiles {
		data := loadTestFileB(b, name)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var v interface{}
				if err := json.Unmarshal(data, &v); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// loadTestFileB is the *testing.B variant of loadTestFile.
func loadTestFileB(b *testing.B, name string) []byte {
	b.Helper()
	path := filepath.Join("testdata", name+".json.zst")
	if _, err := os.Stat(path); err != nil {
		b.Skipf("testdata/%s.json.zst not found", name)
	}
	out, err := exec.Command("zstd", "-d", path, "--stdout").Output()
	if err != nil {
		b.Fatalf("decompress %s: %v", name, err)
	}
	return out
}

// BenchmarkInterfaceNoCopy benchmarks Interface() with WithCopyStrings(false).
func BenchmarkInterfaceNoCopy(b *testing.B) {
	for _, name := range benchmarkFiles {
		data := loadTestFileB(b, name)
		b.Run(name, func(b *testing.B) {
			pj := GetParser()
			defer PutParser(pj)
			pj, _ = Parse(data, pj, WithCopyStrings(false))
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				iter, _ := pj.Iter()
				_, _ = iter.Interface()
			}
		})
	}
}

// BenchmarkFindKey benchmarks Object.FindKey on real-world files.
func BenchmarkFindKey(b *testing.B) {
	// twitter.json has "statuses" at root
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		obj, _ := iter.Object(nil)
		obj.FindKey("statuses", nil)
	}
}

// BenchmarkFindPath benchmarks nested key lookup.
func BenchmarkFindPath(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		obj, _ := iter.Object(nil)
		obj.FindPath(nil, "search_metadata", "count") //nolint:errcheck
	}
}

// BenchmarkForEachObject benchmarks Object.ForEach iteration.
func BenchmarkForEachObject(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		obj, _ := iter.Object(nil)
		_ = obj.ForEach(func(key string, val Iter) error {
			return nil
		})
	}
}

// BenchmarkForEachArray benchmarks Array.ForEach iteration.
func BenchmarkForEachArray(b *testing.B) {
	data := loadTestFileB(b, "citm_catalog")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	elem := obj.FindKey("performances", nil)
	arrIter := elem.Iter
	arr, _ := arrIter.Array(nil)
	count, _ := arr.Count()
	b.SetBytes(int64(count))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		obj, _ := iter.Object(nil)
		elem := obj.FindKey("performances", nil)
		ai := elem.Iter
		arr, _ := ai.Array(nil)
		_ = arr.ForEach(func(val Iter) error {
			return nil
		})
	}
}

// BenchmarkAsFloat benchmarks typed array extraction.
func BenchmarkAsFloat(b *testing.B) {
	// numbers.json is an array of floats
	data := loadTestFileB(b, "numbers")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		arr, _ := iter.Array(nil)
		arr.AsFloat() //nolint:errcheck
	}
}

// BenchmarkAsInteger benchmarks typed integer array extraction.
func BenchmarkAsInteger(b *testing.B) {
	// Build a large int array
	data := []byte("[")
	for i := 0; i < 10000; i++ {
		if i > 0 {
			data = append(data, ',')
		}
		data = append(data, []byte(fmt.Sprintf("%d", i))...)
	}
	data = append(data, ']')
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		arr, _ := iter.Array(nil)
		arr.AsInteger() //nolint:errcheck
	}
}

// BenchmarkNextElement benchmarks stateful object iteration.
func BenchmarkNextElement(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		obj, _ := iter.Object(nil)
		var dst Iter
		for {
			name, t, _ := obj.NextElement(&dst)
			if name == "" && t == TypeNull {
				break
			}
		}
	}
}

// BenchmarkNextElementBytes benchmarks zero-alloc object iteration.
func BenchmarkNextElementBytes(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, _ := pj.Iter()
		obj, _ := iter.Object(nil)
		var dst Iter
		for {
			name, t, _ := obj.NextElementBytes(&dst)
			if name == nil && t == TypeNull {
				break
			}
		}
	}
}

// BenchmarkTapeAdvance benchmarks pure Go tape cursor navigation.
func BenchmarkTapeAdvance(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	tape, _ := pj.GetTape()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ti := tape.Iter()
		obj, _ := ti.Object()
		oi := obj.Iter()
		for oi.Advance() != Type(-1) {
		}
	}
}

// BenchmarkElementsLookup benchmarks indexed key lookup vs FindKey.
func BenchmarkElementsLookup(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	iter, _ := pj.Iter()
	obj, _ := iter.Object(nil)
	elems, _ := obj.Parse(nil)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		elems.Lookup("statuses")
		elems.Lookup("search_metadata")
	}
}

// BenchmarkClone benchmarks deep copy of parsed document.
func BenchmarkClone(b *testing.B) {
	data := loadTestFileB(b, "twitter")
	pj := GetParser()
	defer PutParser(pj)
	pj, _ = Parse(data, pj)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pj.Clone(nil)
	}
}
