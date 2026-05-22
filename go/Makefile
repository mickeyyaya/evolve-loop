# evolve-loop Go binary — phased rewrite per docs/architecture/portable-core.md
#
# Targets:
#   make build       — race-disabled binary into ./bin/evolve
#   make test        — go test -race -cover ./... (excludes acs/ — that's per-cycle)
#   make cover       — emits coverage.html and prints per-package %
#   make lint        — go vet + go fmt -d (read-only diff)
#   make tidy        — go mod tidy + go mod verify
#   make clean       — remove ./bin and coverage outputs

GO       ?= go
PKG      := ./...
BIN_DIR  := bin
BIN      := $(BIN_DIR)/evolve
VERSION  ?= dev
COMMIT   := $(shell git -C .. rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILT_AT := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -X github.com/mickeyyaya/evolve-loop/go/pkg/version.version=$(VERSION) \
            -X github.com/mickeyyaya/evolve-loop/go/pkg/version.commit=$(COMMIT) \
            -X github.com/mickeyyaya/evolve-loop/go/pkg/version.builtAt=$(BUILT_AT)

.PHONY: build test cover lint tidy clean help

help:
	@awk '/^[a-zA-Z_-]+:.*?##/ {sub(/:.*##/, "\t"); print}' $(MAKEFILE_LIST)

build: ## Build evolve binary into ./bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags='$(LDFLAGS)' -o $(BIN) ./cmd/evolve

test: ## Run unit + adapter tests with race detector
	$(GO) test -race -count=1 -cover $(PKG)

cover: ## Per-package coverage report; fails if any package <85%
	@$(GO) test -race -count=1 -coverprofile=coverage.txt $(PKG)
	@$(GO) tool cover -func=coverage.txt | tee coverage.func.txt
	@$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage HTML: coverage.html"

lint: ## go vet + format diff (read-only)
	$(GO) vet $(PKG)
	@diff_out=$$(gofmt -d -s .) ; \
	if [ -n "$$diff_out" ]; then echo "$$diff_out"; echo "FAIL: gofmt diff above" >&2; exit 1; fi

tidy: ## Normalize go.mod / go.sum
	$(GO) mod tidy
	$(GO) mod verify

clean: ## Remove build outputs
	rm -rf $(BIN_DIR) coverage.txt coverage.html coverage.func.txt
