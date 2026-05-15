BIN := stordiag
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOOS ?= linux
GOARCH ?= amd64
LDFLAGS := "-s -w -X github.com/chirs/stordiag/cmd.version=$(VERSION)"

.PHONY: all build clean install fmt lint

all: build

build:
	go build -ldflags=$(LDFLAGS) -o $(BIN) .

install:
	go install .

fmt:
	go fmt ./...

lint:
	go vet ./...

clean:
	rm -f $(BIN)

cross: $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-darwin-amd64 $(BIN)-darwin-arm64

$(BIN)-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags=$(LDFLAGS) -o $@ .

$(BIN)-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags=$(LDFLAGS) -o $@ .

$(BIN)-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags=$(LDFLAGS) -o $@ .

$(BIN)-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags=$(LDFLAGS) -o $@ .
