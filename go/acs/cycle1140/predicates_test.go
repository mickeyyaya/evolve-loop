//go:build acs

// Package cycle1140 materialises the cycle-1140 acceptance criteria for the two
// fleet-scoped `## top_n` tasks pinned to this lane:
//
//   - phasecontract-role-artifact-ssot        (merges artifact-name-ssot-retro-backfill
//   - required-roles-ssot)
//   - optional-phase-ev-gating-by-cycle-class
//
// The deferred task (`triage-unified-solution-synthesis`) gets ZERO predicates
// here — R9.3: predicates bind only to triage-committed work.
//
// # Predicate strategy
//
// Every predicate CALLS the system under test and asserts on its return value
// or side effect — never a source-grep of production code (the cycle-85
// degenerate-predicate ban). Concretely:
//
//   - 001 calls phasecontract.For() and asserts on the returned Contract, plus a
//     negative lookup so a fail-open registry cannot satisfy it.
//   - 002 drives the REAL backfill.TryExtract code path over a temp workspace,
//     with the artifact filename SOURCED FROM phasecontract — so the two
//     independent filename declarations (backfill.phaseHeaders and
//     core.backfillArtifactPath) are pinned to the registry rather than to each
//     other. Includes an unknown-phase negative.
//   - 003 derives the required-role vocabulary from phasecontract.RequiredRoles()
//     and feeds a synthesized ledger to the REAL consumer
//     (redteamcheck.LedgerRoleCompleteness), asserting complete⇒pass and
//     role-removed⇒error. The consumer's own literal is thereby forced to agree
//     with the registry.
//   - 004 calls router.Route() on a trivial-class cycle whose advisor plan runs
//     an optional phase, and asserts the phase is SKIPPED — plus the two
//     anti-overreach negatives (non-trivial class still runs it; a floor phase is
//     never skipped by the same rule).
//
// # Test contract for Builder (do NOT modify this file)
//
// Predicate 003 requires one NEW exported accessor — the SSOT surface the
// required-roles half of task 1 exists to create:
//
//	// package phasecontract
//	func RequiredRoles() []string   // canonical roles every completed cycle must ledger
//
// Until it exists this package does not compile, which is the intended RED for
// the SSOT criterion (go/acs/README.md: a predicate package that fails to
// compile is a HARD suite error, never a silent PASS). Builder implements the
// accessor and re-points cyclehealth/redteamcheck/ledgerverify at it; the
// literal `[]string{"scout", "builder", "auditor"}` sites go away.
package cycle1140

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/redteamcheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// ---------------------------------------------------------------------------
// Task 1 — phasecontract-role-artifact-ssot
// ---------------------------------------------------------------------------

// TestC1140_001_RetroAndBuildPlannerRegisteredInPhaseContract pins AC-1: the two
// phases that have real artifacts and real backfill paths but are ABSENT from
// phasecontract.Contracts() (contract_registry.go:121-175) must be registered,
// with the artifact names the rest of the pipeline already writes/reads.
//
// RED today: For("retro") and For("build-planner") both return ok==false.
func TestC1140_001_RetroAndBuildPlannerRegisteredInPhaseContract(t *testing.T) {
	cases := []struct {
		phase        string
		artifactName string
	}{
		{"retro", "retrospective-report.md"},
		{"build-planner", "build-plan.md"},
	}
	for _, tc := range cases {
		c, ok := phasecontract.For(tc.phase)
		if !ok {
			t.Errorf("phasecontract.For(%q): not registered — the retro/build-planner artifact "+
				"vocabulary is still declared only in backfill.phaseHeaders and "+
				"core.backfillArtifactPath, with nothing tying them to the registry", tc.phase)
			continue
		}
		if c.ArtifactName != tc.artifactName {
			t.Errorf("phasecontract.For(%q).ArtifactName = %q, want %q (must match the filename "+
				"core.backfillArtifactPath already writes)", tc.phase, c.ArtifactName, tc.artifactName)
		}
		if c.Kind != phasecontract.KindMarkdown {
			t.Errorf("phasecontract.For(%q).Kind = %v, want KindMarkdown", tc.phase, c.Kind)
		}
		if c.WriteTarget != phasecontract.TargetWorkspace {
			t.Errorf("phasecontract.For(%q).WriteTarget = %q, want %q",
				tc.phase, c.WriteTarget, phasecontract.TargetWorkspace)
		}
		if c.Phase != tc.phase {
			t.Errorf("phasecontract.For(%q).Phase = %q, want %q (self-consistent key)",
				tc.phase, c.Phase, tc.phase)
		}
	}

	// NEGATIVE (anti-no-op): registering by making For() fail open — returning a
	// synthesized default for any name — would satisfy the loop above while
	// destroying the registry's meaning. An unregistered phase must still miss.
	if c, ok := phasecontract.For("no-such-phase-c1140"); ok {
		t.Errorf("phasecontract.For(\"no-such-phase-c1140\") = (%+v, true), want ok==false — "+
			"the registry must not fail open", c)
	}
}

// TestC1140_002_BackfillWritesTheArtifactNamePhaseContractDeclares pins AC-2:
// backfill's per-phase filename must AGREE with the registry, proved by running
// the real extraction and asserting the file the registry names is the file that
// appears with the content preserved.
//
// RED today: For() misses both phases, so artifactName is "" and the write
// target is a bare directory — the extraction cannot land at the declared name.
func TestC1140_002_BackfillWritesTheArtifactNamePhaseContractDeclares(t *testing.T) {
	cases := []struct {
		phase  string
		header string
	}{
		{"retro", "# Retrospective Report"},
		{"build-planner", "# Build Plan"},
	}

	for _, tc := range cases {
		ws := t.TempDir()
		body := tc.header + "\n\n## Section\n\n" +
			strings.Repeat("cycle-1140 backfill fixture body line.\n", 8)
		cleanPath := filepath.Join(ws, tc.phase+"-stdout.clean.txt")
		if err := os.WriteFile(cleanPath, []byte("preamble noise\n"+body), 0o644); err != nil {
			t.Fatalf("fixture write %s: %v", cleanPath, err)
		}

		c, ok := phasecontract.For(tc.phase)
		if !ok || c.ArtifactName == "" {
			t.Errorf("phasecontract.For(%q): no registered ArtifactName — backfill has no SSOT "+
				"to agree with (see predicate 001)", tc.phase)
			continue
		}
		artifactPath := c.ArtifactPath(phasecontract.Roots{Workspace: ws})

		extracted, err := backfill.TryExtract(ws, tc.phase, artifactPath, 50)
		if err != nil {
			t.Errorf("backfill.TryExtract(%q): unexpected error: %v", tc.phase, err)
			continue
		}
		if !extracted {
			t.Errorf("backfill.TryExtract(%q) = false — backfill does not recognise this phase's "+
				"header, so the registry name and backfill's phaseHeaders disagree", tc.phase)
			continue
		}
		got, rerr := os.ReadFile(artifactPath)
		if rerr != nil {
			t.Errorf("backfill wrote nothing at the registry-declared path %s: %v",
				filepath.Base(artifactPath), rerr)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(string(got)), tc.header) {
			t.Errorf("backfilled %s does not start at the phase header %q — got %.60q",
				filepath.Base(artifactPath), tc.header, string(got))
		}
	}

	// EDGE/NEGATIVE (anti-no-op): a phase backfill knows nothing about must still
	// extract nothing. A TryExtract rewritten to "write whatever is in clean.txt"
	// would pass the positives above and fail here.
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "nope-stdout.clean.txt"),
		[]byte("# Not A Known Header\n"+strings.Repeat("x\n", 40)), 0o644); err != nil {
		t.Fatalf("fixture write: %v", err)
	}
	out := filepath.Join(ws, "nope-report.md")
	extracted, err := backfill.TryExtract(ws, "nope", out, 50)
	if err != nil {
		t.Errorf("backfill.TryExtract(unknown phase): unexpected error: %v", err)
	}
	if extracted {
		t.Errorf("backfill.TryExtract(unknown phase) = true, want false")
	}
	if _, serr := os.Stat(out); serr == nil {
		t.Errorf("backfill wrote %s for an unknown phase, want no file", filepath.Base(out))
	}
}

// TestC1140_003_RequiredRoleVocabularyIsSingleSourced pins AC-3: the
// "what counts as a complete cycle" role vocabulary must come from ONE place.
// Four independent declarations exist today (ledgerverify/verify.go:53,
// cyclehealth/cyclehealth.go:203, redteamcheck/redteamcheck.go:108 inline, plus
// cyclehealth's requiredArtifacts sibling at :152).
//
// The predicate derives the role set from the registry accessor and drives a
// REAL consumer with it — so a consumer that keeps its own literal and drifts
// from the registry fails here, which is the whole point of the SSOT fix.
//
// RED today: phasecontract.RequiredRoles does not exist (compile failure).
func TestC1140_003_RequiredRoleVocabularyIsSingleSourced(t *testing.T) {
	roles := phasecontract.RequiredRoles()
	if len(roles) < 3 {
		t.Fatalf("phasecontract.RequiredRoles() = %v — want at least the three canonical roles; "+
			"an empty/stub accessor would make every check below vacuous", roles)
	}
	for _, want := range []string{"scout", "builder", "auditor"} {
		if !containsString(roles, want) {
			t.Errorf("phasecontract.RequiredRoles() = %v, missing canonical role %q", roles, want)
		}
	}

	// POSITIVE: a ledger carrying exactly the registry's roles must satisfy the
	// live consumer. If redteamcheck still demands a role the registry does not
	// declare, this fails — the drift the SSOT fix removes.
	full := filepath.Join(t.TempDir(), "ledger.jsonl")
	writeLedger(t, full, 1140, roles)
	if _, err := redteamcheck.LedgerRoleCompleteness(full); err != nil {
		t.Errorf("redteamcheck.LedgerRoleCompleteness on a ledger built from "+
			"phasecontract.RequiredRoles() = %v: %v — consumer vocabulary has drifted from the "+
			"registry", roles, err)
	}

	// NEGATIVE: dropping any single registry-declared role must still be caught.
	// This is the anti-no-op axis — an accessor that returns junk roles, or a
	// consumer relaxed into accepting anything, passes the positive and fails here.
	for _, drop := range roles {
		partial := filepath.Join(t.TempDir(), "ledger.jsonl")
		writeLedger(t, partial, 1140, without(roles, drop))
		if _, err := redteamcheck.LedgerRoleCompleteness(partial); err == nil {
			t.Errorf("redteamcheck.LedgerRoleCompleteness accepted a ledger missing role %q — "+
				"role completeness is no longer enforced for a registry-declared role", drop)
		}
	}
}

// TestC1140_006_CycleHealthAcceptsBothLedgerKinds pins the dual-kind acceptance
// the `required-roles-ssot` inbox item claimed was broken and that scout's
// live-code read found already working: cyclehealth's role completeness must
// count a role whether the ledger recorded it as `agent_subprocess` (bash
// dispatcher) or `phase` (Go-native orchestrator).
//
// GREEN today — a PIN, not a bug-fix RED (scout Beyond-the-Ask hypothesis 2). It
// fails if the task-1 SSOT migration reintroduces a Kind filter, or if
// cyclehealth's role set drifts from phasecontract.RequiredRoles().
func TestC1140_006_CycleHealthAcceptsBothLedgerKinds(t *testing.T) {
	roles := phasecontract.RequiredRoles()
	const cycle = 1140

	for _, kind := range []string{"agent_subprocess", "phase"} {
		ws := t.TempDir()
		writeLedgerKind(t, filepath.Join(ws, "ledger.jsonl"), cycle, roles, kind)
		rep, err := cyclehealth.Check(cyclehealth.Options{Cycle: cycle, Workspace: ws})
		if err != nil {
			t.Fatalf("cyclehealth.Check(kind=%s): %v", kind, err)
		}
		if msgs := anomaliesFor(rep, "ledger_completeness"); len(msgs) > 0 {
			t.Errorf("cyclehealth.Check rejected a complete kind=%q ledger built from "+
				"phasecontract.RequiredRoles() = %v: %v", kind, roles, msgs)
		}
	}

	// NEGATIVE (anti-no-op): completeness must still be enforced — a signal that
	// never reports would pass both positives above.
	ws := t.TempDir()
	writeLedgerKind(t, filepath.Join(ws, "ledger.jsonl"), cycle, without(roles, roles[0]), "phase")
	rep, err := cyclehealth.Check(cyclehealth.Options{Cycle: cycle, Workspace: ws})
	if err != nil {
		t.Fatalf("cyclehealth.Check(incomplete): %v", err)
	}
	if len(anomaliesFor(rep, "ledger_completeness")) == 0 {
		t.Errorf("cyclehealth.Check accepted a ledger missing role %q — completeness is not enforced",
			roles[0])
	}
}

// ---------------------------------------------------------------------------
// Task 2 — optional-phase-ev-gating-by-cycle-class
// ---------------------------------------------------------------------------

// TestC1140_004_TrivialCycleSkipsAdvisorInsertedOptionalPhases pins task 2:
// declarative skip_when conditions (phase-catalog `routing.skip_when` /
// policy.json — CONFIG, never a Go literal) must gate advisor-plan-driven
// optional-phase insertion, so a trivial-class cycle does not burn the measured
// 0.83M-1.67M cache-read tokens per optional phase.
//
// RED today: router.shouldRun's Advisory+plan branch returns planRuns() directly
// and never consults Cfg.Triggers[phase].SkipWhen — the plan wins unconditionally
// for every non-mandatory phase.
func TestC1140_004_TrivialCycleSkipsAdvisorInsertedOptionalPhases(t *testing.T) {
	const optional = "coverage-gate"

	// SKIP path: trivial class + a configured skip_when on the optional phase.
	dec := routeWith(t, "trivial", config.RoutingBlock{
		SkipWhen: []config.Condition{{Field: "cycle_size", Op: "eq", Value: "trivial"}},
	})
	if dec.NextPhase == optional {
		t.Errorf("router.Route dispatched %q on a trivial-class cycle — the configured "+
			"skip_when(cycle_size==trivial) did not gate the advisor's plan", optional)
	}
	if !containsString(dec.SkipPhases, optional) {
		t.Errorf("router.Route SkipPhases = %v, want %q recorded as skipped (the routing-plan "+
			"artifact must cite the skip, not silently drop it)", dec.SkipPhases, optional)
	}

	// NON-REGRESSION (semantic axis): the SAME config on a non-trivial cycle must
	// still run the phase. Guards against a fix that just disables the optionals.
	dec = routeWith(t, "medium", config.RoutingBlock{
		SkipWhen: []config.Condition{{Field: "cycle_size", Op: "eq", Value: "trivial"}},
	})
	if dec.NextPhase != optional {
		t.Errorf("router.Route NextPhase = %q on a medium-class cycle, want %q — cycle-class "+
			"gating must not disable optional phases outright", dec.NextPhase, optional)
	}
	if containsString(dec.SkipPhases, optional) {
		t.Errorf("router.Route skipped %q on a medium-class cycle: SkipPhases = %v",
			optional, dec.SkipPhases)
	}

	// EDGE (anti-overreach): no skip_when configured ⇒ the advisor's plan still
	// governs, even on a trivial cycle. The gate is config-driven, not a
	// hardcoded trivial-class blanket.
	dec = routeWith(t, "trivial", config.RoutingBlock{})
	if dec.NextPhase != optional {
		t.Errorf("router.Route NextPhase = %q with NO skip_when configured, want %q — the gate "+
			"must come from config, not a Go literal about cycle class", dec.NextPhase, optional)
	}
}

// TestC1140_005_CycleClassGateNeverSkipsFloorPhases is the integrity-floor
// negative for task 2: `ship ⇒ build ∧ audit ∧ (tdd unless trivial)` is
// non-configurable. A skip_when aimed at a floor phase must be refused even on a
// trivial cycle — otherwise the new gate becomes a floor bypass.
//
// GREEN today by construction (mandatory phases short-circuit before Triggers is
// ever consulted). It is a PIN, not a bug-fix RED: it fails the moment the task-2
// gate is wired in a way that reaches a floor phase.
func TestC1140_005_CycleClassGateNeverSkipsFloorPhases(t *testing.T) {
	dec := routeWithTarget(t, "trivial", "audit", config.RoutingBlock{
		SkipWhen: []config.Condition{{Field: "cycle_size", Op: "eq", Value: "trivial"}},
	})
	if containsString(dec.SkipPhases, "audit") {
		t.Errorf("router.Route skipped the floor phase \"audit\" on a trivial cycle: SkipPhases = %v — "+
			"the cycle-class gate must never reach a ship-chain phase", dec.SkipPhases)
	}
	if dec.NextPhase != "audit" {
		t.Errorf("router.Route NextPhase = %q, want \"audit\" (floor phase must still run)", dec.NextPhase)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// routeWith builds a minimal Advisory-stage RouteInput whose advisor plan runs
// "coverage-gate" between build and audit, with the given cycle class and
// trigger block, and returns the router's decision.
func routeWith(t *testing.T, cycleSize string, block config.RoutingBlock) router.RouterDecision {
	t.Helper()
	return routeWithTarget(t, cycleSize, "coverage-gate", block)
}

// routeWithTarget is routeWith with the trigger block attached to an arbitrary
// phase (used to aim a skip_when at a floor phase in predicate 005).
func routeWithTarget(t *testing.T, cycleSize, target string, block config.RoutingBlock) router.RouterDecision {
	t.Helper()
	// When the trigger block is aimed at a LATER phase (the floor-phase case),
	// the advisor's plan declines the earlier optional so the walk actually
	// reaches the target instead of stopping at coverage-gate.
	runOptional := target == "coverage-gate"
	in := router.RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Completed: []string{"scout", "build"},
		Signals: router.RoutingSignals{
			Triage: router.TriageSignals{CycleSize: cycleSize, Present: true},
		},
		Cfg: config.RoutingConfig{
			Stage:         config.StageAdvisory,
			Mandatory:     []string{"scout", "build", "audit"},
			MaxInsertions: 10,
			Order:         []string{"scout", "build", "coverage-gate", "audit", "ship"},
			Triggers:      map[string]config.RoutingBlock{target: block},
		},
		Plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{
			{Phase: "scout", Run: true},
			{Phase: "build", Run: true},
			{Phase: "coverage-gate", Run: runOptional, Justification: "advisor-inserted optional"},
			{Phase: "audit", Run: true},
		}},
	}
	return router.Route(in, nil)
}

// writeLedger emits a minimal ledger.jsonl: one agent_subprocess entry per role
// plus the cycle_terminal entry redteamcheck needs to pick the cycle.
func writeLedger(t *testing.T, path string, cycle int, roles []string) {
	t.Helper()
	writeLedgerKind(t, path, cycle, roles, "agent_subprocess")
}

// writeLedgerKind is writeLedger with the ledger `kind` under test (the bash
// dispatcher records agent_subprocess; the Go orchestrator records phase).
func writeLedgerKind(t *testing.T, path string, cycle int, roles []string, kind string) {
	t.Helper()
	var b strings.Builder
	for _, r := range roles {
		line, err := json.Marshal(map[string]any{
			"cycle": cycle, "kind": kind, "role": r,
		})
		if err != nil {
			t.Fatalf("marshal ledger entry: %v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	term, err := json.Marshal(map[string]any{"cycle": cycle, "kind": "cycle_terminal"})
	if err != nil {
		t.Fatalf("marshal terminal entry: %v", err)
	}
	b.Write(term)
	b.WriteByte('\n')
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write ledger %s: %v", path, err)
	}
}

// anomaliesFor returns the messages of every anomaly the report raised for one
// signal name.
func anomaliesFor(rep cyclehealth.Report, signal string) []string {
	var out []string
	for _, a := range rep.Anomalies {
		if a.Signal == signal {
			out = append(out, a.Message)
		}
	}
	return out
}

func containsString(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

func without(in []string, drop string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}
