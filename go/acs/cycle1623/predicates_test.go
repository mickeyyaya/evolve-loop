//go:build acs

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

var fixedNow = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

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

func TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine(t *testing.T) {
	sig, ws := triagedWorkspace(t)

	if !sig.Triage.Present {
		t.Fatalf("RED: router.Digest(%s) produced no triage signals — triage-decision.json/handoff-triage.json are on disk", ws)
	}
	if got := sig.Triage.CommittedCount; got != 0 {
		t.Errorf("RED: Triage.CommittedCount = %d, want 0 — the empty top_n in triage-decision.json is not plumbed into the routing signals", got)
	}

	d := routeAfterTriage(sig)
	switch d.NextPhase {
	case "tdd", "build", "build-planner", "tester", "audit":
		t.Errorf("RED: Route after triage returned next_phase=%q (reason %q) on an empty top_n — cycle 1623 burned tdd+build+audit exactly this way; an empty commitment must terminate", d.NextPhase, d.Reason)
	case router.PhaseEnd:
	default:
		t.Errorf("RED: Route after triage returned next_phase=%q (reason %q), want %q on an empty top_n", d.NextPhase, d.Reason, router.PhaseEnd)
	}
}

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
	return r.inner.Run(ctx, req)
}

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

func writeWorkspaceFile(req core.PhaseRequest, name, body string) error {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(req.Workspace, name), []byte(body), 0o644)
}

func commitTriage(committedIDs ...string) func(core.PhaseRequest) error {
	return func(req core.PhaseRequest) error {
		if err := writeWorkspaceFile(req, "triage-decision.json", triageDecisionJSON(committedIDs...)); err != nil {
			return err
		}
		return writeWorkspaceFile(req, "handoff-triage.json",
			`{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`)
	}
}

func composedRoutingConfig() config.RoutingConfig {
	cfg := spineConfig()
	cfg.Order = []string{"scout", "triage", "tdd", "build", "audit", "ship"}
	cfg.PhaseEnable = map[string]config.Enable{"triage": config.EnableOn}
	return cfg
}

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

func reachedTriage(t *testing.T, log *dispatchLog, err error) {
	t.Helper()
	if !log.ran("scout") || !log.ran("triage") {
		t.Fatalf("fixture invalid: the cycle never reached the triage→next edge — dispatched=%v, RunCycle err=%v",
			log.dispatched(), err)
	}
}

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

func TestC1623_007_ShipPlannedEarlyExitStaysBlockedWithoutAKnownEmptyCommitment(t *testing.T) {
	sm := core.NewStateMachine()
	if sm.CanTerminateEarly(core.PhaseTriage, true) {
		t.Errorf("RED: CanTerminateEarly(triage, shipPlanned=true) = true — the ship-intended cycle invariant was traded away instead of adding a commitment-aware authority beside it; internal/core/extra_coverage_test.go:45 pins this edge")
	}
	if !sm.CanTerminateEarly(core.PhaseTriage, false) {
		t.Errorf("RED: CanTerminateEarly(triage, shipPlanned=false) = false — the pre-existing no-ship early exit was broken")
	}

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
