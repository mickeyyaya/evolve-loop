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

# One-binary S2: single macOS fingerprint (lipo'd fat binary) + a checksums
# manifest over every dist artifact (for approval-request fingerprinting and
# install.sh --binary verification). The universal target is macOS-only (lipo);
# `make dist` is operator-run locally on macOS — no CI/release job invokes it
# (goreleaser's own universal_binaries block builds the release-time universal).
DIST_UNIVERSAL := $(DIST_DIR)/evolve-darwin-universal
DIST_CHECKSUMS := $(DIST_DIR)/checksums.txt

.PHONY: build test test-component test-integration test-e2e test-acs-durable test-all \
        test-latency cover apicover lint tidy clean help dist dist-clean \
        cover-strict apicover-enforce apicover-check \
        dist-darwin-amd64 dist-darwin-arm64 dist-linux-amd64 dist-linux-arm64 \
        dist-darwin-universal dist-checksums

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
dist: $(DIST_TARGETS) $(DIST_UNIVERSAL) $(DIST_CHECKSUMS) ## Cross-compile all release binaries + universal macOS binary + checksums.txt into ./bin/dist/

dist-darwin-amd64: $(DIST_DIR)/evolve-darwin-amd64
dist-darwin-arm64: $(DIST_DIR)/evolve-darwin-arm64
dist-linux-amd64:  $(DIST_DIR)/evolve-linux-amd64
dist-linux-arm64:  $(DIST_DIR)/evolve-linux-arm64
dist-darwin-universal: $(DIST_UNIVERSAL)
dist-checksums: $(DIST_CHECKSUMS)

$(DIST_DIR)/evolve-%:
	@mkdir -p $(DIST_DIR)
	@os=$$(echo $* | cut -d- -f1); arch=$$(echo $* | cut -d- -f2); \
	echo "==> cross-compile $$os/$$arch -> $@"; \
	CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	  $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o $@ ./cmd/evolve

# lipo-merge the two darwin arch binaries into ONE fat macOS binary — a single
# fingerprint per version. An explicit rule so it overrides the evolve-% pattern
# above (which would otherwise try GOARCH=universal). macOS-only.
$(DIST_UNIVERSAL): $(DIST_DIR)/evolve-darwin-amd64 $(DIST_DIR)/evolve-darwin-arm64
	@command -v lipo >/dev/null 2>&1 || { echo "dist-darwin-universal requires lipo (macOS)"; exit 1; }
	@echo "==> lipo darwin universal -> $@"
	@lipo -create -output $@ $(DIST_DIR)/evolve-darwin-amd64 $(DIST_DIR)/evolve-darwin-arm64

# SHA256 of every dist artifact — the fingerprint manifest for approval requests
# and install.sh --binary checksum verification. Rebuilt whenever any binary
# changes. $(notdir $^) is the basename list of the prerequisites above (single
# source of truth for "what gets checksummed"); sorted for a stable manifest.
$(DIST_CHECKSUMS): $(DIST_TARGETS) $(DIST_UNIVERSAL)
	@echo "==> checksums -> $@"
	@cd $(DIST_DIR) && shasum -a 256 $(notdir $^) | sort -k2 > checksums.txt

dist-clean: ## Remove cross-compiled artifacts
	rm -rf $(DIST_DIR)

test: ## FAST layer: co-located unit/functional/component tests, race on (excludes acs/, integration, e2e)
	$(GO) test -race -count=1 -cover $(FAST_PKGS)

test-component: ## Component tier: several internal packages wired against temp FS (no subprocess)
	$(GO) test -race -count=1 ./test/component/...

test-integration: ## Full non-ACS runtime suite with integration, race, and coverage (shared by CI and release)
	@pkgs=$$($(GO) list -tags integration ./...) || exit $$?; \
	pkgs=$$(printf '%s\n' "$$pkgs" | grep -v '/acs/') || exit $$?; \
	$(GO) test -race -count=1 -tags integration -coverprofile=coverage.txt $$pkgs

test-e2e: ## E2E tier: full-cycle subprocess paths (//go:build e2e + evolve_test_phases); live sub-tier self-skips without its opt-in
	$(GO) test -count=1 -timeout 45m -tags "e2e evolve_test_phases" ./cmd/... ./test/e2e/...

test-acs-durable: ## Durable ACS regression tier (//go:build acs); artifact-free predicates, mirrors the ci.yml acs-durable gate
	$(GO) test -count=1 -tags acs ./acs/regression/...

test-all: test-integration ## All runtime tiers, preserving no-race E2E and its test-only phase tag
	$(MAKE) -f "$(firstword $(MAKEFILE_LIST))" test-e2e
	$(MAKE) -f "$(firstword $(MAKEFILE_LIST))" test-acs-durable

test-latency: ## Render the fast-suite latency report to test/latency-report.md
	@$(GO) test -json $(FAST_PKGS) | $(GO) run ./cmd/testlatency -title "Fast suite latency" > test/latency-report.md
	@echo "Latency report: test/latency-report.md"

cover: test-integration ## Coverage report; hard package floors are enforced separately by cover-strict
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

cover-strict: ## Per-package line-coverage hard-fail over go/.cover-strict (ADR-0101 §11: `<pkg> <min%>` per line; mirrors the go.yml strict step)
	@test -f .cover-strict && test -r .cover-strict || { echo "cover-strict: cannot read regular file .cover-strict" >&2; exit 1; }
	@fail=0; while read -r pkg min || [ -n "$$pkg" ]; do \
		case "$$pkg" in ""|\#*) continue;; esac; \
		out=$$($(GO) test -count=1 -tags integration -cover $$pkg 2>&1) || { printf 'cover-strict: %s: test failed\n%s\n' "$$pkg" "$$out" >&2; fail=1; continue; }; \
		got=$$(printf '%s\n' "$$out" | sed -n 's/.*coverage: \([0-9.]*\)%.*/\1/p') || { echo "cover-strict: $$pkg: coverage parsing failed" >&2; fail=1; continue; }; \
		if [ -z "$$got" ]; then echo "cover-strict: $$pkg: no coverage figure (build or test failure)"; fail=1; continue; fi; \
		if awk -v g="$$got" -v m="$$min" 'BEGIN{if (g !~ /^[0-9]+([.][0-9]+)?$$/ || m !~ /^[0-9]+([.][0-9]+)?$$/ || g+0 > 100 || m+0 > 100) exit 2; exit !(g+0 >= m+0)}'; then echo "cover-strict: $$pkg: $$got% >= $$min% ok"; else echo "cover-strict: $$pkg: invalid or unsatisfied floor ($$got% / $$min%) FAIL"; fail=1; fi; \
	done < .cover-strict; exit $$fail

# Test-only ACS enrollments have no production API, and their artifact-dependent
# predicates belong to the cycle runner. Inspect those dirs without running them.
apicover-enforce: ## Per-package hard-fail over go/.apicover-enforce (mirrors go.yml Phase-5; the composed-gate recipe — a red here means an uncovered/false-green EXPORT, and it must fail the tree)
	@pkgs="$${APICOVER_PKGS}"; \
	if [ -z "$$pkgs" ]; then pkgs=$$(sed '/^[[:space:]]*\#/d; /^[[:space:]]*$$/d' .apicover-enforce) || exit $$?; fi; \
	if [ -z "$$pkgs" ]; then echo "apicover-enforce: no enforced packages — skipping"; exit 0; fi; \
	set --; for pkg in $$pkgs; do \
		case "$$pkg" in */acs/*|acs/*) continue;; esac; \
		set -- "$$@" "$$pkg"; \
	done; \
	if [ "$$#" -eq 0 ]; then printf 'mode: set\n' > coverage.txt; \
	else $(GO) test -count=1 -tags integration -coverprofile=coverage.txt "$$@" || exit $$?; fi
	$(MAKE) -f "$(firstword $(MAKEFILE_LIST))" apicover-check

apicover-check: ## Enforce public API coverage from the successful integration run's coverage.txt
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/apicover ./cmd/apicover
	@pkgs="$${APICOVER_PKGS}"; \
	if [ -z "$$pkgs" ]; then pkgs=$$(sed '/^[[:space:]]*\#/d; /^[[:space:]]*$$/d' .apicover-enforce) || exit $$?; fi; \
	if [ -z "$$pkgs" ]; then echo "apicover-enforce: no enforced packages — skipping"; exit 0; fi; \
	$(GO) tool cover -func=coverage.txt > coverage.func.txt || exit $$?; \
	$(GO) list -e -f '{{if .Dir}}{{.Dir}}{{else}}MISSING_PACKAGE{{end}}' $$pkgs > $(BIN_DIR)/apicover-dirs.txt || exit $$?; \
	set --; while IFS= read -r dir; do \
		test -d "$$dir" || { echo "apicover-check: missing package directory: $$dir" >&2; exit 1; }; \
		set -- "$$@" "$$dir"; \
	done < $(BIN_DIR)/apicover-dirs.txt; \
	test "$$#" -gt 0 || { echo "apicover-check: no package directories" >&2; exit 1; }; \
	$(BIN_DIR)/apicover -enforce -cover coverage.func.txt "$$@"

lint: ## go vet + format diff (read-only)
	$(GO) vet $(PKG)
	@diff_out=$$(gofmt -d -s .) ; \
	if [ -n "$$diff_out" ]; then echo "$$diff_out"; echo "FAIL: gofmt diff above" >&2; exit 1; fi

tidy: ## Normalize go.mod / go.sum
	$(GO) mod tidy
	$(GO) mod verify

clean: dist-clean ## Remove build outputs (incl. cross-compile artifacts)
	rm -rf $(BIN_DIR) coverage.txt coverage.html coverage.func.txt
