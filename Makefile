APP=devkit
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")

.PHONY: build build-macos package test clean run

build:
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$(APP)-darwin-arm64 ./cmd/devkit
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$(APP)-darwin-amd64 ./cmd/devkit
	lipo -create -output dist/$(APP) dist/$(APP)-darwin-arm64 dist/$(APP)-darwin-amd64

build-simple:
	mkdir -p dist
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$(APP) ./cmd/devkit

package: build
	shasum -a 256 dist/$(APP) > dist/$(APP).sha256

test:
	go test ./...

clean:
	rm -rf dist/

run:
	go run ./cmd/devkit start

run-port:
	go run ./cmd/devkit start --port 9109

install-deps:
	go mod tidy

# Development helpers
dev: clean build-simple
	./dist/$(APP) start --port 9109

format-json:
	echo '{"name":"John","age":30}' | go run ./cmd/devkit format json

format-xml:
	echo '<root><item>value</item></root>' | go run ./cmd/devkit format xml