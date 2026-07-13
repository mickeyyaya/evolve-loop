package core_test

// Chronicle S3 RED contract (cycle-784, chronicle-s3-digest-wiring, task
// seed-digest-at-cycle-start). newCycleRun must resolve the chronicle policy
// ONCE and, per stage:
//
//   off     → write nothing, inject nothing (byte-identical cycle start).
//   shadow  → assemble DigestInput (last-N dossiers from
//             knowledge-base/cycles/cycle-*.json, entriesFromRecords(state.FailedAt),
//             best-effort recurrence ledger) and WriteDigest into the run
//             workspace — but do NOT inject Context["recent_outcomes"].
//   enforce → same write, PLUS Context["recent_outcomes"] carries the digest
//             bytes into every phase request (scout/triage render it).
//
// WriteDigest is best-effort: a digest failure logs a WARN to stderr and the
// cycle proceeds (mirrors the archivePollutedWorkspace idiom two blocks away).
//
// API pin (mirrors WithRetryConfig/WithWorkflowConfig, orchestrator.go): the
// resolved policy.ChronicleConfig is injected at the composition root via
// core.WithChronicleConfig; the zero-option default is the compiled default
// (shadow). Builder implements; must NOT modify these tests.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// ctxCaptureRunner is a PASS runner that records every PhaseRequest it
// receives so tests can assert on the Context a phase actually observed.
type ctxCaptureRunner struct {
	name string
	mu   sync.Mutex
	reqs []core.PhaseRequest
}

func (r *ctxCaptureRunner) Name() string { return r.name }
func (r *ctxCaptureRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.mu.Lock()
	r.reqs = append(r.reqs, req)
	r.mu.Unlock()
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func (r *ctxCaptureRunner) lastContext(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reqs) == 0 {
		return ""
	}
	return r.reqs[len(r.reqs)-1].Context[key]
}

// seedChronicleDossier writes a minimal dossier JSON where the digest
// assembler must find it: <projectRoot>/knowledge-base/cycles/cycle-<n>.json.
func seedChronicleDossier(t *testing.T, projectRoot string, cycle int, goal, verdict string) {
	t.Helper()
	dir := filepath.Join(projectRoot, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir dossier dir: %v", err)
	}
	doc := fmt.Sprintf(`{"cycle": %d, "goal": %q, "final_verdict": %q, "phases": []}`, cycle, goal, verdict)
	path := filepath.Join(dir, fmt.Sprintf("cycle-%d.json", cycle))
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("write dossier %s: %v", path, err)
	}
}

// runChronicleCycle runs one full cycle against projectRoot with a
// context-capturing scout and returns (result, error, scoutCapture).
func runChronicleCycle(t *testing.T, projectRoot string, disableGuard bool, opts ...core.Option) (core.CycleResult, error, *ctxCaptureRunner) {
	t.Helper()
	scout := &ctxCaptureRunner{name: string(core.PhaseScout)}
	orch, _, _ := newTestOrchestratorOpts(t, newRunners(map[core.Phase]core.PhaseRunner{core.PhaseScout: scout}), opts...)
	res, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot:           projectRoot,
		GoalHash:              "chronicle-s3-test-goal",
		Context:               map[string]string{"commit_message": "test commit"},
		DisableWorkspaceGuard: disableGuard,
	})
	return res, err, scout
}

// newTestOrchestratorOpts mirrors newTestOrchestrator but forwards Options.
func newTestOrchestratorOpts(t *testing.T, runners map[core.Phase]core.PhaseRunner, opts ...core.Option) (*core.Orchestrator, *recStorage, *fakeLedger) {
	t.Helper()
	st := &recStorage{}
	ld := &fakeLedger{}
	return core.NewOrchestrator(st, ld, runners, opts...), st, ld
}

func chronicleDigestPath(projectRoot string, cycle int) string {
	return filepath.Join(core.RunWorkspacePath(projectRoot, cycle), "recent-outcomes.md")
}

// Shadow (the compiled default — NO option passed): the digest artifact is
// seeded into the run workspace at cycle start from the dossier history, but
// the context key is NOT injected (today's prompt bytes preserved).
func TestNewCycleRun_SeedsRecentOutcomesDigestAtShadow(t *testing.T) {
	root := t.TempDir()
	seedChronicleDossier(t, root, 42, "harden the flux capacitor", "PASS")

	res, err, scout := runChronicleCycle(t, root, false)
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	data, rerr := os.ReadFile(chronicleDigestPath(root, res.Cycle))
	if rerr != nil {
		t.Fatalf("shadow stage must seed recent-outcomes.md into the run workspace at cycle start: %v", rerr)
	}
	if !strings.Contains(string(data), "cycle 42") || !strings.Contains(string(data), "harden the flux capacitor") {
		t.Errorf("digest does not render the seeded dossier (want cycle 42 + goal line):\n%s", data)
	}
	if got := scout.lastContext("recent_outcomes"); got != "" {
		t.Errorf("shadow stage must NOT inject Context[\"recent_outcomes\"], got %q", got)
	}
}

// Off: nothing is written and nothing is injected — a cycle start
// byte-identical to the pre-chronicle behavior.
func TestNewCycleRun_OffStageWritesNoDigest(t *testing.T) {
	root := t.TempDir()
	seedChronicleDossier(t, root, 42, "harden the flux capacitor", "PASS")

	res, err, scout := runChronicleCycle(t, root, false,
		core.WithChronicleConfig(policy.ChronicleConfig{Digest: "off"}))
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if _, serr := os.Stat(chronicleDigestPath(root, res.Cycle)); serr == nil {
		t.Errorf("off stage must not write recent-outcomes.md")
	}
	if got := scout.lastContext("recent_outcomes"); got != "" {
		t.Errorf("off stage must NOT inject Context[\"recent_outcomes\"], got %q", got)
	}
}

// Enforce: the digest is written AND its content is injected once at the
// cycle-start resolution point, so every downstream phase request carries
// Context["recent_outcomes"].
func TestNewCycleRun_EnforceInjectsRecentOutcomesContext(t *testing.T) {
	root := t.TempDir()
	seedChronicleDossier(t, root, 42, "harden the flux capacitor", "PASS")

	res, err, scout := runChronicleCycle(t, root, false,
		core.WithChronicleConfig(policy.ChronicleConfig{Digest: "enforce", DigestTokens: 1200, DigestCycles: 10}))
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if _, serr := os.Stat(chronicleDigestPath(root, res.Cycle)); serr != nil {
		t.Fatalf("enforce stage must still write recent-outcomes.md: %v", serr)
	}
	got := scout.lastContext("recent_outcomes")
	if got == "" {
		t.Fatalf("enforce stage must inject Context[\"recent_outcomes\"] into phase requests")
	}
	if !strings.Contains(got, "cycle 42") {
		t.Errorf("injected recent_outcomes does not carry the digest content, got %q", got)
	}
}

// Digest failure is best-effort: with the digest target pre-created as a
// DIRECTORY the atomic rename inside WriteDigest fails — the cycle must still
// complete PASS and a WARN naming the digest must reach stderr (the
// archivePollutedWorkspace WARN idiom). No t.Parallel(): os.Stderr is swapped.
func TestNewCycleRun_DigestFailureWarnsNotAborts(t *testing.T) {
	root := t.TempDir()
	seedChronicleDossier(t, root, 42, "harden the flux capacitor", "PASS")
	// Fresh state → allocateCycle mints cycle 1; pre-create the collision there.
	ws := core.RunWorkspacePath(root, 1)
	if err := os.MkdirAll(filepath.Join(ws, "recent-outcomes.md"), 0o755); err != nil {
		t.Fatalf("pre-create digest collision dir: %v", err)
	}

	var res core.CycleResult
	var runErr error
	stderr := captureStderr(t, func() {
		res, runErr, _ = runChronicleCycle(t, root, true) // guard disabled: workspace is pre-seeded
	})
	if runErr != nil {
		t.Fatalf("digest failure must never abort the cycle, got: %v", runErr)
	}
	if res.Cycle != 1 {
		t.Fatalf("harness assumption broken: fresh state minted cycle %d, want 1 (collision seeded at cycle-1)", res.Cycle)
	}
	if res.FinalVerdict != core.VerdictPASS {
		t.Errorf("cycle verdict = %q, want PASS despite digest failure", res.FinalVerdict)
	}
	if !strings.Contains(stderr, "WARN") || !strings.Contains(stderr, "digest") {
		t.Errorf("digest failure must WARN loudly on stderr (want WARN + \"digest\"), got:\n%s", stderr)
	}
}

// captureStderr swaps os.Stderr for a pipe around fn and returns what was
// written. Callers must not run in parallel.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() {
		os.Stderr = old
	}()
	fn()
	_ = w.Close()
	os.Stderr = old
	return <-done
}
