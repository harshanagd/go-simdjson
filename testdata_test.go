package simdjson

import (
	"encoding/json"
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
