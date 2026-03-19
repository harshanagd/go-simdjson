.PHONY: all build test race lint bench release clean

all: lint test

release: lint build race bench

build:
	go build ./...

test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

bench:
	go test -bench=. -benchmem -benchtime=1s -run='^$$' ./...

clean:
	go clean -testcache
