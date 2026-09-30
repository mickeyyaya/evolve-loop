//go:build acs

package cycle1495

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func TestC1495_001_CleanWorktreeProducesNoShadowReuseMatch(t *testing.T) {
	repo := initProbeRepo(t)
	treeSHA := stageAndWriteTree(t, repo)
	seedVerdictCache(t, repo, treeSHA)

	obs, runners := runProbeCycle(t, repo)
	if obs.calls != 1 {
		t.Fatalf("verdict-cache probe fired %d time(s), want exactly 1 — the pre-loop probe did not execute", obs.calls)
	}
	if obs.sha != treeSHA {
		t.Fatalf("probe candidate sha = %q, want the worktree content identity %q", obs.sha, treeSHA)
	}
	if !obs.skipped {
		t.Errorf("clean worktree at the base tree was NOT suppressed (skipped=false) — fresh-base collision is live")
	}
	if obs.matched {
		t.Errorf("clean worktree produced a shadow reuse MATCH against a sibling-lane base entry — this is the cycle-1495 defect")
	}
	assertPhasesRan(t, runners)
}

func TestC1495_002_ChangedWorktreeKeepsCacheEligibility(t *testing.T) {
	repo := initProbeRepo(t)
	writeFile(t, filepath.Join(repo, "changed.txt"), "a real delta")
	treeSHA := stageAndWriteTree(t, repo)
	if treeSHA == baseTreeSHA(t, repo) {
		t.Fatalf("fixture bug: changed worktree tree %q equals base tree", treeSHA)
	}
	seedVerdictCache(t, repo, treeSHA)

	obs, runners := runProbeCycle(t, repo)
	if obs.calls != 1 {
		t.Fatalf("verdict-cache probe fired %d time(s), want exactly 1", obs.calls)
	}
	if obs.sha != treeSHA {
		t.Fatalf("probe candidate sha = %q, want %q", obs.sha, treeSHA)
	}
	if obs.skipped {
		t.Errorf("changed worktree was suppressed — the guard over-fires and disables real reuse")
	}
	if !obs.matched {
		t.Errorf("changed worktree with a seeded entry did NOT match — the normal cache path is broken")
	}
	assertPhasesRan(t, runners)
}

func TestC1495_003_NoDiffAuditDoesNotSeedCacheEntry(t *testing.T) {
	repo := initProbeRepo(t)
	treeSHA := stageAndWriteTree(t, repo)
	if treeSHA != baseTreeSHA(t, repo) {
		t.Fatalf("fixture bug: clean worktree tree %q != base tree %q", treeSHA, baseTreeSHA(t, repo))
	}

	_, _, led := runBindingCycle(t, repo)
	bound := auditBindingTreeSHA(t, led)
	if bound != treeSHA {
		t.Fatalf("audit binding worktree_tree_sha = %q, want %q — the binding did not run, so the cache assertion below would be vacuous", bound, treeSHA)
	}

	if e, ok := verdictcache.NewStore(repo, nil).Lookup(treeSHA); ok {
		t.Errorf("no-diff audit SEEDED a verdict-cache entry at the fresh-base tree %s (cycle=%d verdict=%s) — every sibling lane at this base will now false-match",
			treeSHA, e.Cycle, e.Verdict)
	}
}

func TestC1495_004_ChangedAuditStillRecordsBoundTreeIdentity(t *testing.T) {
	repo := initProbeRepo(t)
	writeFile(t, filepath.Join(repo, "changed.txt"), "a real delta")
	treeSHA := stageAndWriteTree(t, repo)

	_, _, led := runBindingCycle(t, repo)
	bound := auditBindingTreeSHA(t, led)
	if bound != treeSHA {
		t.Fatalf("audit binding worktree_tree_sha = %q, want the changed worktree tree %q", bound, treeSHA)
	}

	e, ok := verdictcache.NewStore(repo, nil).Lookup(bound)
	if !ok {
		t.Fatalf("changed audit did NOT record a verdict-cache entry at the bound tree %s — the guard suppressed real reuse (blanket disable)", bound)
	}
	if e.TreeSHA != bound {
		t.Errorf("cache entry TreeSHA = %q, want the audit-bound identity %q — producer and consumer keys have drifted", e.TreeSHA, bound)
	}
	if e.Verdict == "" {
		t.Errorf("cache entry recorded with an empty verdict — the projection carries no reusable result")
	}
}

func TestC1495_005_ProbeEligibleSharedPredicateEdges(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		candidate string
		want      bool
	}{
		{"empty candidate has no content identity", "base-tree", "", false},
		{"empty candidate with empty base", "", "", false},
		{"candidate equal to resolved base is a fresh worktree", "base-tree", "base-tree", false},
		{"candidate differing from base is real work", "base-tree", "changed-tree", true},
		{"unresolvable base leaves the candidate eligible", "", "changed-tree", true},
	}
	for _, tc := range cases {
		if got := verdictcache.ProbeEligible(tc.base, tc.candidate); got != tc.want {
			t.Errorf("%s: ProbeEligible(base=%q, candidate=%q) = %t, want %t", tc.name, tc.base, tc.candidate, got, tc.want)
		}
	}
}

const probeCycle = 1495

type probeObservation struct {
	calls   int
	sha     string
	skipped bool
	matched bool
}

func runProbeCycle(t *testing.T, repo string) (probeObservation, map[core.Phase]core.PhaseRunner) {
	t.Helper()
	var obs probeObservation
	runners := buildProbeRunners()
	o := core.NewOrchestrator(
		&probeStorage{state: core.State{LastCycleNumber: probeCycle - 1}},
		&probeLedger{},
		runners,
		core.WithWorktreeProvisioner(fixedWorktree{dir: repo}),
		core.WithVerdictCacheLookupHook(func(sha string, skipped, matched bool, _ verdictcache.Entry) {
			obs.calls++
			obs.sha, obs.skipped, obs.matched = sha, skipped, matched
		}),
	)
	if _, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: repo, GoalHash: "acs-1495"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return obs, runners
}

func runBindingCycle(t *testing.T, repo string) (probeObservation, map[core.Phase]core.PhaseRunner, *probeLedger) {
	t.Helper()
	var obs probeObservation
	runners := buildProbeRunners()
	led := &probeLedger{}
	o := core.NewOrchestrator(
		&probeStorage{state: core.State{LastCycleNumber: probeCycle - 1}},
		led,
		runners,
		core.WithWorktreeProvisioner(fixedWorktree{dir: repo}),
		core.WithVerdictCacheLookupHook(func(sha string, skipped, matched bool, _ verdictcache.Entry) {
			obs.calls++
			obs.sha, obs.skipped, obs.matched = sha, skipped, matched
		}),
	)
	if _, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: repo, GoalHash: "acs-1495"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return obs, runners, led
}

func auditBindingTreeSHA(t *testing.T, led *probeLedger) string {
	t.Helper()
	for _, e := range led.snapshot() {
		if e.Role == "auditor" && e.Kind == "agent_subprocess" {
			return e.WorktreeTreeSHA
		}
	}
	return ""
}

func assertPhasesRan(t *testing.T, runners map[core.Phase]core.PhaseRunner) {
	t.Helper()
	for _, p := range []core.Phase{core.PhaseTDD, core.PhaseBuild, core.PhaseAudit} {
		if got := runners[p].(*probeRunner).calls; got != 1 {
			t.Errorf("%s ran %d time(s), want 1 — shadow stage must never skip a phase", p, got)
		}
	}
}

func initProbeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "f.txt"), "x")
	writeFile(t, filepath.Join(repo, ".gitignore"), ".evolve/\n")
	git(t, repo, "init", "-q")
	git(t, repo, "add", "f.txt", ".gitignore")
	git(t, repo, "commit", "-q", "-m", "init")
	return repo
}

func seedVerdictCache(t *testing.T, repo, treeSHA string) {
	t.Helper()
	if err := verdictcache.NewStore(repo, nil).Put(verdictcache.Entry{
		TreeSHA:        treeSHA,
		Cycle:          probeCycle - 1,
		Verdict:        core.VerdictPASS,
		ArtifactSHA256: "acs-seed",
		ArtifactPath:   "acs-seed",
	}); err != nil {
		t.Fatalf("seed verdict cache: %v", err)
	}
}

func stageAndWriteTree(t *testing.T, repo string) string {
	t.Helper()
	git(t, repo, "add", "-A")
	return git(t, repo, "write-tree")
}

func baseTreeSHA(t *testing.T, repo string) string {
	t.Helper()
	return git(t, repo, "rev-parse", "HEAD^{tree}")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type probeStorage struct {
	mu         sync.Mutex
	state      core.State
	cycleState core.CycleState
	locked     bool
}

func (s *probeStorage) ReadState(context.Context) (core.State, error) { return s.state, nil }
func (s *probeStorage) WriteState(_ context.Context, st core.State) error {
	s.state = st
	return nil
}
func (s *probeStorage) ReadCycleState(context.Context) (core.CycleState, error) {
	return s.cycleState, nil
}
func (s *probeStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	s.cycleState = cs
	return nil
}
func (s *probeStorage) AcquireLock(context.Context) (func() error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, core.ErrLockHeld
	}
	s.locked = true
	return func() error {
		s.mu.Lock()
		s.locked = false
		s.mu.Unlock()
		return nil
	}, nil
}

type probeLedger struct {
	mu      sync.Mutex
	entries []core.LedgerEntry
}

func (l *probeLedger) Append(_ context.Context, e core.LedgerEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
	return nil
}
func (l *probeLedger) Verify(context.Context) error { return nil }
func (l *probeLedger) Iter(context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("acs: ledger iteration not used")
}
func (l *probeLedger) snapshot() []core.LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]core.LedgerEntry(nil), l.entries...)
}

type probeRunner struct {
	name  string
	calls int
}

func (r *probeRunner) Name() string { return r.name }
func (r *probeRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	if r.name == string(core.PhaseAudit) && req.Workspace != "" {
		if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
			return core.PhaseResponse{}, err
		}
		if err := os.WriteFile(filepath.Join(req.Workspace, "audit-report.md"),
			[]byte("## Verdict\n**PASS**\n"), 0o644); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func buildProbeRunners() map[core.Phase]core.PhaseRunner {
	out := map[core.Phase]core.PhaseRunner{}
	for _, p := range []core.Phase{
		core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD,
		core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro,
	} {
		out[p] = &probeRunner{name: string(p)}
	}
	return out
}

type fixedWorktree struct{ dir string }

func (f fixedWorktree) Create(string, int) (string, error) { return f.dir, nil }
func (f fixedWorktree) Cleanup(string, string) error       { return nil }
