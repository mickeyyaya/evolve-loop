//go:build acs

package cycle1271

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasetimingPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	corePkg         = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	contextfillPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	internalPattern = "github.com/mickeyyaya/evolve-loop/go/internal/..."
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1271_001_EntryCarriesContextFillAndDegradesToAbsent(t *testing.T) {
	ok, out := runGoTest(t, phasetimingPkg,
		"TestEntry_ContextFillFieldsRoundTripJSON|TestEntry_LegacyLogWithoutContextFillParsesToZero|TestEntry_ContextFillOmittedWhenUnknown")
	if !ok {
		t.Errorf("phasetiming.Entry does not carry the context-fill telemetry contract (fields absent, wrong JSON keys, missing omitempty, or a legacy log no longer parses):\n%s", out)
	}
}

func TestC1271_002_ChokepointDerivesFillFromRealDispatch(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestRecordPhaseOutcome_ProjectsContextFillWhenTierResolvable|TestRecordPhaseOutcome_ColdPhaseIsNotFlaggedHot|TestRecordPhaseOutcome_UnresolvableTierLeavesContextFillZero|TestRecordPhaseOutcome_ZeroTokensRecordsZeroFillNotHot|TestPhaseOutcomeFrom_CarriesTierProvenanceAndTokens")
	if !ok {
		t.Errorf("the context-fill ratio is not derived at the production recordPhaseOutcome chokepoint (unwired, imprecise, or fabricating a ratio when no tier is resolvable):\n%s", out)
	}
}

func TestC1271_003_RollupSummarisesHotPhases(t *testing.T) {
	ok, out := runGoTest(t, phasetimingPkg,
		"TestRollup_HotPhaseCountAndNames|TestRollup_NoHotPhasesReportsEmpty|TestRollup_HotThresholdBoundaryIsInclusive|TestRollup_EmptyEntriesNoHotFields")
	if !ok {
		t.Errorf("phasetiming.Rollup does not summarise hot phases correctly (fields absent, wrong threshold boundary, or a tier-unresolved entry reported hot):\n%s", out)
	}
}

func TestC1271_004_PhasetimingSuiteStillGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", phasetimingPkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", phasetimingPkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("the pre-existing phasetiming suite regressed — the context-fill fields must be purely additive:\n%s", out)
	}
}

func TestC1271_005_ContextfillStaysALeafImportedOnlyByTheWiringPackages(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", internalPattern)
	if code < 0 {
		t.Fatalf("go list failed to launch: code=%d err=%v\n%s", code, err, stderr)
	}
	if code != 0 {
		t.Fatalf("go list over %s failed (exit=%d) — the build graph must resolve (an import cycle fails here):\n%s", internalPattern, code, stderr)
	}
	allowedSuffixes := []string{"internal/phasetiming", "internal/core"}
	allowed := func(pkg string) bool {
		for _, sfx := range allowedSuffixes {
			if strings.HasSuffix(pkg, sfx) {
				return true
			}
		}
		return false
	}
	var importers []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		for _, imp := range fields[1:] {
			if imp == contextfillPkg {
				importers = append(importers, pkg)
			}
		}
	}
	if len(importers) == 0 {
		t.Errorf("no package imports %s — the whole point of this cycle is to wire the derivation into the durable telemetry record; it is still an orphan leaf", contextfillPkg)
	}
	for _, pkg := range importers {
		if !allowed(pkg) {
			t.Errorf("%s imports %s — this cycle wires the leaf into phasetiming/core ONLY (Stage dial and advisory prompt hint remain deferred)", pkg, contextfillPkg)
		}
	}
}
