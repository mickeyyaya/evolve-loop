//go:build acs

// Package cycle1271 materialises the cycle-1271 acceptance criteria for the two
// triage-committed top_n tasks (see scout-report.md / triage-report.md). Both
// close the FIRST of the follow-ups internal/contextfill's own header comment
// defers: the pure fill-ratio derivation exists but nothing imports it, so no
// cycle has ever recorded how full a phase's context window got.
//
//   - wire-contextfill-into-phasetiming-entry: phasetiming.Entry gains
//     ContextFillRatio/ContextWindowHot, populated at the ADR-0044 C1 chokepoint
//     (core.recordPhaseOutcome) from the tier the phase actually ran at, and
//     degrading to zero — never a fabricated ratio — when no tier is resolvable
//     (TestC1271_001, 002).
//   - contextfill-rollup-hot-phase-summary: phasetiming.Summary gains
//     HotPhaseCount/HotPhases, the cycle-level twin of the existing token rollup
//     (TestC1271_003).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-976 precedent). Each
// predicate shells `go test -run` over the RED tests authored this cycle in the
// two production packages; none is a source-grep (the cycle-85 degenerate-
// predicate ban). 002 is the WIRING PROOF — it drives the real production
// chokepoint (*Orchestrator).recordPhaseOutcome, the sole writer of
// phase-timing.json, so a derivation that exists only in a helper nobody calls
// stays RED. 005 asserts the scope constraint on the real build graph.
//
// RED now: Entry/Summary carry no such fields, so both target packages fail to
// COMPILE — the intended RED signal. GREEN once Builder adds the fields and the
// chokepoint derivation.
package cycle1271

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasetimingPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	corePkg        = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	contextfillPkg = "github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	// internalPattern is the build-graph scan target for the import-scope
	// predicate. It is a `go list` argv, never a `go test` one: no suite is run.
	internalPattern = "github.com/mickeyyaya/evolve-loop/go/internal/..."
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. Full import paths (never
// ./relative) so the invocation is independent of the predicate's own cwd;
// -count=1 defeats the test cache so current source is always exercised; every
// invocation is -run-narrowed to the tests this cycle owns. code < 0 is a launch
// failure (toolchain missing / killed), never a test verdict, so it is a hard
// error rather than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1271_001_EntryCarriesContextFillAndDegradesToAbsent — task-1 schema AC.
// phasetiming.Entry must carry ContextFillRatio/ContextWindowHot under the
// snake_case on-disk keys, round-trip them, OMIT them when unknown, and parse a
// legacy timing log (written before the fields existed) to their zero value
// without erroring. The omit + legacy halves are the anti-fabrication contract:
// "unknown fill" must stay distinguishable from a genuine 0.0.
func TestC1271_001_EntryCarriesContextFillAndDegradesToAbsent(t *testing.T) {
	ok, out := runGoTest(t, phasetimingPkg,
		"TestEntry_ContextFillFieldsRoundTripJSON|TestEntry_LegacyLogWithoutContextFillParsesToZero|TestEntry_ContextFillOmittedWhenUnknown")
	if !ok {
		t.Errorf("phasetiming.Entry does not carry the context-fill telemetry contract (fields absent, wrong JSON keys, missing omitempty, or a legacy log no longer parses):\n%s", out)
	}
}

// TestC1271_002_ChokepointDerivesFillFromRealDispatch — task-1 WIRING PROOF.
// The derivation must run inside the real ADR-0044 C1 chokepoint,
// (*Orchestrator).recordPhaseOutcome — the sole writer of phase-timing.json —
// producing exactly contextfill.FillRatio for a tier-resolvable phase, NOT
// flagging a cold phase hot, and leaving both fields zero when no canonical tier
// is resolvable (concrete model id, empty provenance, unknown tier). A
// contextfill call reachable only from a test would leave this RED.
func TestC1271_002_ChokepointDerivesFillFromRealDispatch(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestRecordPhaseOutcome_ProjectsContextFillWhenTierResolvable|TestRecordPhaseOutcome_ColdPhaseIsNotFlaggedHot|TestRecordPhaseOutcome_UnresolvableTierLeavesContextFillZero|TestRecordPhaseOutcome_ZeroTokensRecordsZeroFillNotHot|TestPhaseOutcomeFrom_CarriesTierProvenanceAndTokens")
	if !ok {
		t.Errorf("the context-fill ratio is not derived at the production recordPhaseOutcome chokepoint (unwired, imprecise, or fabricating a ratio when no tier is resolvable):\n%s", out)
	}
}

// TestC1271_003_RollupSummarisesHotPhases — task-2 AC. Rollup must produce
// HotPhaseCount/HotPhases from the per-entry fields, honour contextfill.IsHot's
// INCLUSIVE threshold boundary, report an empty list when nothing ran hot, never
// name a tier-unresolved (ratio 0) entry, and leave the existing duration
// aggregate untouched.
func TestC1271_003_RollupSummarisesHotPhases(t *testing.T) {
	ok, out := runGoTest(t, phasetimingPkg,
		"TestRollup_HotPhaseCountAndNames|TestRollup_NoHotPhasesReportsEmpty|TestRollup_HotThresholdBoundaryIsInclusive|TestRollup_EmptyEntriesNoHotFields")
	if !ok {
		t.Errorf("phasetiming.Rollup does not summarise hot phases correctly (fields absent, wrong threshold boundary, or a tier-unresolved entry reported hot):\n%s", out)
	}
}

// TestC1271_004_PhasetimingSuiteStillGreen — the no-regression AC for both
// tasks. The change is additive: the whole pre-existing phasetiming suite (a
// single small leaf package, ~instant) must still pass unchanged. Fails loudly
// if the new fields disturb the existing rollup, projection, or JSON contract.
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

// TestC1271_005_ContextfillStaysALeafImportedOnlyByTheWiringPackages — the
// scope AC (scout-report "Acceptance Criteria Summary", third bullet): this
// cycle wires the leaf into phasetiming/core ONLY. The Stage dial and the
// advisory prompt hint stay deferred, so no other package may import
// contextfill. Asserted against the REAL build graph via `go list` (which also
// fails outright on an import cycle), never a source grep.
func TestC1271_005_ContextfillStaysALeafImportedOnlyByTheWiringPackages(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", internalPattern)
	if code < 0 {
		t.Fatalf("go list failed to launch: code=%d err=%v\n%s", code, err, stderr)
	}
	if code != 0 {
		t.Fatalf("go list over %s failed (exit=%d) — the build graph must resolve (an import cycle fails here):\n%s", internalPattern, code, stderr)
	}
	// Module-relative suffixes, deliberately NOT written as full package
	// patterns: this predicate shells `go list`, never `go test`, and naming a
	// known-slow suite in package-pattern form here would read to the
	// flaky-shape lint as a suite shell that forgot its -run.
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
