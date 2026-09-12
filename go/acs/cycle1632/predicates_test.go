//go:build acs

// Package cycle1632 materializes the acceptance criteria for cycle 1632's
// single fleet-scoped task `tokenopt-handoff-digests` — the retry of the
// cycle-1593 attempt. That attempt was audit-FAILed for (H1) a shadow
// comparator that compared the raw legacy value against the newly-capped
// typed getter and so leaked the uncapped text into the ledger, and (H2/M1)
// a raw ctxSnap["carryover_summary"] writer that ADDED 218,480 bytes to every
// below-enforce triage prompt on a path that previously carried zero.
//
// The contract this cycle (single-sourced in internal/phaseio):
//
//	MaxFieldBytes    exported positive cap (bytes)
//	TruncationMarker exported non-empty visible marker
//	CapField(s)      identity at/under the cap; rune-aligned prefix + marker
//	                 above it, always <= MaxFieldBytes, idempotent
//	NewCycleInputs   applies CapField to Carryover and PreviousVerdict
//	shadow comparator compares CapField(legacy) against the typed getter
//	NO raw carryover_summary writer on any dispatch path
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 oversized field capped with a visible marker (exported helper)  → TestC1632_001_OversizedFieldIsCappedWithVisibleMarker
//	AC2 the cap reaches the real dispatch → triage prompt at enforce     → TestC1632_002_EnforceDispatchBoundsCarryoverInRealTriagePrompt
//	AC3 empty / exact-boundary / multibyte inputs stay safe             → TestC1632_003_BoundaryAndMultibyteInputsStaySafe
//	AC4 cap-equivalent legacy value → no false shadow mismatch (live)   → TestC1632_004_ShadowDispatchEmitsNoFalseMismatchForCappedCarryover
//	AC5 a genuinely mutated typed value is still reported as drift      → TestC1632_005_GenuineDriftStillDetected_BindsCoreTest
//	AC6 StageOff/StageShadow mint no raw carryover bytes (baseline 0)   → TestC1632_006_NoRawCarryoverWriterBelowEnforce
//
// Audit-repair round (audit round 1 FAILed the build on H1/M1, L1 prescribed):
//
//	AC7 (H1) build-report.md claims no full closure of the partially-met
//	    inbox item and names where the unmet remainder is queued          → TestC1632_007_BuildReportDoesNotClaimFullClosureOfPartialInboxItem
//	AC8 (H1) the unmet remainder (per-edge explicit artifact-flow config;
//	    the ComposePrompt phases still without a digest; instead-of, not
//	    additive) is a tracked, claimable, rankable .evolve/inbox item    → TestC1632_008_UnmetRemainderIsQueuedAsTrackedInboxItem
//	AC9 (M1) an oversized scalar in ANY one UpstreamDigest section can
//	    never evict another present section (degraded rows survive)       → TestC1632_009_UpstreamDigestOversizedScalarCannotEvictOtherSections
//	AC10 (L1) tdd renders the digest at internal/phaseio's package-default
//	    cap — no forked literal at the call site                          → TestC1632_010_TDDPromptDigestCapIsThePackageDefault
//
// Audit round 2 (the rebuild's `Status: PASS` rested on round 1's suite line):
//
//	AC11 (H1) every `[acs suite]` receipt in build-report.md is this cycle's,
//	     green, and counts >= the declared TestC1632_ predicates             → TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound
//
// Adversarial axes (skills/adversarial-testing §6):
//   - NEGATIVE — 001 rejects a silent prefix cut and an unchanged pass-through;
//     005 is the anti-blanket-suppression case (a comparator that simply
//     skips long fields passes 004 and FAILS 005); 006 fails the moment any
//     writer mints the key.
//   - EDGE / OOD — 003 drives "", the exact boundary, cap-1, and 2/3/4-byte
//     runes straddling the boundary.
//   - SEMANTIC — 001/003 pin the REPRESENTATION, 002 pins REACHABILITY through
//     the production dispatch seam into the real triage prompt, 004 pins the
//     COMPARATOR through the live shadow artifact + ledger, 006 pins the
//     PROMPT BASELINE. Five distinct behaviors, not one restated.
//
// No grep-only predicates (cycle-85 ban): 001/003 call the exported helper
// and DTO constructor; 002/004/006 drive the real core.Orchestrator.RunCycle
// over the production PhaseIO seam (fixtures fakes, a real git worktree so the
// Build explanation contract seals) and compose the REAL triage prompt from
// the dispatched request; 005 binds the frozen core unit test by its
// `--- PASS:` marker (one package, -run-narrowed — the cycle-976/1587
// precedent) using the shared "binding test X did NOT pass" vocabulary so a
// phantom (renamed / never-written) binding is classified, not just red.
//
// Reachability probe (cycle-644 rule): every package-qualified pin here was
// compiled from this package during authoring — core already imports phaseio
// (internal/core/phaseio_shadow.go), and acs/cycle1632 → core/triage/phaseio/
// fixtures is a leaf import; no cycle is possible.
package cycle1632

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// carryoverLine is the exact prompt line triage renders for the carryover
// (internal/phases/triage/triage.go ComposePrompt).
const carryoverLine = "- carryover_summary: "

// ---------------------------------------------------------------------------
// Fixtures: a real git worktree (the fresh-cycle Build explanation contract
// requires a resolvable base SHA), a production-shaped routing config with
// triage on the spine, and one RunCycle driver.
// ---------------------------------------------------------------------------

type tempWorktree struct{ path string }

func (w *tempWorktree) Create(string, int) (string, error) { return w.path, nil }
func (w *tempWorktree) Cleanup(string, string) error       { return nil }

// gitRepo returns a temp dir holding one empty commit. Every git call is
// -C-anchored (never resolves the repo from process cwd).
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.email=acs@evolve", "-c", "user.name=acs"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("commit", "-q", "--allow-empty", "-m", "base")
	return dir
}

func routingCfg(stage config.Stage) config.RoutingConfig {
	cfg := config.RoutingConfig{
		Stage:         config.StageEnforce,
		Mode:          config.ModeStaticPreset,
		Mandatory:     []string{"scout", "triage", "build", "audit", "ship"},
		Conditional:   map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers:      map[string]config.RoutingBlock{},
	}
	cfg.PhaseIO = stage
	return cfg
}

type cycleRun struct {
	runners map[core.Phase]core.PhaseRunner
	ledger  *fixtures.FakeLedger
	triage  *fixtures.FakeRunner
}

// runCycle drives one full production RunCycle at the given PhaseIO stage
// with the given request Context and carryover backlog in state, and returns
// the dispatched requests. Fails (never skips) if triage was not dispatched,
// so every assertion over triage requests is non-vacuous.
func runCycle(t *testing.T, stage config.Stage, reqCtx map[string]string, backlog []core.CarryoverTodo) cycleRun {
	t.Helper()
	st := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 0, CarryoverTodos: backlog}}
	led := &fixtures.FakeLedger{}
	runners := fixtures.BuildRunners(nil)
	o := core.NewOrchestrator(st, led, runners,
		core.WithRouting(routingCfg(stage), router.StaticPreset{}),
		core.WithWorktreeProvisioner(&tempWorktree{path: gitRepo(t)}))
	if _, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "c1632", Context: reqCtx,
	}); err != nil {
		t.Fatalf("RunCycle(PhaseIO=%v): %v", stage, err)
	}
	tr := runners[core.PhaseTriage].(*fixtures.FakeRunner)
	if len(tr.Requests) == 0 {
		t.Fatalf("triage was never dispatched at PhaseIO=%v — prompt assertions would be vacuous", stage)
	}
	return cycleRun{runners: runners, ledger: led, triage: tr}
}

// triagePrompt composes the REAL triage prompt for a dispatched request (the
// exported BaseRunner.ComposePrompt seam, no bridge launch).
func triagePrompt(stage config.Stage, req core.PhaseRequest) string {
	return triage.New(triage.Config{PhaseIO: stage}).ComposePrompt("BODY", req)
}

// overCap is a valid-UTF-8 value of MaxFieldBytes+extra bytes made of rune r.
func overCap(r rune, extra int) string {
	w := len(string(r))
	return strings.Repeat(string(r), (phaseio.MaxFieldBytes+extra+w-1)/w)
}

// ---------------------------------------------------------------------------
// AC1 — representation: oversized field capped with a visible marker.
// ---------------------------------------------------------------------------

func TestC1632_001_OversizedFieldIsCappedWithVisibleMarker(t *testing.T) {
	if phaseio.MaxFieldBytes <= 0 || phaseio.MaxFieldBytes > 64*1024 {
		t.Fatalf("MaxFieldBytes=%d: want 0 < cap <= 64KiB (cycle-1593 measured a 218,480-byte carryover; a cap above that bounds nothing)", phaseio.MaxFieldBytes)
	}
	if phaseio.TruncationMarker == "" || !utf8.ValidString(phaseio.TruncationMarker) || len(phaseio.TruncationMarker) >= phaseio.MaxFieldBytes {
		t.Fatalf("TruncationMarker=%q: want non-empty valid UTF-8 shorter than the cap", phaseio.TruncationMarker)
	}
	// The cycle-1593 measured incident size, and the smallest over-cap input.
	for _, in := range []string{overCap('x', 218480-phaseio.MaxFieldBytes), overCap('y', 1)} {
		got := phaseio.CapField(in)
		if len(got) > phaseio.MaxFieldBytes {
			t.Errorf("len(CapField(%d bytes))=%d > MaxFieldBytes=%d", len(in), len(got), phaseio.MaxFieldBytes)
		}
		if got == in {
			t.Errorf("CapField passed a %d-byte value through unchanged — cap inert", len(in))
		}
		if !strings.Contains(got, phaseio.TruncationMarker) {
			t.Errorf("capped value carries no visible marker %q", phaseio.TruncationMarker)
		}
		// NEGATIVE: a silent byte cut is not a cap the reader can see.
		if got == in[:phaseio.MaxFieldBytes] {
			t.Errorf("CapField is a silent prefix cut with no marker")
		}
		if i := strings.Index(got, phaseio.TruncationMarker); i >= 0 && !strings.HasPrefix(in, got[:i]) {
			t.Errorf("retained text is not a prefix of the input (value rewritten, not truncated)")
		}
		if twice := phaseio.CapField(got); twice != got {
			t.Errorf("CapField is not idempotent: %d bytes → %d bytes on a second pass", len(got), len(twice))
		}
	}
	// The DTO stores exactly the helper's output for BOTH free-text fields.
	big := overCap('z', 7)
	ci := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: big, PreviousVerdict: big})
	if ci.Carryover() != phaseio.CapField(big) || ci.PreviousVerdict() != phaseio.CapField(big) {
		t.Errorf("NewCycleInputs did not apply CapField to Carryover/PreviousVerdict: carryover=%d previous_verdict=%d bytes, want %d", len(ci.Carryover()), len(ci.PreviousVerdict()), len(phaseio.CapField(big)))
	}
}

// ---------------------------------------------------------------------------
// AC2 — reachability: the cap reaches the real dispatch seam and the real
// triage prompt at enforce (assembleCycleInputs → buildPhaseInput →
// triage.ComposePrompt). A cap that only lives in a unit test is dead code.
// ---------------------------------------------------------------------------

func TestC1632_002_EnforceDispatchBoundsCarryoverInRealTriagePrompt(t *testing.T) {
	raw := overCap('r', 218480-phaseio.MaxFieldBytes)
	run := runCycle(t, config.StageEnforce, map[string]string{"goal": "g", "strategy": "s", "carryover_summary": raw}, nil)
	for i, req := range run.triage.Requests {
		if !req.Input.Active() {
			t.Fatalf("triage request[%d]: typed PhaseInput inactive at enforce — the seam under test was not reached", i)
		}
		typed := req.Input.CycleInputs().Carryover()
		if typed != phaseio.CapField(raw) {
			t.Errorf("request[%d]: typed Carryover() (%d bytes) != CapField(raw) (%d bytes) — cap not applied on the dispatch path", i, len(typed), len(phaseio.CapField(raw)))
		}
		prompt := triagePrompt(config.StageEnforce, req)
		if !strings.Contains(prompt, carryoverLine+phaseio.CapField(raw)+"\n") {
			t.Errorf("request[%d]: real triage prompt does not render the bounded carryover line", i)
		}
		if strings.Contains(prompt, raw) {
			t.Errorf("request[%d]: real triage prompt embeds the full %d-byte raw carryover — the cap is inert on the prompt path", i, len(raw))
		}
		if len(prompt) >= len(raw) {
			t.Errorf("request[%d]: triage prompt is %d bytes for a %d-byte carryover — no reduction reached the prompt", i, len(prompt), len(raw))
		}
	}
}

// ---------------------------------------------------------------------------
// AC3 — edge / OOD: empty, exact boundary, cap-1 round-trip byte-identical;
// multibyte over-cap input never splits a rune.
// ---------------------------------------------------------------------------

func TestC1632_003_BoundaryAndMultibyteInputsStaySafe(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"empty", ""},
		{"cap-minus-one", strings.Repeat("u", phaseio.MaxFieldBytes-1)},
		{"exact-boundary", strings.Repeat("e", phaseio.MaxFieldBytes)},
	} {
		ci := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: tc.in, PreviousVerdict: tc.in})
		if ci.Carryover() != tc.in || ci.PreviousVerdict() != tc.in || phaseio.CapField(tc.in) != tc.in {
			t.Errorf("%s: at/under-cap value must round-trip byte-identical (len=%d)", tc.name, len(tc.in))
		}
	}
	for _, tc := range []struct {
		name string
		r    rune
	}{{"2-byte", 'é'}, {"3-byte", '日'}, {"4-byte", '😀'}} {
		in := overCap(tc.r, 1) // the byte cut at MaxFieldBytes lands mid-rune
		got := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: in}).Carryover()
		if len(got) > phaseio.MaxFieldBytes || !utf8.ValidString(got) {
			t.Errorf("%s: capped value len=%d valid=%v — want <= %d and valid UTF-8", tc.name, len(got), utf8.ValidString(got), phaseio.MaxFieldBytes)
		}
		idx := strings.Index(got, phaseio.TruncationMarker)
		if idx < 0 {
			t.Errorf("%s: no marker on a multibyte over-cap value", tc.name)
			continue
		}
		for _, rr := range got[:idx] {
			if rr != tc.r {
				t.Errorf("%s: retained prefix holds rune %U, want only %U — a rune was split at the boundary", tc.name, rr, tc.r)
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC4 — comparator through the LIVE shadow path: an over-cap legacy value
// dispatched at StageShadow yields no cycle_inputs.carryover mismatch in the
// shadow artifact or the ledger, and no ledger message carries the raw text.
// Guards the cycle-1593 round-2 half-fix (cap in the DTO, raw compare in the
// comparator).
// ---------------------------------------------------------------------------

type shadowDoc struct {
	Phase      string `json:"phase"`
	Mismatches []struct {
		Field string `json:"field"`
		Want  string `json:"want"`
		Got   string `json:"got"`
	} `json:"mismatches"`
}

func TestC1632_004_ShadowDispatchEmitsNoFalseMismatchForCappedCarryover(t *testing.T) {
	raw := overCap('s', 218480-phaseio.MaxFieldBytes)
	run := runCycle(t, config.StageShadow, map[string]string{"goal": "g", "strategy": "s", "carryover_summary": raw}, nil)
	req := run.triage.Requests[0]
	// The DTO must have capped on this path — otherwise "no mismatch" is the
	// trivial raw==raw equality, not cap-awareness.
	if got := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: req.Context["carryover_summary"]}).Carryover(); len(got) > phaseio.MaxFieldBytes {
		t.Fatalf("NewCycleInputs does not bound the legacy value on this path (%d bytes) — AC1 must land first", len(got))
	}
	path := filepath.Join(req.Workspace, "phaseio-shadow-triage.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("shadow artifact not written at StageShadow: %v", err)
	}
	var doc shadowDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("shadow artifact %s unparsable: %v", path, err)
	}
	for _, m := range doc.Mismatches {
		if m.Field == "cycle_inputs.carryover" {
			t.Errorf("false shadow mismatch on cycle_inputs.carryover for a cap-equivalent value: want=%d bytes got=%d bytes", len(m.Want), len(m.Got))
		}
		if len(m.Want) > phaseio.MaxFieldBytes || len(m.Got) > phaseio.MaxFieldBytes {
			t.Errorf("shadow artifact leaks an uncapped value on %s (want=%d got=%d bytes)", m.Field, len(m.Want), len(m.Got))
		}
	}
	for _, e := range run.ledger.Entries {
		if e.Kind != "phaseio_shadow_mismatch" {
			continue
		}
		if strings.Contains(e.Message, "cycle_inputs.carryover") {
			t.Errorf("ledger phaseio_shadow_mismatch entry names cycle_inputs.carryover for a cap-equivalent value")
		}
		if strings.Contains(e.Message, raw) {
			t.Errorf("ledger phaseio_shadow_mismatch entry embeds the %d-byte raw carryover (the cycle-1593 H1 leak)", len(raw))
		}
	}
}

// ---------------------------------------------------------------------------
// AC5 — the negative: a typed value that genuinely differs INSIDE the retained
// prefix is still reported (with a bounded want). compareCycleInputsShadow is
// package-private, so this binds the frozen core unit test by name and
// requires its PASS marker — a renamed or never-written test is a phantom
// binding, never a silent green.
// ---------------------------------------------------------------------------

func TestC1632_005_GenuineDriftStillDetected_BindsCoreTest(t *testing.T) {
	const name = "TestCompareCycleInputsShadow_DetectsRealDrift"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^"+name+"$", corePkg)
	if code < 0 {
		t.Fatalf("go test failed to launch: code=%d err=%v\n%s", code, err, stderr)
	}
	if code != 0 || !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("binding test %s did NOT pass (exit %d)\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
	}
}

// ---------------------------------------------------------------------------
// AC6 — prompt baseline: with a populated carryover backlog in state and no
// carryover_summary in the request, StageOff and StageShadow dispatch NO
// carryover bytes — no Context key minted, zero PhaseInput, and the real
// triage prompt has no carryover line. Enforce is included: no writer may
// exist on ANY path (cycle-1593 M1 measured +4,096 there too).
// ---------------------------------------------------------------------------

func TestC1632_006_NoRawCarryoverWriterBelowEnforce(t *testing.T) {
	backlog := []core.CarryoverTodo{
		{ID: "todo-big-1", Action: overCap('a', 100000), Priority: "high"},
		{ID: "todo-big-2", Action: overCap('b', 100000), Priority: "high"},
	}
	for _, tc := range []struct {
		name  string
		stage config.Stage
	}{{"off", config.StageOff}, {"shadow", config.StageShadow}, {"enforce", config.StageEnforce}} {
		t.Run(tc.name, func(t *testing.T) {
			run := runCycle(t, tc.stage, map[string]string{"goal": "g", "strategy": "s"}, backlog)
			for phase, r := range run.runners {
				for i, req := range r.(*fixtures.FakeRunner).Requests {
					if v, has := req.Context["carryover_summary"]; has {
						t.Errorf("phase %s request[%d]: Context[carryover_summary] minted (%d bytes) — a raw carryover writer exists", phase, i, len(v))
					}
					if tc.stage < config.StageEnforce && !reflect.DeepEqual(req.Input, phaseio.PhaseInput{}) {
						t.Errorf("phase %s request[%d]: PhaseInput not zero below enforce", phase, i)
					}
					if req.Input.Active() && req.Input.CycleInputs().Carryover() != "" {
						t.Errorf("phase %s request[%d]: typed Carryover() non-empty (%d bytes) with no carryover in the request", phase, i, len(req.Input.CycleInputs().Carryover()))
					}
				}
			}
			for i, req := range run.triage.Requests {
				prompt := triagePrompt(tc.stage, req)
				if strings.Contains(prompt, carryoverLine) {
					t.Errorf("request[%d]: real triage prompt renders a carryover line from state backlog — baseline is zero bytes", i)
				}
				for _, todo := range backlog {
					if strings.Contains(prompt, todo.ID) || strings.Contains(prompt, todo.Action[:64]) {
						t.Errorf("request[%d]: real triage prompt embeds backlog todo %s", i, todo.ID)
					}
				}
			}
		})
	}
}

// ===========================================================================
// Audit-repair round — the audit's own findings, encoded as RED before the
// rebuild (never a weakening of the six predicates above).
//
//   H1  build-report.md declared `Closes-Inbox: tokenopt-handoff-digests` for
//       an item whose second acceptance criterion (per-edge explicit
//       artifact-flow config) is unimplemented and whose first is met for one
//       phase, additively; committedInboxIDs (internal/phases/ship/postship.go)
//       unions triage top_n ∪ lane-scope ∪ the marker, so a PASS landing
//       retires the item with the unmet remainder recorded NOWHERE — the
//       third audit on this lane to find it (cycle-1604 debb0673…/d7f2448d…).
//   M1  Handoffs.UpstreamDigest renders the typed views first and truncates
//       blindly at the rune cap with no per-section budget, so one oversized
//       agent-authored scalar (ScoutView.CycleSizeEstimate is copied
//       unvalidated from JSON) evicts every `degraded:` row (cycle-1604
//       df875c36…/d69bd9f3…). TestC1604_003 could not observe it because its
//       fixture set only Degraded.
//   L1  tdd.go passes a bare 1024 literal that forks the unexported
//       defaultUpstreamDigestRunes (cycle-1604 dbd6c43a…/dd88cbdc…).
//
// Dual-root idiom (go/acs/README.md): EVOLVE_PROJECT_ROOT → the STATE root
// (.evolve/runs/cycle-1632/build-report.md lives on main), EVOLVE_WORKTREE_ROOT
// → the SOURCE root (the tracked .evolve/inbox copy and go/ sources the ship
// lands). Each falls back to the repo root the suite runs from.
// ===========================================================================

// laneItemID is the fleet-scoped inbox item this lane is bound to
// (lane-scope.json / triage-decision.json top_n).
const laneItemID = "tokenopt-handoff-digests"

// thisCycle is the run whose build-report.md postship.go's committedInboxIDs
// reads at landing (readBuildReport(cycleDir)).
const thisCycle = 1632

var (
	// The unmet remainder, in the audit's own words: "AC2 per-edge explicit
	// artifact-flow config; the six ComposePrompt phases without a digest;
	// instead-of rather than additive".
	perEdgeRe   = regexp.MustCompile(`(?i)per[- ](phase[- ])?edge`)
	configRe    = regexp.MustCompile(`(?i)config`)
	insteadOfRe = regexp.MustCompile(`(?i)instead[- ]of`)
	phasesRe    = regexp.MustCompile(`(?i)(ComposePrompt|(every|each|all|remaining|other|six)\s+(phase|prompt))`)
	// Mirrors inboxmover's inboxIDPattern: the id shape a Closes-Inbox marker
	// and an inbox filename can carry.
	inboxIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// stateRoot resolves the MAIN project root (runtime state under .evolve/).
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// sourceRoot resolves the cycle worktree (the tree the ship lands).
func sourceRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv(ipcenv.WorktreeRootKey); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// cycleRunDir returns this cycle's workspace on the STATE root. Absent runtime
// state (an archived run, a bare export) is the documented SKIP posture, never
// a false red.
func cycleRunDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(stateRoot(t), ".evolve", "runs", fmt.Sprintf("cycle-%d", thisCycle))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("cycle workspace %s absent (runtime state not present): %v", dir, err)
	}
	return dir
}

// refersToLaneItem reports whether an inbox item links itself to the lane's
// item through the machine-read edges (connects_to / deps), not prose.
func refersToLaneItem(it inboxbatch.Item) bool {
	for _, edge := range append(append([]string{}, it.ConnectsTo...), it.Deps...) {
		if strings.Contains(edge, laneItemID) {
			return true
		}
	}
	return false
}

// remainderItems loads the inbox ROOT through the real loader (the same
// inboxbatch.LoadDir the lane-scope resolver and `evolve inbox batches` use —
// processing/ and consumed/ are not the queue) and returns every item that is
// not the lane item itself, links to it, and whose acceptance names all four
// parts of the unmet remainder. Candidates that link but do not name the
// remainder are logged so a near-miss is diagnosable, not just red.
func remainderItems(t *testing.T, src string) (matched []inboxbatch.Item, warnings []string) {
	t.Helper()
	items, warnings, err := inboxbatch.LoadDir(filepath.Join(src, ".evolve", "inbox"))
	if err != nil {
		t.Fatalf("inboxbatch.LoadDir(%s/.evolve/inbox): %v", src, err)
	}
	for _, it := range items {
		if it.ID == laneItemID || !refersToLaneItem(it) {
			continue
		}
		acc := strings.Join(it.Acceptance, "\n")
		var missing []string
		for _, want := range []struct {
			name string
			re   *regexp.Regexp
		}{{"per-edge", perEdgeRe}, {"config", configRe}, {"instead-of", insteadOfRe}, {"remaining ComposePrompt phases", phasesRe}} {
			if !want.re.MatchString(acc) {
				missing = append(missing, want.name)
			}
		}
		if len(missing) > 0 {
			t.Logf("inbox item %s links to %s but its acceptance does not name: %v", it.ID, laneItemID, missing)
			continue
		}
		matched = append(matched, it)
	}
	return matched, warnings
}

// ---------------------------------------------------------------------------
// AC7 (H1) — the landing report is honest: it does not assert full closure of
// the partially-met item, it names the unmet criterion, and it points at the
// queued remainder. Runs the SAME parser postship.go's committedInboxIDs runs
// over the SAME file (readBuildReport(cycleDir)), so what this predicate sees
// is exactly what the landing would union into the consumed set.
// ---------------------------------------------------------------------------

func TestC1632_007_BuildReportDoesNotClaimFullClosureOfPartialInboxItem(t *testing.T) {
	runDir := cycleRunDir(t)
	body, err := os.ReadFile(filepath.Join(runDir, "build-report.md"))
	if err != nil {
		t.Fatalf("build-report.md unreadable in %s: %v — the build must produce the report the landing reads", runDir, err)
	}
	closes := inboxmover.ClosesInboxIDs(body)
	for _, id := range closes {
		if id == laneItemID {
			t.Errorf("build-report.md declares `Closes-Inbox: %s` while inbox acceptance[1] (per-edge explicit artifact-flow config) is unimplemented and acceptance[0] is met for one phase, additively — a PASS landing would retire the item with the unmet remainder recorded nowhere (cycle-1604 debb0673…/d7f2448d…, third audit on this lane)", laneItemID)
		}
	}
	report := string(body)
	if !perEdgeRe.MatchString(report) {
		t.Errorf("build-report.md never names the unmet per-edge artifact-flow criterion — the Handoff Summary must state what this landing does NOT deliver")
	}
	matched, _ := remainderItems(t, sourceRoot(t))
	if len(matched) == 0 {
		t.Errorf("build-report.md cannot point at a queued remainder because none exists at the inbox root (see TestC1632_008) — the unmet criteria are recorded nowhere durable")
		return
	}
	for _, rem := range matched {
		if !strings.Contains(report, rem.ID) {
			t.Errorf("build-report.md never names the remainder inbox item %s — the unmet criteria are not traceable from the landing report", rem.ID)
		}
		for _, id := range closes {
			if id == rem.ID {
				t.Errorf("build-report.md declares `Closes-Inbox: %s` for the remainder it just queued", rem.ID)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC8 (H1) — the remainder is DURABLE: exactly one fresh inbox-root item links
// to the lane item, names all four parts of the unmet work in its acceptance
// (the words the ADR-0098 Task Contract will project next time), is resolvable
// by the real claim-path resolver, is rankable, and is git-tracked (cycle-93:
// on-disk-but-untracked is dropped at ship). A scout-report `## Deferred`
// paragraph is prose, not a queue.
// ---------------------------------------------------------------------------

func TestC1632_008_UnmetRemainderIsQueuedAsTrackedInboxItem(t *testing.T) {
	src := sourceRoot(t)
	inboxDir := filepath.Join(src, ".evolve", "inbox")
	matched, warnings := remainderItems(t, src)
	if len(matched) != 1 {
		t.Fatalf("want exactly ONE inbox-root item that links to %s (connects_to/deps) and whose acceptance names per-edge config, instead-of (not additive) and the remaining ComposePrompt phases; got %d", laneItemID, len(matched))
	}
	rem := matched[0]
	if !inboxIDRe.MatchString(rem.ID) {
		t.Errorf("remainder id %q is not a valid inbox id (Closes-Inbox / filename shape)", rem.ID)
	}
	if rem.Title == "" {
		t.Errorf("remainder %s has no title — it renders as an empty Task Contract heading", rem.ID)
	}
	if rem.Weight <= 0 {
		t.Errorf("remainder %s has weight %v — an unweighted item sorts to the bottom of every triage menu and is lost in practice", rem.ID, rem.Weight)
	}
	if rem.Continuation != nil {
		t.Errorf("remainder %s carries a continuation binding — the remainder must start fresh, not resume the FAILed lane's snapshot", rem.ID)
	}
	// The real claim-path resolver (inboxmover.Claim → FindFileByTaskID) must
	// find it by id at the inbox root.
	path, err := inboxmover.FindFileByTaskID(inboxDir, rem.ID)
	if err != nil {
		t.Errorf("inboxmover.FindFileByTaskID(%s, %s): %v — the remainder is not claimable", inboxDir, rem.ID, err)
	} else if filepath.Base(path) != rem.Path {
		t.Errorf("FindFileByTaskID resolved %s to %s, LoadDir loaded it from %s — duplicate id at the inbox root", rem.ID, filepath.Base(path), rem.Path)
	}
	for _, w := range warnings {
		if strings.HasPrefix(w, rem.Path+":") {
			t.Errorf("the loader sanitised the remainder file: %s", w)
		}
	}
	rel := filepath.Join(".evolve", "inbox", rem.Path)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", src, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is on disk but not git-tracked — it would be dropped at ship (cycle-93)", rel)
	}
}

// ---------------------------------------------------------------------------
// AC9 (M1) — no section of UpstreamDigest can evict another. With every typed
// view present, one degraded read-miss and one generic key, an oversized value
// placed in ANY one section must leave every other section rendered, the
// degraded row still naming its edge, the total under the cap, and the
// oversized value absent verbatim. A blind post-render truncation fails every
// placement; reordering degraded rows first still fails the scout/triage/
// generic placements (the row that follows the oversized one is evicted).
// ---------------------------------------------------------------------------

// digestBase is the all-sections fixture the placements mutate.
func digestBase() phaseio.HandoffsInit {
	return phaseio.HandoffsInit{
		Scout:    &phaseio.ScoutView{CycleSizeEstimate: "small", ItemCount: 4, CarryoverCount: 2, BacklogSize: 9},
		Triage:   &phaseio.TriageView{CycleSize: "small", PhaseSkip: []string{"tdd"}},
		Build:    &phaseio.BuildView{Verdict: "PASS", ACSGreen: 3, ACSTotal: 3, SeverityMax: "LOW"},
		Audit:    &phaseio.AuditView{Verdict: "FAIL", Confidence: 0.9, RedCount: 2},
		Degraded: []string{"build: handoff-build.json: permission denied"},
		Generic:  map[string]any{"scout.notes": "n"},
	}
}

// compactLines trims every line of s to width runes for failure output (the
// oversized fixture would otherwise dump 4 KiB per assertion).
func compactLines(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if r := []rune(ln); len(r) > width {
			lines[i] = string(r[:width]) + "…(" + fmt.Sprint(len(r)) + " runes)"
		}
	}
	return strings.Join(lines, "\n")
}

func TestC1632_009_UpstreamDigestOversizedScalarCannotEvictOtherSections(t *testing.T) {
	huge := strings.Repeat("x", 4096) // > any cap under test, > the package default
	placements := []struct {
		name string
		put  func(*phaseio.HandoffsInit)
	}{
		{"scout-scalar", func(i *phaseio.HandoffsInit) { i.Scout.CycleSizeEstimate = huge }},
		{"triage-scalar", func(i *phaseio.HandoffsInit) { i.Triage.CycleSize = huge }},
		{"generic-value", func(i *phaseio.HandoffsInit) { i.Generic["scout.notes"] = huge }},
		{"degraded-entry", func(i *phaseio.HandoffsInit) { i.Degraded = []string{"build: handoff-build.json: " + huge} }},
	}
	sections := []string{"scout:", "triage:", "build:", "audit:", "degraded:", "generic "}
	for _, cap := range []int{512, 0} { // an explicit cap, and the package default
		for _, pl := range placements {
			t.Run(fmt.Sprintf("cap%d/%s", cap, pl.name), func(t *testing.T) {
				init := digestBase()
				pl.put(&init)
				got := phaseio.NewHandoffs(init).UpstreamDigest(cap)
				if n := len([]rune(got)); cap > 0 && n > cap {
					t.Errorf("digest is %d runes, cap %d", n, cap)
				}
				if strings.Contains(got, huge) {
					t.Errorf("the %d-rune value is embedded verbatim — no section bound applied", len(huge))
				}
				lines := strings.Split(got, "\n")
				for _, sec := range sections {
					found := false
					for _, ln := range lines {
						if strings.HasPrefix(ln, sec) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("section %q evicted by the oversized %s:\n%s", sec, pl.name, compactLines(got, 72))
					}
				}
				for _, ln := range lines {
					if strings.HasPrefix(ln, "degraded:") && !strings.Contains(ln, "build") {
						t.Errorf("degraded row no longer names its edge (R5 read-miss made anonymous): %q", ln)
					}
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// AC10 (L1) — the tdd phase renders the digest at internal/phaseio's
// package-default cap. Behavioral half: for an upstream whose rendering
// exceeds 1024 runes, the fenced block in the REAL tdd prompt must equal the
// package-default rendering byte-for-byte (a forked literal that ever drifts
// from the default is caught here). Auxiliary source half (carries the RED
// while the two constants still coincide): the call site passes no positive
// numeric literal — pass a non-positive cap so the default is single-sourced.
// ---------------------------------------------------------------------------

// fencedDigest returns the text inside the ```text fence that follows the
// "## Upstream Handoff Digest" heading of a tdd prompt, or "".
func fencedDigest(prompt string) string {
	_, after, ok := strings.Cut(prompt, "## Upstream Handoff Digest")
	if !ok {
		return ""
	}
	_, after, ok = strings.Cut(after, "```text\n")
	if !ok {
		return ""
	}
	block, _, ok := strings.Cut(after, "\n```")
	if !ok {
		return ""
	}
	return block
}

func TestC1632_010_TDDPromptDigestCapIsThePackageDefault(t *testing.T) {
	generic := map[string]any{}
	for i := 0; i < 64; i++ { // 64 × ~60 runes ≈ 3.8k runes rendered — past any fixed literal
		generic[fmt.Sprintf("scout.k%02d", i)] = strings.Repeat("v", 40)
	}
	req := core.PhaseRequest{Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
		Phase:    string(core.PhaseTDD),
		Upstream: phaseio.NewHandoffs(phaseio.HandoffsInit{Generic: generic}),
	})}
	if !req.Input.Active() {
		t.Fatalf("fixture PhaseInput inactive — the digest hook would not run")
	}
	prompt := tdd.New(tdd.Config{}).ComposePrompt("BODY", req)
	block := fencedDigest(prompt)
	if block == "" {
		t.Fatalf("tdd's real ComposePrompt rendered no fenced upstream digest block for an active PhaseInput")
	}
	want := req.Input.Upstream().UpstreamDigest(0)
	if block != want {
		t.Errorf("tdd's rendered digest (%d runes) != internal/phaseio's package-default rendering (%d runes) — the phase forks its own cap", len([]rune(block)), len([]rune(want)))
	}
	src, err := os.ReadFile(filepath.Join(sourceRoot(t), "go", "internal", "phases", "tdd", "tdd.go"))
	if err != nil {
		t.Fatalf("read tdd.go: %v", err)
	}
	if m := regexp.MustCompile(`UpstreamDigest\(\s*[1-9][0-9]*`).Find(src); m != nil {
		t.Errorf("tdd.go passes a bare numeric cap %q to UpstreamDigest — pass a non-positive cap so internal/phaseio's default is the single source (cycle-1604 dbd6c43a…/dd88cbdc…)", m)
	}
}

// ===========================================================================
// Audit round 2 (audit-repair, second rejection): the rebuild did the H1 file
// writes but the remainder's acceptance never said "per-edge", so 007/008
// stayed RED on the handed-off bytes — and build-report.md nevertheless
// claimed `./acs/cycle1632 PASS` while its `## Regression Slice` repeated
// round 1's `[acs suite] … (cycle=9 …)` line verbatim (the round-2 lane had 21
// this-cycle rows). The suite was never re-run after 007–010 landed; a
// `Status: PASS` rested on a stale receipt (claim-discrepancy).
//
//	AC11 (H1, round 2) every `[acs suite]` receipt in build-report.md is a
//	     fresh, green run over THIS tree's predicates            → TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound
//
// The wording half of round-2 H1 needs no new predicate: 007/008 already
// encode it and are RED on the current bytes (never weakened).
// ===========================================================================

// suiteReceiptRe mirrors, field for field, the line runACSSuite prints
// (cmd/evolve/cmd_acs.go) — the only receipt a build can paste for a suite run.
var suiteReceiptRe = regexp.MustCompile(`\[acs suite\] cycle=(\d+) verdict=(\w+) green=(\d+) red=(\d+) skip=(\d+) total=(\d+) \(cycle=(\d+) regression=(\d+) red-team=(\d+)\)`)

// declaredCyclePredicates lists this package's top-level TestC1632_ functions
// through the compiler (`go test -list` compiles the tagged package and runs
// nothing) — the same inventory acssuite's this_cycle_count is built from,
// minus subtests, which can only ADD rows. Lists by import path so the module
// resolves from the test binary's cwd like TestC1632_005 does.
func declaredCyclePredicates(t *testing.T) []string {
	t.Helper()
	const self = "github.com/mickeyyaya/evolve-loop/go/acs/cycle1632"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-tags", "acs", "-list", "^TestC1632_", self)
	if code != 0 {
		t.Fatalf("go test -list %s: code=%d err=%v\n%s%s", self, code, err, stdout, stderr)
	}
	var names []string
	for _, line := range strings.Split(stdout, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "TestC1632_") {
			names = append(names, line)
		}
	}
	if len(names) == 0 {
		t.Fatalf("go test -list found no TestC1632_ predicate in %s:\n%s", self, stdout)
	}
	return names
}

// ---------------------------------------------------------------------------
// AC11 (H1, round 2) — the landing report's suite receipt is a run over THESE
// predicates, and it is green. Every `[acs suite]` line the report carries must
// name this cycle, report verdict=PASS with red=0, and count at least as many
// this-cycle rows as there are TestC1632_ predicates declared on the tree: a
// receipt with fewer rows than declared tests cannot have been produced by
// running them (round 2 pasted cycle=9 against 10 declared). A report with no
// receipt at all, a red receipt, or a stale one left beside a fresh one is
// each RED — a `Status: PASS` may rest only on a suite run over this tree.
// ---------------------------------------------------------------------------

func TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound(t *testing.T) {
	runDir := cycleRunDir(t)
	body, err := os.ReadFile(filepath.Join(runDir, "build-report.md"))
	if err != nil {
		t.Fatalf("build-report.md unreadable in %s: %v — the build must produce the report the landing reads", runDir, err)
	}
	declared := declaredCyclePredicates(t)
	receipts := suiteReceiptRe.FindAllStringSubmatch(string(body), -1)
	if len(receipts) == 0 {
		t.Fatalf("build-report.md carries no `[acs suite] cycle=… (cycle=N …)` receipt — paste the line `evolve acs suite --cycle %d --root <worktree> --evolve-dir <runtime>/.evolve` prints for THIS tree", thisCycle)
	}
	for _, m := range receipts {
		cycle, _ := strconv.Atoi(m[1])
		verdict := m[2]
		red, _ := strconv.Atoi(m[4])
		rows, _ := strconv.Atoi(m[7])
		if cycle != thisCycle {
			t.Errorf("receipt %q names cycle %d, not %d — a foreign cycle's suite run cannot back this build", m[0], cycle, thisCycle)
		}
		if verdict != "PASS" || red != 0 {
			t.Errorf("receipt %q reports verdict=%s red=%d — a `Status: PASS` cannot rest on a red suite; fix the reds and re-run", m[0], verdict, red)
		}
		if rows < len(declared) {
			t.Errorf("stale receipt %q: %d this-cycle row(s), but %d TestC1632_ predicates are declared on this tree (%s) — the suite was not re-run after they landed (round-2 claim-discrepancy); replace it with the fresh line", m[0], rows, len(declared), strings.Join(declared, ", "))
		}
	}
}
