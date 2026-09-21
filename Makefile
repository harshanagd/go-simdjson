.PHONY: all build test race lint bench fuzz static release clean

all: lint test

release: lint build static race bench fuzz

build:
	go build ./...

test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

# The opt-in link mode is invisible to every other target, so it rots without this.
# CI additionally asserts the produced binary's linkage, which needs ldd/otool.
static:
	go build -tags simdjson_static_cxx ./...
	go test -count=1 -tags simdjson_static_cxx ./...

bench:
	go test -bench=. -benchmem -benchtime=1s -run='^$$' ./...

# Plain `go test` only replays each target's seed corpus; this mutates.
# Override the budget with e.g. make fuzz FUZZTIME=5m.
FUZZTIME ?= 30s
fuzz:
	@for t in FuzzParse FuzzCorrect FuzzDeserialize FuzzTapeContract; do \
		echo "fuzzing $$t for $(FUZZTIME)"; \
		go test -run='^$$' -fuzz="^$$t$$" -fuzztime=$(FUZZTIME) . || exit 1; \
	done

clean:
	go clean -testcache
