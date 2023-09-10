.PHONY: all build fmt test lint

VERSION := $(if $(VERSION),$(VERSION),$(shell git describe --tags --match 'v*' HEAD))
LDFLAGS := -X github.com/isobit/ndog.Version=$(VERSION)

all: build fmt lint test

build:
	go build -ldflags "$(LDFLAGS)" -o ndog ./cmd/ndog.go

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
	CGO_ENABLED=0 GOOS=$(word 1,$(subst -, ,$*)) GOARCH=$(word 2,$(subst -, ,$*)) \
		go build -ldflags "$(LDFLAGS)" -o $@ ./cmd/ndog.go
