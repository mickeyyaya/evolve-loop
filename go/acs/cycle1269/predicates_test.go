//go:build acs

// Package cycle1269 materialises the cycle-1269 acceptance criteria for the one
// fleet-scoped task triage committed to this lane:
//
//   - contextfill-telemetry-package → a new stdlib+cyclestate leaf package
//     go/internal/contextfill that DERIVES a context-window fill ratio from the
//     TokenUsage counts phasetiming/cyclestate already persist, plus a stubbed
//     model-tier context-window-size map.
//
// The two sibling proposals (wire-context-fill-stage, context-fill-hint-prompt-
// injection) are `## deferred` in triage-report.md, so this package authors ZERO
// predicates for them (R9.3: predicates bind only to triage-committed work).
//
// Predicate strategy — every load-bearing assertion CALLS the system under test
// and asserts on its return value (the cycle-85 degenerate-predicate ban). The
// package does not exist at RED, so this file does not compile: an ACS package
// that fails to compile is a HARD suite error, never a silent PASS, which is the
// correct RED signal for a greenfield package.
//
//   - 001 pins the occupancy SEMANTICS: all four TokenUsage fields count toward
//     the window, so a builder that sums only Input+Output fails.
//   - 002 is the negative predicate: a non-positive window must return
//     ErrInvalidWindow, never panic and never a silent NaN/Inf/0.
//   - 003 is the table-driven edge sweep: zero tokens, sub-threshold,
//     at-threshold, and over-window (ratio > 1.0, deliberately unclamped).
//   - 004 pins the hot-classification boundary against the exported threshold.
//   - 005 exercises the model-tier window map over modelcatalog's canonical
//     tier vocabulary, and pins unknown -> 0 (no invented default).
//   - 006 is the ADR-0069 new-package graduation check (config-check waiver).
//   - 007 runs the package's OWN unit suite as a subprocess — proof the builder
//     shipped the table-driven test file and that the package builds standalone.
//
// Import shape was compiler-probed at RED (per the reachability-probe
// obligation): a throwaway go/internal/probecf importing internal/cyclestate
// built clean (`go build ./internal/probecf` rc=0), so pinning
// `contextfill.FillRatio` over a `cyclestate.TokenUsage` argument does not
// commit the tree to an unbuildable import cycle. acs -> internal imports are
// precedented (acs/redteam imports internal/redteamcheck).
package cycle1269

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ratioEpsilon is the float tolerance for ratio comparisons. The inputs below
// are chosen so every expected value is exactly representable; the epsilon
// guards formatting/rounding drift only, never a wrong formula.
const ratioEpsilon = 1e-9

// TestC1269_001_FillRatioCountsEveryTokenKind pins the occupancy semantics:
// what fills a context window is the whole terminal token record — uncached
// input, cache reads, cache writes AND generated output — not just Input.
//
// The fixture is deliberately asymmetric (1000/500/2000/500 = 4000 of an 8000
// window = 0.5) so a builder who sums only Input+Output gets 0.1875 and a
// builder who ignores CacheWrite gets 0.4375: both fail loudly here.
func TestC1269_001_FillRatioCountsEveryTokenKind(t *testing.T) {
	tokens := cyclestate.TokenUsage{Input: 1000, Output: 500, CacheRead: 2000, CacheWrite: 500}

	got, err := contextfill.FillRatio(tokens, 8000)
	if err != nil {
		t.Fatalf("FillRatio(%+v, 8000) returned error %v, want nil", tokens, err)
	}
	if math.Abs(got-0.5) > ratioEpsilon {
		t.Errorf("FillRatio(%+v, 8000) = %v, want 0.5 — every TokenUsage field (input+output+cache_read+cache_write) must count toward window occupancy", tokens, got)
	}
}

// TestC1269_002_FillRatioRejectsNonPositiveWindow is the negative predicate and
// the strongest anti-no-op signal in this package: a window size of 0 or a
// negative window must be REJECTED with contextfill.ErrInvalidWindow. A silent
// divide-by-zero (+Inf / NaN) or a panic both fail.
func TestC1269_002_FillRatioRejectsNonPositiveWindow(t *testing.T) {
	tokens := cyclestate.TokenUsage{Input: 100, Output: 100}

	for _, window := range []int{0, -1, -8000} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("FillRatio(tokens, %d) PANICKED (%v); an invalid window must return ErrInvalidWindow, never panic", window, r)
				}
			}()

			got, err := contextfill.FillRatio(tokens, window)
			if err == nil {
				t.Errorf("FillRatio(tokens, %d) = %v, nil — want a non-nil error for a non-positive window", window, got)
				return
			}
			if !errors.Is(err, contextfill.ErrInvalidWindow) {
				t.Errorf("FillRatio(tokens, %d) error = %v, want errors.Is(..., contextfill.ErrInvalidWindow)", window, err)
			}
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("FillRatio(tokens, %d) returned %v alongside its error; the ratio must be a finite zero value, not a divide-by-zero artifact", window, got)
			}
		}()
	}
}

// TestC1269_003_FillRatioEdgeCases is the table-driven sweep the acceptance
// criteria name: zero tokens, sub-threshold, at-threshold, and over-window.
//
// The over-window row is load-bearing: the ratio must NOT be clamped to 1.0.
// Telemetry that silently saturates at "full" cannot distinguish a phase that
// just fit from one that overran its window by 50%, which is the whole
// diagnostic value of this package.
func TestC1269_003_FillRatioEdgeCases(t *testing.T) {
	const window = 1000

	cases := []struct {
		name   string
		tokens cyclestate.TokenUsage
		want   float64
	}{
		{"zero tokens", cyclestate.TokenUsage{}, 0.0},
		{"sub threshold", cyclestate.TokenUsage{Input: 250}, 0.25},
		{"at hot threshold", cyclestate.TokenUsage{Input: 500, CacheRead: 350}, contextfill.HotThreshold},
		{"over window unclamped", cyclestate.TokenUsage{Input: 1000, Output: 500}, 1.5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := contextfill.FillRatio(tc.tokens, window)
			if err != nil {
				t.Fatalf("FillRatio(%+v, %d) returned error %v, want nil", tc.tokens, window, err)
			}
			if math.Abs(got-tc.want) > ratioEpsilon {
				t.Errorf("FillRatio(%+v, %d) = %v, want %v", tc.tokens, window, got, tc.want)
			}
		})
	}
}

// TestC1269_004_IsHotBoundary pins the classification boundary against the
// exported HotThreshold, so the constant and the predicate can never drift: a
// ratio exactly AT the threshold is hot (>=, not >), and anything measurably
// below it is not. It also pins the threshold into the (0,1] range — a
// threshold of 0 would classify every phase hot and a threshold above 1 could
// never fire, and either would make the telemetry useless without failing any
// boundary-only assertion.
func TestC1269_004_IsHotBoundary(t *testing.T) {
	if contextfill.HotThreshold <= 0 || contextfill.HotThreshold > 1 {
		t.Fatalf("HotThreshold = %v, want a usable fraction in (0, 1]", contextfill.HotThreshold)
	}

	if !contextfill.IsHot(contextfill.HotThreshold) {
		t.Errorf("IsHot(HotThreshold=%v) = false, want true — the boundary is inclusive (>=)", contextfill.HotThreshold)
	}
	if !contextfill.IsHot(1.5) {
		t.Errorf("IsHot(1.5) = false, want true — an over-window phase is hot")
	}
	if contextfill.IsHot(contextfill.HotThreshold - 0.01) {
		t.Errorf("IsHot(%v) = true, want false — below the threshold is not hot", contextfill.HotThreshold-0.01)
	}
	if contextfill.IsHot(0) {
		t.Errorf("IsHot(0) = true, want false — an empty phase is never hot")
	}
}

// TestC1269_005_WindowSizeForTierCoversCanonicalTiers drives the stubbed
// model-tier window map over the REAL canonical tier vocabulary
// (modelcatalog.CanonicalTiers), so the stub cannot silently cover a
// hand-copied subset that drifts from the catalog.
//
// Unknown/empty tiers must return 0 — "unknown", not an invented default. That
// pairs with predicate 002: a 0 window flows into FillRatio as ErrInvalidWindow
// rather than a fabricated fill number.
func TestC1269_005_WindowSizeForTierCoversCanonicalTiers(t *testing.T) {
	for _, tier := range modelcatalog.CanonicalTiers {
		got := contextfill.WindowSizeForTier(tier)
		if got <= 0 {
			t.Errorf("WindowSizeForTier(%q) = %d, want a positive window size — every canonical tier needs a stub entry", tier, got)
			continue
		}
		// The map is only useful if it feeds FillRatio without erroring.
		if _, err := contextfill.FillRatio(cyclestate.TokenUsage{Input: 1}, got); err != nil {
			t.Errorf("FillRatio(tokens, WindowSizeForTier(%q)=%d) returned error %v, want nil", tier, got, err)
		}
	}

	for _, unknown := range []string{"", "nonexistent-tier", "opus"} {
		if got := contextfill.WindowSizeForTier(unknown); got != 0 {
			t.Errorf("WindowSizeForTier(%q) = %d, want 0 — an unknown tier must report unknown, never an invented default", unknown, got)
		}
	}
}

// TestC1269_006_NewPackageGraduatesIntoAPICover is the ADR-0069 new-package
// graduation obligation: a new go/internal/<pkg> must land BOTH halves in the
// same diff — the enrollment line in go/.apicover-enforce AND an
// apicover_named_test.go that names every export. Enrolled-but-unnamed fails
// the repo-wide gate; unenrolled aborts the build phase.
//
// acs-predicate: config-check — the enrollment half is inherently a
// config-presence assertion (a line in a gate manifest); the named-test half is
// EXERCISED for real by predicate 007, which runs the package suite.
func TestC1269_006_NewPackageGraduatesIntoAPICover(t *testing.T) {
	root := acsassert.RepoRoot(t)

	enroll := filepath.Join(root, "go", ".apicover-enforce")
	data, err := os.ReadFile(enroll)
	if err != nil {
		t.Fatalf("read %s: %v", enroll, err)
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "./internal/contextfill" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("go/.apicover-enforce does not enroll ./internal/contextfill; a new internal package must graduate in the SAME diff (ADR-0069 repo-wide gate)")
	}

	named := filepath.Join(root, "go", "internal", "contextfill", "apicover_named_test.go")
	if !acsassert.FileExists(t, named) {
		t.Errorf("missing %s — every exported symbol (FillRatio, IsHot, WindowSizeForTier, HotThreshold, ErrInvalidWindow) must be NAMED by identifier in a real assertion", named)
	}
}

// TestC1269_007_PackageUnitSuiteIsGreen runs the new package's own test suite as
// a subprocess — ONE named package, never a ./... sweep (flaky-predicate-shape
// rule), with cmd.Dir set explicitly so it resolves against the worktree rather
// than whatever cwd the host lane happens to hold.
//
// This is the proof that the builder shipped the table-driven contextfill_test.go
// the acceptance criteria call for AND that the package compiles standalone,
// which no assertion in this file can establish on its own.
func TestC1269_007_PackageUnitSuiteIsGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)

	src := filepath.Join(root, "go", "internal", "contextfill", "contextfill_test.go")
	if !acsassert.FileExists(t, src) {
		t.Errorf("missing %s — the acceptance criteria require a table-driven unit test beside the implementation", src)
	}

	cmd := exec.Command("go", "test", "-count=1", "./internal/contextfill")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./internal/contextfill failed: %v\n%s", err, out)
	}
}
