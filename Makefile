BIN := stordiag
GOOS ?= linux
GOARCH ?= amd64

.PHONY: all build clean install fmt lint

all: build

build:
	go build -ldflags="-s -w" -o $(BIN) .

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
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $@ .

$(BIN)-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $@ .

$(BIN)-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $@ .

$(BIN)-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $@ .
