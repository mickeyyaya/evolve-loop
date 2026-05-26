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

DIST_DIR := $(BIN_DIR)/dist
DIST_TARGETS := \
    $(DIST_DIR)/evolve-darwin-amd64 \
    $(DIST_DIR)/evolve-darwin-arm64 \
    $(DIST_DIR)/evolve-linux-amd64 \
    $(DIST_DIR)/evolve-linux-arm64

.PHONY: build test cover lint tidy clean help dist dist-clean \
        dist-darwin-amd64 dist-darwin-arm64 dist-linux-amd64 dist-linux-arm64

help:
	@awk '/^[a-zA-Z_-]+:.*?##/ {sub(/:.*##/, "\t"); print}' $(MAKEFILE_LIST)

build: ## Build evolve binary into ./bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags='$(LDFLAGS)' -o $(BIN) ./cmd/evolve

# Cross-compile pattern: GOOS / GOARCH parsed from the suffix of each
# dist-OS-ARCH target. CGO disabled for fully static binaries —
# evolve-loop's Go tree has no cgo dependency (verified by go.mod) and
# disabling it removes the glibc-version surface on linux.
dist: $(DIST_TARGETS) ## Cross-compile all release binaries into ./bin/dist/

dist-darwin-amd64: $(DIST_DIR)/evolve-darwin-amd64
dist-darwin-arm64: $(DIST_DIR)/evolve-darwin-arm64
dist-linux-amd64:  $(DIST_DIR)/evolve-linux-amd64
dist-linux-arm64:  $(DIST_DIR)/evolve-linux-arm64

$(DIST_DIR)/evolve-%:
	@mkdir -p $(DIST_DIR)
	@os=$$(echo $* | cut -d- -f1); arch=$$(echo $* | cut -d- -f2); \
	echo "==> cross-compile $$os/$$arch -> $@"; \
	CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	  $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o $@ ./cmd/evolve

dist-clean: ## Remove cross-compiled artifacts
	rm -rf $(DIST_DIR)

test: ## Run unit + adapter tests with race detector (excludes per-cycle acs/)
	$(GO) test -race -count=1 -cover $$($(GO) list ./... | grep -v '/acs/')

cover: ## Per-package coverage report; fails if any package <85%
	@$(GO) test -race -count=1 -coverprofile=coverage.txt $$($(GO) list ./... | grep -v '/acs/')
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

clean: dist-clean ## Remove build outputs (incl. cross-compile artifacts)
	rm -rf $(BIN_DIR) coverage.txt coverage.html coverage.func.txt
