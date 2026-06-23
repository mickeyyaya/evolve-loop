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

.PHONY: build test test-component test-integration test-e2e test-acs-durable test-all \
        test-latency cover apicover lint tidy clean help dist dist-clean \
        dist-darwin-amd64 dist-darwin-arm64 dist-linux-amd64 dist-linux-arm64

# FAST_PKGS is every non-acs package. The default (no build tags) compiles
# only the fast cost-axis layer: tests in files tagged `//go:build integration`
# or `//go:build e2e` are excluded, so `make test` stays sub-10s. See
# docs/testing.md for the two-axis (cost × granularity) model.
FAST_PKGS = $$($(GO) list ./... | grep -v '/acs/')

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

test: ## FAST layer: co-located unit/functional/component tests, race on (excludes acs/, integration, e2e)
	$(GO) test -race -count=1 -cover $(FAST_PKGS)

test-component: ## Component tier: several internal packages wired against temp FS (no subprocess)
	$(GO) test -race -count=1 ./test/component/...

test-integration: ## Integration tier: real FS/git/tmux subprocess tests (//go:build integration)
	$(GO) test -race -count=1 -tags integration ./internal/... ./cmd/... ./test/integration/...

test-e2e: ## E2E tier: full-cycle subprocess paths (//go:build e2e + evolve_test_phases); live sub-tier self-skips without its opt-in
	$(GO) test -count=1 -tags "e2e evolve_test_phases" ./cmd/... ./test/e2e/...

test-acs-durable: ## Durable ACS regression tier (//go:build acs); artifact-free predicates, mirrors the ci.yml acs-durable gate
	$(GO) test -count=1 -tags acs ./acs/regression/...

test-all: ## All tiers in one race run (local superset of CI; CI runs e2e WITHOUT -race in a separate step)
	$(GO) test -race -count=1 -tags "integration e2e" $(FAST_PKGS)

test-latency: ## Render the fast-suite latency report to test/latency-report.md
	@$(GO) test -json $(FAST_PKGS) | $(GO) run ./cmd/testlatency -title "Fast suite latency" > test/latency-report.md
	@echo "Latency report: test/latency-report.md"

cover: ## Per-package coverage report (incl. integration tier); fails if any internal/* package <85%
	@$(GO) test -race -count=1 -tags integration -coverprofile=coverage.txt $(FAST_PKGS)
	@$(GO) tool cover -func=coverage.txt | tee coverage.func.txt
	@$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage HTML: coverage.html"

# APICOVER_PKGS scopes the public-API-coverage measurement. Default is all
# internal packages; override (e.g. APICOVER_PKGS=./internal/paths/...) to focus.
APICOVER_PKGS ?= ./internal/...

apicover: ## Public-API coverage report (warning-only; APICOVER_PKGS=... to scope, -enforce in Phase 5)
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/apicover ./cmd/apicover
	@$(GO) test -count=1 -coverprofile=coverage.txt $(APICOVER_PKGS) >/dev/null
	@$(GO) tool cover -func=coverage.txt > coverage.func.txt
	@$(BIN_DIR)/apicover -cover coverage.func.txt $$($(GO) list -f '{{.Dir}}' $(APICOVER_PKGS))

lint: ## go vet + format diff (read-only)
	$(GO) vet $(PKG)
	@diff_out=$$(gofmt -d -s .) ; \
	if [ -n "$$diff_out" ]; then echo "$$diff_out"; echo "FAIL: gofmt diff above" >&2; exit 1; fi

tidy: ## Normalize go.mod / go.sum
	$(GO) mod tidy
	$(GO) mod verify

clean: dist-clean ## Remove build outputs (incl. cross-compile artifacts)
	rm -rf $(BIN_DIR) coverage.txt coverage.html coverage.func.txt
