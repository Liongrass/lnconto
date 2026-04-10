BINARY  := lnconto
MODULE  := github.com/lnconto/lnconto
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X main.version=$(VERSION) -s -w

GOFLAGS ?=

.PHONY: all build install clean test vet fmt check

all: build

## build: compile the binary into the current directory
build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

## install: compile and install to $GOPATH/bin (or $GOBIN)
install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

## clean: remove the compiled binary
clean:
	rm -f $(BINARY)

## test: run all tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go source files
fmt:
	gofmt -l -w .

## check: vet + fmt check (no writes) — useful in CI
check:
	go vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted files:"; echo "$$unformatted"; exit 1; \
	fi

## help: print this help
help:
	@grep -E '^## ' Makefile | sed 's/^## /  /'
