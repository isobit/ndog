.PHONY: all build fmt test lint

all: build fmt lint test

build:
	go build ./cmd/ndog.go

fmt:
	go fmt ./...

lint:
	@test -z $(shell gofmt -l . | tee /dev/stderr) || { echo "files above are not go fmt"; exit 1; }
	go vet ./...

test:
	# TODO no tests yet
	# go test ./...
