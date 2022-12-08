.PHONY: all build fmt test lint vet

all: build fmt lint test

build:
	go build ./cmd/ndog.go

fmt:
	go fmt ./...

lint:
	@test -z $(shell gofmt -l . | tee /dev/stderr) || { echo "files above are not go fmt"; exit 1; }
	# golangci-lint run

vet:
	# This is also run by golangci-lint (make lint)
	go vet ./...

# test:
# 	go test ./...
