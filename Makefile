.PHONY: build clean test lint format vendor tidy

BINARY := 10000speedtest
BUILD_DIR := .
GO := go
GOFLAGS := -mod=vendor
CGO_ENABLED := 0

VERSION ?= $(shell git describe --tags 2>/dev/null || echo 0.1.0)
COMMIT ?= $(shell git describe --match=NeVeRmAtCh --always --abbrev=40 --dirty)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT)

build:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) .

clean:
	rm -f $(BUILD_DIR)/$(BINARY)
	rm -rf coverage/ dist/ package/

test:
	$(GO) test $(GOFLAGS) ./... -count=1 -timeout=5m

lint:
	golangci-lint run ./...
	@command -v mulint >/dev/null 2>&1 && mulint -test=false ./... || echo "mulint not installed; skipping"

format:
	gofmt -s -w .
	goimports -w .

vendor:
	$(GO) mod tidy
	$(GO) mod vendor

tidy:
	$(GO) mod tidy
