NAME := ndog

VERSION := $(if $(VERSION),$(VERSION),$(shell git describe --tags --match 'v*' HEAD))
LDFLAGS := -X github.com/isobit/ndog/internal/version.Version=$(VERSION)

.PHONY: all build fmt test lint

all: build fmt lint test

build:
	go build -ldflags "$(LDFLAGS)" .

fmt:
	go fmt ./...

lint:
	@test -z $(shell gofmt -l . | tee /dev/stderr) || { echo "files above are not go fmt"; exit 1; }
	go vet ./...

test:
	go test ./...

clean:
	rm $(NAME)
	rm -rf _dist

DIST_OS_ARCH := \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64

DISTS := $(DIST_OS_ARCH:%=_dist/$(NAME)-%)

.PHONY: dist $(DISTS)
dist: $(DISTS)

$(DISTS): _dist/$(NAME)-%:
	mkdir -p _dist
	CGO_ENABLED=0 GOOS=$(word 1,$(subst -, ,$*)) GOARCH=$(word 2,$(subst -, ,$*)) \
		go build -ldflags "$(LDFLAGS)" -o $@ .
