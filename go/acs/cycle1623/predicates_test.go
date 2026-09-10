//go:build acs

// Package cycle1623 materializes the acceptance criteria for cycle 1623's
// AUDIT-REPAIR round. The cycle's first round selected the inbox item
// `agy-tier-map-single-source`, but triage committed an EMPTY `top_n` (the
// atomic inbox claim failed), and the audit REJECTED the result. The audit's
// findings — not the unclaimed inbox item — are this round's scope.
//
// Why the agy predicates that used to live here are gone: triage-decision.json
// lists `agy-tier-map-single-source` under `deferred`, never `top_n`. R9.3
// binds predicates to triage-COMMITTED work only, and the host's floor-binding
// gate rejects a predicate that gates deferred work (cycle-280: predicates
// gating a deferred item starved the committed task). Those predicates were
// also the audit's own M1 finding — untracked RED residue that any later lane
// inheriting this worktree would inherit as a false regression. They are
// removed here, not weakened: the work they encoded is deferred intact and its
// contract is re-authored by the cycle that actually claims the item.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC-R1 (audit H1) sandbox denies the inbox-claim path  → manual+checklist (NOT a predicate; see below)
//	AC-R2 (audit H2) empty top_n must terminate the cycle → TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine
//	AC-R3 (audit H2) a committed top_n must still advance → TestC1623_002_CommittedTopNStillAdvancesTheSpine
//	AC-R4 (audit M2) claim failure must strand no item    → TestC1623_003_ClaimFailureLeavesInboxItemInPlace
//	AC-R5 (audit M1) this cycle's ACS package is tracked  → TestC1623_004_CycleACSPackageIsGitTracked
//
// AC-R1 carries NO predicate on purpose. The audit located H1's fix at
// go/internal/bridge/sandbox_wrap.go:209 (sandboxWritePaths omits the
// project-root inbox from the write allowlist). Both that file and
// go/internal/adapters/sandbox/ are entries in
// guards.ProtectedSurfaceManifest — verified live, not inferred:
// guards.IsProtectedSurface("go/internal/bridge/sandbox_wrap.go") == true,
// which internal/guards/role.go:62 denies at write time and
// internal/phases/ship/integrity.go:30 blocks at ship. A lane CANNOT land that
// edit. Freezing a doNotModifyTests predicate against it would be exactly the
// cycle-644 shape: an acceptance criterion that is unsatisfiable by
// construction, burning the whole cycle. It is dispositioned
// manual+checklist and routed to the console owner instead.
//
// Adversarial axes (skills/adversarial-testing §6):
//   - NEGATIVE — TestC1623_002 is the anti-no-op: a "fix" that simply always
//     terminates after triage passes 001 and FAILS 002. The gate must key on
//     the committed count, not on the phase.
//   - EDGE — TestC1623_003 drives the permission-denied branch (the exact
//     failure mode that blanked this cycle), not the happy path.
//   - SEMANTIC — 001 pins the routing DECISION, 003 pins inbox ownership
//     ATOMICITY, 004 pins ship-tree TRACKING: three distinct behaviors.
//
// No grep-only predicates (the cycle-85 ban): 001/002 drive the real exported
// router.Digest → router.Route composition over a real on-disk workspace (the
// composed dispatch path, per lesson inst-L1563a — never a hand-built signal
// literal); 003 calls the real exported inboxmover.Claim and asserts on the
// filesystem side effect; 004 asserts on `git ls-files` exit status.
package cycle1623

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// fixedNow keeps Route deterministic — no wall-clock dependence (the
// flaky-predicate-shape ban on time-derived bounds).
var fixedNow = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

// spineConfig is a production-shaped RoutingConfig whose Mandatory spine
// INCLUDES build. That is deliberate: cycle 1623 routed `spine:build` off an
// empty top_n (routing-decision-4.json), so the contract under test is that
// the empty-commitment gate outranks the mandatory spine. Mirrors the
// defaults internal/routingtest.buildConfig fills in.
func spineConfig() config.RoutingConfig {
	return config.RoutingConfig{
		Stage:         config.StageAdvisory,
		Mode:          config.ModeDynamicLLM,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		Conditional:   map[string]config.CondRule{"tdd": config.DefaultTddRule()},
		PhaseEnable:   map[string]config.Enable{},
		Triggers:      map[string]config.RoutingBlock{},
		MaxInsertions: 4,
	}
}

// triagedWorkspace materializes a real cycle workspace on disk whose triage
// decision commits exactly the given task ids, then returns the routing
// signals the production digest derives from it. Writing the artifacts and
// reading them back through router.Digest is the wiring proof: it fails if the
// committed-count signal is never plumbed from triage-decision.json, which is
// the actual H2 defect.
func triagedWorkspace(t *testing.T, committedIDs ...string) (router.RoutingSignals, string) {
	t.Helper()
	ws := t.TempDir()

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("triage-decision.json", triageDecisionJSON(committedIDs...))
	write("handoff-triage.json", `{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`)
	write("handoff-scout.json", `{"cycle_size_estimate":"small","deliverable_kind":"code","backlog_size":1}`)

	sig, err := router.Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("router.Digest(%s): %v", ws, err)
	}
	return sig, ws
}

// routeAfterTriage runs the real pure kernel for the triage→next-phase edge —
// the exact transition that produced routing-decision-3/4.json in cycle 1623.
func routeAfterTriage(sig router.RoutingSignals) router.RouterDecision {
	return router.Route(router.RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Signals:   sig,
		Cfg:       spineConfig(),
		Completed: []string{"scout", "triage"},
		Now:       fixedNow,
	}, nil)
}

// TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine encodes audit
// finding H2. Triage committed `"top_n": []` at 21:21; the router nonetheless
// returned tdd, then build, then audit, burning two full phases plus an audit
// against a task no phase was authorized to own. An empty commitment must
// terminate the cycle at the triage edge.
func TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine(t *testing.T) {
	sig, ws := triagedWorkspace(t)

	// Wiring half: the committed count must actually reach the router from
	// triage-decision.json. Without this the gate below can never fire in
	// production no matter how it is written.
	if !sig.Triage.Present {
		t.Fatalf("RED: router.Digest(%s) produced no triage signals — triage-decision.json/handoff-triage.json are on disk", ws)
	}
	if got := sig.Triage.CommittedCount; got != 0 {
		t.Errorf("RED: Triage.CommittedCount = %d, want 0 — the empty top_n in triage-decision.json is not plumbed into the routing signals", got)
	}

	// Behavior half: the decision itself.
	d := routeAfterTriage(sig)
	switch d.NextPhase {
	case "tdd", "build", "build-planner", "tester", "audit":
		t.Errorf("RED: Route after triage returned next_phase=%q (reason %q) on an empty top_n — cycle 1623 burned tdd+build+audit exactly this way; an empty commitment must terminate", d.NextPhase, d.Reason)
	case router.PhaseEnd:
		// contract satisfied
	default:
		t.Errorf("RED: Route after triage returned next_phase=%q (reason %q), want %q on an empty top_n", d.NextPhase, d.Reason, router.PhaseEnd)
	}
}

// TestC1623_002_CommittedTopNStillAdvancesTheSpine is the NEGATIVE control for
// TestC1623_001 — the anti-no-op. A gate that terminates on every post-triage
// transition would satisfy 001 while bricking every productive cycle. With one
// committed task the spine MUST still advance.
func TestC1623_002_CommittedTopNStillAdvancesTheSpine(t *testing.T) {
	sig, ws := triagedWorkspace(t, "agy-tier-map-single-source")

	if got := sig.Triage.CommittedCount; got != 1 {
		t.Errorf("RED: Triage.CommittedCount = %d, want 1 — router.Digest(%s) is not counting triage-decision.json top_n entries", got, ws)
	}

	d := routeAfterTriage(sig)
	if d.NextPhase == router.PhaseEnd {
		t.Errorf("RED: Route after triage terminated (reason %q) even though top_n commits 1 task — the empty-commitment gate must key on the committed count, not on the triage phase itself", d.Reason)
	}
}

// TestC1623_003_ClaimFailureLeavesInboxItemInPlace encodes audit finding M2.
// inboxmover.Claim mkdirs `<inbox>/processing/cycle-N` and only then renames
// the item in. When the mkdir fails — the exact sandbox denial that blanked
// cycle 1623's top_n — the item must remain in the inbox root, claimable by a
// later cycle. Today that holds only by statement ordering
// (inboxmover.go:209-215) and nothing asserts it, so a future reordering could
// strand the item silently.
func TestC1623_003_ClaimFailureLeavesInboxItemInPlace(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not deny mkdir, so the failure branch is unreachable")
	}
	projectRoot := t.TempDir()
	inboxDir := filepath.Join(projectRoot, ".evolve", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	item := filepath.Join(inboxDir, "2026-09-09T00-10-00Z-probe-claim-atomicity.json")
	if err := os.WriteFile(item, []byte(`{"id":"probe-claim-atomicity","priority":"H","files":["go/internal/nowhere/x.go"]}`), 0o644); err != nil {
		t.Fatalf("write inbox item: %v", err)
	}

	// Deny the processing-dir creation the way the phase sandbox does.
	if err := os.Chmod(inboxDir, 0o555); err != nil {
		t.Fatalf("chmod inbox read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(inboxDir, 0o755) })

	_, err := inboxmover.Claim(inboxmover.Options{
		ProjectRoot: projectRoot,
		InboxDir:    inboxDir,
		Stderr:      io.Discard,
	}, "probe-claim-atomicity", "1623")
	if err == nil {
		t.Fatalf("RED: Claim succeeded against a read-only inbox dir — the mkdir denial was not surfaced")
	}
	if _, statErr := os.Stat(item); statErr != nil {
		t.Errorf("RED: claim failed (%v) AND stranded the item — %s is gone; a failed claim must leave the inbox item in place for a later cycle", err, item)
	}
}

// TestC1623_004_CycleACSPackageIsGitTracked encodes audit finding M1 and the
// audit gate's own stated reason ("predicate execution tree includes
// undeclared inputs absent from the ship tree"). Cycle 1623's predicates were
// left UNTRACKED, so the audit's predicate tree and the ship tree disagreed.
// Disk presence alone is not enough — the cycle-93 lesson: an untracked file
// is silently dropped at ship.
func TestC1623_004_CycleACSPackageIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "acs", "cycle1623", "predicates_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
		t.Errorf("RED: %s is untracked (git ls-files exit=%d err=%v) — the audit's predicate tree would carry an input absent from the ship tree", rel, code, err)
	}
}

// ---------------------------------------------------------------------------
// AUDIT-REPAIR ROUND 3 — audit round 2 finding H1 (CRITICAL).
//
// Round 2's gate was written and graded at router.Route. Route's decision is
// only a PROPOSAL: go/internal/core/cyclerun_select.go:103 routes it through
// Orchestrator.enforceNext, whose PhaseEnd branch
// (go/internal/core/routing_dispatch.go:59-63) asks
// StateMachine.CanTerminateEarly(current, shipPlanned) — and that returns false
// unconditionally when shipPlanned is true
// (go/internal/core/statemachine.go:230-233). Cycle 1623's own clamped plan
// schedules ship, so the proposal was dropped and the orchestrator dispatched
// tdd anyway. TestC1623_001 stayed GREEN through the whole defect because it
// asserts one layer ABOVE the authority that decides the next phase.
//
// The three predicates below re-grade that criterion where the decision is
// CONSUMED, not where it is produced: they drive a real core.Orchestrator
// through a real RunCycle with routing at the live advisory stage and no
// clamped plan (planRunsShip(nil) == true — the exact ship-planned
// configuration that made the round-2 gate inert) and assert on the phases the
// orchestrator ACTUALLY DISPATCHED. A fix that only changes router.Route
// leaves 005 RED.
// ---------------------------------------------------------------------------

// dispatchLog records, in order, the phases a real cycle actually dispatched.
// The orchestrator may run phases concurrently (parallel evaluate), so the
// mutex is load-bearing, not decoration.
type dispatchLog struct {
	mu    sync.Mutex
	order []string
}

func (l *dispatchLog) record(phase string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.order = append(l.order, phase)
}

func (l *dispatchLog) ran(phase string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, p := range l.order {
		if p == phase {
			return true
		}
	}
	return false
}

func (l *dispatchLog) dispatched() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.order...)
}

// recordingRunner is a no-LLM core.PhaseRunner that records its own dispatch
// and, for the phases that own an artifact this test depends on, materializes
// that artifact into the cycle workspace exactly where the production phase
// writes it — so router.Digest reads a real on-disk handoff, not a literal.
type recordingRunner struct {
	phase string
	log   *dispatchLog
	onRun func(core.PhaseRequest) error
	inner core.PhaseRunner
}

func (r *recordingRunner) Name() string { return r.phase }

func (r *recordingRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.log.record(r.phase)
	if r.onRun != nil {
		if err := r.onRun(req); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	// Delegate to the canonical fixture runner so the explanation-documentation
	// contract (build-report.md) is satisfied the same way every other
	// orchestrator test satisfies it — no bespoke second implementation.
	return r.inner.Run(ctx, req)
}

// triageDecisionJSON renders a triage-decision.json committing exactly the
// given task ids in top_n. No ids ⇒ the explicit empty commitment that blanked
// cycle 1623.
func triageDecisionJSON(committedIDs ...string) string {
	topN := "["
	for i, id := range committedIDs {
		if i > 0 {
			topN += ","
		}
		topN += `{"id":"` + id + `"}`
	}
	topN += "]"
	return `{"cycle":1623,"top_n":` + topN + `,"deferred":[{"id":"agy-tier-map-single-source"}],"dropped":[]}`
}

// writeWorkspaceFile writes one artifact into the cycle workspace the
// orchestrator handed the phase.
func writeWorkspaceFile(req core.PhaseRequest, name, body string) error {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(req.Workspace, name), []byte(body), 0o644)
}

// commitTriage returns the triage runner hook that commits the given ids. A
// nil hook (see TestC1623_007) writes NOTHING — the commitment-unknown case.
func commitTriage(committedIDs ...string) func(core.PhaseRequest) error {
	return func(req core.PhaseRequest) error {
		if err := writeWorkspaceFile(req, "triage-decision.json", triageDecisionJSON(committedIDs...)); err != nil {
			return err
		}
		return writeWorkspaceFile(req, "handoff-triage.json",
			`{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`)
	}
}

// composedRoutingConfig is the LIVE dispatch configuration: routing at the
// advisory stage (the production default since 2026-06-06), the full ordered
// spine, and triage force-enabled the way the phase registry enables it in
// production. Derived from spineConfig so the pure-kernel predicates (001/002)
// and the composed ones cannot drift apart on the mandatory set.
func composedRoutingConfig() config.RoutingConfig {
	cfg := spineConfig()
	cfg.Order = []string{"scout", "triage", "tdd", "build", "audit", "ship"}
	cfg.PhaseEnable = map[string]config.Enable{"triage": config.EnableOn}
	return cfg
}

// initCycleRepo materializes a real git repository for the cycle's
// ProjectRoot. The orchestrator provisions the cycle worktree with
// `git worktree add` and seals the Build explanation against the base SHA, so a
// bare temp dir degrades the run before it reaches the triage→next edge under
// test. Every git invocation is -C scoped (never cwd-relative) and the identity
// is supplied by env, so the check is independent of the operator's git config.
func initCycleRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	runGit("add", ".gitignore")
	runGit("commit", "-m", "base")
	return repo
}

// runComposedCycle drives the REAL orchestrator — core.NewOrchestrator +
// RunCycle — over the real routing kernel (router.StaticPreset, whose Decide is
// the same pure Route the production loop calls). No planner is wired, so
// clampedPlan is nil and planRunsShip reports SHIP PLANNED: the configuration
// under which the round-2 gate was proven inert.
//
// It returns the dispatch log AND RunCycle's error rather than failing on the
// error, deliberately. The contract under test is the triage→next DISPATCH
// DECISION, which is settled before the epilogue: in the control cases the
// no-LLM build runner cannot satisfy the real explanation-documentation floor,
// so RunCycle legitimately ends in a build-floor rejection AFTER the phases
// under test have already been dispatched. Every assertion below quotes both
// the dispatch log and this error, and each test guards against a vacuous pass
// by requiring the cycle to have reached triage at all — so an early abort can
// never be mistaken for a satisfied contract.
func runComposedCycle(t *testing.T, triageHook func(core.PhaseRequest) error) (*dispatchLog, error) {
	t.Helper()

	log := &dispatchLog{}
	runners := map[core.Phase]core.PhaseRunner{}
	for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage,
		core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit,
		core.PhaseShip, core.PhaseRetro} {
		r := &recordingRunner{
			phase: string(p),
			log:   log,
			inner: &fixtures.FakeRunner{PhaseName: string(p)},
		}
		switch p {
		case core.PhaseScout:
			r.onRun = func(req core.PhaseRequest) error {
				return writeWorkspaceFile(req, "handoff-scout.json",
					`{"cycle_size_estimate":"small","deliverable_kind":"code","backlog_size":1}`)
			}
		case core.PhaseTriage:
			r.onRun = triageHook
		}
		runners[p] = r
	}

	o := core.NewOrchestrator(
		&fixtures.FakeStorage{State: core.State{LastCycleNumber: 1622}},
		&fixtures.FakeLedger{},
		runners,
		core.WithRouting(composedRoutingConfig(), router.StaticPreset{}),
	)
	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: initCycleRepo(t),
		GoalHash:    "14d8eae2",
	})
	return log, err
}

// reachedTriage fails the test when the cycle never got as far as the
// transition under test — the guard that stops an early abort from reading as a
// satisfied contract.
func reachedTriage(t *testing.T, log *dispatchLog, err error) {
	t.Helper()
	if !log.ran("scout") || !log.ran("triage") {
		t.Fatalf("fixture invalid: the cycle never reached the triage→next edge — dispatched=%v, RunCycle err=%v",
			log.dispatched(), err)
	}
}

// TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath re-grades
// audit finding H2 at the layer that DECIDES the next phase. Round 2's fix
// stops at router.Route; enforceNext discards its PhaseEnd whenever ship is
// planned, so the orchestrator still dispatches tdd. This drives the composed
// path end to end and asserts on what was actually dispatched.
func TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath(t *testing.T) {
	log, err := runComposedCycle(t, commitTriage())
	reachedTriage(t, log, err)

	for _, phase := range []string{"tdd", "build", "audit", "ship"} {
		if log.ran(phase) {
			t.Errorf("RED: the composed dispatch path ran %q after an explicit empty top_n — dispatched=%v (RunCycle err=%v). router.Route proposes PhaseEnd, but core.enforceNext drops it (CanTerminateEarly(triage, shipPlanned=true)=false), so the gate is inert exactly where the cycle is decided",
				phase, log.dispatched(), err)
		}
	}
}

// TestC1623_006_CommittedTopNStillDispatchesTheSpine is the NEGATIVE control
// for 005 on the SAME composed harness — the anti-no-op. A "fix" that
// terminates every post-triage transition, or that hard-codes
// CanTerminateEarly to true, satisfies 005 and bricks every productive cycle.
// With one committed task the orchestrator MUST still dispatch the spine.
func TestC1623_006_CommittedTopNStillDispatchesTheSpine(t *testing.T) {
	log, err := runComposedCycle(t, commitTriage("agy-tier-map-single-source"))
	reachedTriage(t, log, err)

	for _, phase := range []string{"tdd", "build"} {
		if !log.ran(phase) {
			t.Errorf("RED: the composed dispatch path skipped %q even though top_n commits 1 task — dispatched=%v (RunCycle err=%v). The early-exit gate must key on the KNOWN-EMPTY commitment, never on the triage phase itself",
				phase, log.dispatched(), err)
		}
	}
}

// TestC1623_007_ShipPlannedEarlyExitStaysBlockedWithoutAKnownEmptyCommitment
// pins the kernel invariant the fix must NOT trade away. Two axes:
//
//   - The 2-arg authority StateMachine.CanTerminateEarly keeps its documented
//     contract — a ship-intended cycle can never terminate early on it alone.
//     internal/core/extra_coverage_test.go:45 already pins the same behavior
//     through enforceNext ("early-exit-blocked-when-ship"); that test must stay
//     GREEN and unmodified, so the empty-commitment authority has to be
//     ADDITIVE, not a rewrite of this method's meaning.
//   - Fail-open survives: a workspace with NO triage-decision.json has an
//     UNKNOWN commitment (commitmentKnown=false), which must never be read as
//     an empty one. The composed cycle must advance normally.
func TestC1623_007_ShipPlannedEarlyExitStaysBlockedWithoutAKnownEmptyCommitment(t *testing.T) {
	sm := core.NewStateMachine()
	if sm.CanTerminateEarly(core.PhaseTriage, true) {
		t.Errorf("RED: CanTerminateEarly(triage, shipPlanned=true) = true — the ship-intended cycle invariant was traded away instead of adding a commitment-aware authority beside it; internal/core/extra_coverage_test.go:45 pins this edge")
	}
	if !sm.CanTerminateEarly(core.PhaseTriage, false) {
		t.Errorf("RED: CanTerminateEarly(triage, shipPlanned=false) = false — the pre-existing no-ship early exit was broken")
	}

	// Commitment UNKNOWN (no triage-decision.json at all) — must not be
	// mistaken for an empty commitment.
	log, err := runComposedCycle(t, func(req core.PhaseRequest) error {
		return writeWorkspaceFile(req, "handoff-triage.json",
			`{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`)
	})
	reachedTriage(t, log, err)
	if !log.ran("build") {
		t.Errorf("RED: the composed cycle terminated with NO triage-decision.json on disk — dispatched=%v (RunCycle err=%v). An unknown commitment must fail OPEN; only an explicit empty top_n terminates",
			log.dispatched(), err)
	}
}
