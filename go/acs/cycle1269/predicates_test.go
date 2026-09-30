//go:build acs

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

const ratioEpsilon = 1e-9

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

func TestC1269_005_WindowSizeForTierCoversCanonicalTiers(t *testing.T) {
	for _, tier := range modelcatalog.CanonicalTiers {
		got := contextfill.WindowSizeForTier(tier)
		if got <= 0 {
			t.Errorf("WindowSizeForTier(%q) = %d, want a positive window size — every canonical tier needs a stub entry", tier, got)
			continue
		}
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

// acs-predicate: config-check — the enrollment half is inherently a
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
