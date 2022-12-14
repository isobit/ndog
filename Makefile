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

clean:
	rm ndog
	rm -rf _dist

DIST_OS_ARCH := \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64

DISTS := $(DIST_OS_ARCH:%=_dist/ndog-%)

.PHONY: dist $(DISTS)
dist: $(DISTS)

$(DISTS): _dist/ndog-%:
	mkdir -p _dist
	GOOS=$(word 1,$(subst -, ,$*)) GOARCH=$(word 2,$(subst -, ,$*)) go build -o $@ ./cmd/ndog.go
