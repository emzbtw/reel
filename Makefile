.PHONY: build test

build:
	go build -o ./reel ./cmd/reel

test:
	go test ./...
