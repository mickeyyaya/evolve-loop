//go:build acs

package cycle1799

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	fakeGHEnv       = "C1799_FAKE_GH"
	fakeGHMergedEnv = "C1799_FAKE_GH_MERGED"
	fakeGHLogEnv    = "C1799_FAKE_GH_LOG"
	fixtureLane     = "c0ffee01"
	devTask         = "t1799"
	devBranch       = "dev-b1799"
)

const (
	sealedUnbound    = 21
	sealedBound      = 32
	sealedFreshLease = 43
	unsealed         = 54
	sealedStaleLease = 65
	liveLaneCycle    = 87
	deadLaneCycle    = 98
)

var (
	keptWord      = regexp.MustCompile(`(?i)kept`)
	boundReason   = regexp.MustCompile(`(?i)continuation|bound|binding`)
	leaseReason   = regexp.MustCompile(`(?i)lease`)
	liveWord      = regexp.MustCompile(`(?i)live`)
	dirtyCause    = regexp.MustCompile(`(?i)dirty|uncommitted|not clean|unclean|local changes|modified`)
	unmergedCause = regexp.MustCompile(`(?i)unmerged|not merged`)
)

var staleHeartbeat = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func TestMain(m *testing.M) {
	if os.Getenv(fakeGHEnv) == "1" {
		os.Exit(serveFakeGH(os.Args[1:], os.Stdout, os.Stderr))
	}
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1799: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(scrubbedEnviron(), "PATH="+os.Getenv("PATH"), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitSucceeds(dir string, args ...string) bool {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(scrubbedEnviron(), "PATH="+os.Getenv("PATH"), "GIT_TERMINAL_PROMPT=0")
	return cmd.Run() == nil
}

func branchExists(dir, branch string) bool {
	return gitSucceeds(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
}

func pathExists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Stat(p)
	if err == nil {
		return true
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", p, err)
	}
	return false
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func commitIn(t *testing.T, dir, name, body, msg string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, name), body)
	gitIn(t, dir, "add", name)
	gitIn(t, dir, "commit", "-q", "-m", msg)
}

func resolved(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("resolve %s: %v", p, err)
	}
	return r
}

func linesNaming(out, leaf string) []string {
	re := regexp.MustCompile(regexp.QuoteMeta(leaf) + `(\D|$)`)
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if re.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return lines
}

func anyLineMatches(lines []string, res ...*regexp.Regexp) bool {
	for _, line := range lines {
		all := true
		for _, re := range res {
			if !re.MatchString(line) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func laneLeaf(cycle int) string { return fmt.Sprintf("cycle-%s-%d", fixtureLane, cycle) }

func laneDir(root string, cycle int) string {
	return filepath.Join(root, ".evolve", "worktrees", laneLeaf(cycle))
}

func runDir(root string, cycle int) string {
	return filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
}

func sealCycle(t *testing.T, root string, cycle int) {
	t.Helper()
	writeFile(t, filepath.Join(root, "knowledge-base", "cycles", fmt.Sprintf("cycle-%d.json", cycle)),
		fmt.Sprintf(`{"cycle":%d,"verdict":"PASS"}`+"\n", cycle))
}

func leaseCycle(t *testing.T, root string, cycle int, heartbeat time.Time) {
	t.Helper()
	dir := runDir(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(dir, runlease.Lease{RunID: "fixture-run", OwnerPID: os.Getpid()}, heartbeat); err != nil {
		t.Fatalf("write lease for cycle %d: %v", cycle, err)
	}
}

func bindCycleBranch(t *testing.T, root string, cycle int) {
	t.Helper()
	leaf := laneLeaf(cycle)
	sha := gitIn(t, root, "rev-parse", leaf)
	binding := continuation.Continuation{
		Worktree:    filepath.Join("/retired-hub", ".evolve", "worktrees", leaf),
		Branch:      leaf,
		SnapshotSHA: sha,
		BaseSHA:     sha,
		Cycle:       cycle,
	}
	if err := continuation.WriteRegistryEntry(root, "scope-"+leaf, binding); err != nil {
		t.Fatalf("bind %s: %v", leaf, err)
	}
}

func addLane(t *testing.T, r *gittest.Repo, cycle int) {
	t.Helper()
	r.Git("worktree", "add", "-q", "-b", laneLeaf(cycle), laneDir(r.Dir, cycle), "main")
}

func staleLaneFixture(t *testing.T) string {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("config", "commit.gpgsign", "false")
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	for _, n := range []int{sealedUnbound, sealedBound, sealedFreshLease, unsealed, sealedStaleLease} {
		addLane(t, r, n)
	}
	r.Git("commit", "-q", "--allow-empty", "-m", "advance main past every lane")
	for _, n := range []int{sealedUnbound, sealedBound, sealedFreshLease, sealedStaleLease} {
		sealCycle(t, r.Dir, n)
	}
	leaseCycle(t, r.Dir, sealedFreshLease, time.Now())
	leaseCycle(t, r.Dir, sealedStaleLease, staleHeartbeat)
	bindCycleBranch(t, r.Dir, sealedBound)
	return r.Dir
}

func registeredWorktreeLeaves(t *testing.T, root string) map[string]bool {
	t.Helper()
	leaves := map[string]bool{}
	for _, line := range strings.Split(gitIn(t, root, "worktree", "list", "--porcelain"), "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			leaves[filepath.Base(p)] = true
		}
	}
	return leaves
}

func runStaleCleanup(t *testing.T, root string, extra ...string) result {
	t.Helper()
	env, _ := cliEnv(t, false)
	args := append([]string{"worktree", "cleanup", "--stale", "--project-root", root}, extra...)
	return runEvolve(t, root, env, args...)
}

func TestC1799_001_WorktreeCleanupStaleDryRunListsSealedUnboundAndRemovesNothing(t *testing.T) {
	root := staleLaneFixture(t)
	res := runStaleCleanup(t, root)
	if res.code != 0 {
		t.Fatalf("`evolve worktree cleanup --stale` (dry-run default) must exit 0: %s", res)
	}
	for _, n := range []int{sealedUnbound, sealedStaleLease} {
		lines := linesNaming(res.combined(), laneLeaf(n))
		if len(lines) == 0 {
			t.Errorf("dry-run must list sealed, unbound %s as removable; it is absent: %s", laneLeaf(n), res)
		} else if anyLineMatches(lines, keptWord) {
			t.Errorf("dry-run reports sealed, unbound, unleased %s as kept: %q", laneLeaf(n), lines)
		}
	}
	for _, n := range []int{sealedBound, sealedFreshLease, unsealed} {
		if !anyLineMatches(linesNaming(res.combined(), laneLeaf(n)), keptWord) {
			t.Errorf("dry-run must report %s as kept: %s", laneLeaf(n), res)
		}
	}
	registered := registeredWorktreeLeaves(t, root)
	for _, n := range []int{sealedUnbound, sealedBound, sealedFreshLease, unsealed, sealedStaleLease} {
		if !pathExists(t, laneDir(root, n)) || !registered[laneLeaf(n)] || !branchExists(root, laneLeaf(n)) {
			t.Errorf("the dry-run default removed %s (dir, registration or branch): it must remove nothing", laneLeaf(n))
		}
	}
}

func TestC1799_002_WorktreeCleanupStaleApplyRemovesExactlySealedUnboundLanes(t *testing.T) {
	root := staleLaneFixture(t)
	res := runStaleCleanup(t, root, "--apply")
	if res.code != 0 {
		t.Fatalf("`evolve worktree cleanup --stale --apply` must exit 0: %s", res)
	}
	registered := registeredWorktreeLeaves(t, root)
	for _, n := range []int{sealedUnbound, sealedStaleLease} {
		leaf := laneLeaf(n)
		if pathExists(t, laneDir(root, n)) || registered[leaf] {
			t.Errorf("--apply must remove sealed, unbound %s; it is still on disk or registered: %s", leaf, res)
		}
		if branchExists(root, leaf) {
			t.Errorf("--apply must delete the superseded branch %s of the removed worktree: %s", leaf, res)
		}
	}
	for _, n := range []int{sealedBound, sealedFreshLease, unsealed} {
		leaf := laneLeaf(n)
		if !pathExists(t, laneDir(root, n)) || !registered[leaf] {
			t.Errorf("--apply removed %s, which must be kept: %s", leaf, res)
		}
		if !branchExists(root, leaf) {
			t.Errorf("--apply deleted the branch %s of a kept worktree: %s", leaf, res)
		}
	}
}

func TestC1799_003_WorktreeCleanupStaleKeepsBoundAndLeasedLanesWithTheirReason(t *testing.T) {
	root := staleLaneFixture(t)
	for _, mode := range [][]string{nil, {"--apply"}} {
		res := runStaleCleanup(t, root, mode...)
		if res.code != 0 {
			t.Fatalf("`evolve worktree cleanup --stale %s` must exit 0: %s", strings.Join(mode, " "), res)
		}
		if !anyLineMatches(linesNaming(res.combined(), laneLeaf(sealedBound)), keptWord, boundReason) {
			t.Errorf("mode %q: %s is named by a continuation binding and must be reported kept with that reason: %s",
				mode, laneLeaf(sealedBound), res)
		}
		if !anyLineMatches(linesNaming(res.combined(), laneLeaf(sealedFreshLease)), keptWord, leaseReason) {
			t.Errorf("mode %q: %s holds a fresh run lease and must be reported kept with that reason: %s",
				mode, laneLeaf(sealedFreshLease), res)
		}
	}
	if !pathExists(t, laneDir(root, sealedBound)) || !pathExists(t, laneDir(root, sealedFreshLease)) {
		t.Errorf("a bound or leased lane was removed by --apply")
	}
}

func TestC1799_004_WorktreeCleanupStaleOnAnEmptyWorktreeSetIsANoOp(t *testing.T) {
	r := gittest.Fixture(t)
	r.Git("config", "commit.gpgsign", "false")
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	for _, mode := range [][]string{nil, {"--apply"}} {
		res := runStaleCleanup(t, r.Dir, mode...)
		if res.code != 0 {
			t.Errorf("`evolve worktree cleanup --stale %s` with no cycle worktrees must exit 0: %s", strings.Join(mode, " "), res)
		}
	}
	if got := len(registeredWorktreeLeaves(t, r.Dir)); got != 1 {
		t.Errorf("the main worktree must be the only registered worktree, got %d", got)
	}
}

func TestC1799_005_BranchesAuditReportsALiveLaneAsLiveNeverSuperseded(t *testing.T) {
	r := gittest.Fixture(t)
	r.Git("config", "commit.gpgsign", "false")
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	addLane(t, r, liveLaneCycle)
	r.Git("branch", laneLeaf(deadLaneCycle))
	r.Git("commit", "-q", "--allow-empty", "-m", "advance main past both lanes")
	leaseCycle(t, r.Dir, liveLaneCycle, time.Now())
	sealCycle(t, r.Dir, deadLaneCycle)

	env, _ := cliEnv(t, false)
	res := runEvolve(t, r.Dir, env, "branches", "audit", "--project-root", r.Dir, "--base", "main")
	if res.code != 0 {
		t.Fatalf("`evolve branches audit` must exit 0: %s", res)
	}
	live := linesNaming(res.stdout, laneLeaf(liveLaneCycle))
	if len(live) == 0 {
		t.Fatalf("audit must report the live lane's branch %s: %s", laneLeaf(liveLaneCycle), res)
	}
	if anyLineMatches(live, regexp.MustCompile(`superseded=true`)) {
		t.Errorf("audit reports the live lane's branch %s as superseded: %q", laneLeaf(liveLaneCycle), live)
	}
	if !anyLineMatches(live, liveWord) {
		t.Errorf("audit must show the live lane's branch %s as live: %q", laneLeaf(liveLaneCycle), live)
	}
	if !anyLineMatches(linesNaming(res.stdout, laneLeaf(deadLaneCycle)), regexp.MustCompile(`superseded=true`)) {
		t.Errorf("a sealed, unleased branch %s contained in main must still be reported superseded=true: %s",
			laneLeaf(deadLaneCycle), res)
	}
	if !branchExists(r.Dir, laneLeaf(liveLaneCycle)) || !branchExists(r.Dir, laneLeaf(deadLaneCycle)) {
		t.Errorf("audit must be read-only")
	}
}

type hub struct {
	dir, store, runtime string
	origin, seed        *gittest.Repo
}

func newHub(t *testing.T) *hub {
	t.Helper()
	origin := gittest.Bare(t)
	seed := gittest.Fixture(t)
	seed.Git("config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(seed.Dir, "README.md"), "base\n")
	seed.Git("add", "README.md")
	seed.Git("commit", "-q", "-m", "base")
	seed.Git("remote", "add", "origin", origin.Dir)
	seed.Git("push", "-q", "origin", "main")

	h := &hub{dir: filepath.Join(t.TempDir(), "hub"), origin: origin, seed: seed}
	h.store, h.runtime = filepath.Join(h.dir, ".repo.git"), filepath.Join(h.dir, "runtime")
	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, h.dir, "clone", "-q", "--bare", origin.Dir, h.store)
	for _, kv := range [][2]string{
		{"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"},
		{"user.name", "acs"}, {"user.email", "acs@evolve.local"}, {"commit.gpgsign", "false"},
		{"maintenance.auto", "false"}, {"gc.auto", "0"},
	} {
		gitIn(t, h.store, "config", kv[0], kv[1])
	}
	gitIn(t, h.store, "fetch", "-q", "origin")
	gitIn(t, h.store, "worktree", "add", "-q", h.runtime, "main")
	return h
}

func (h *hub) devDir(task string) string { return filepath.Join(h.dir, "dev", task) }

func (h *hub) landOnOrigin(t *testing.T, name, body, msg string) string {
	t.Helper()
	commitIn(t, h.seed.Dir, name, body, msg)
	h.seed.Git("push", "-q", "origin", "main")
	return h.origin.Git("rev-parse", "main")
}

func (h *hub) createDev(t *testing.T, env []string, task, branch string) result {
	t.Helper()
	return runEvolve(t, h.runtime, env, "worktree", "create", "--dev", task, "--branch", branch, "--project-root", h.runtime)
}

func (h *hub) mustCreateDev(t *testing.T) string {
	t.Helper()
	env, _ := cliEnv(t, false)
	if res := h.createDev(t, env, devTask, devBranch); res.code != 0 {
		t.Fatalf("`evolve worktree create --dev %s --branch %s` must exit 0: %s", devTask, devBranch, res)
	}
	return h.devDir(devTask)
}

func (h *hub) cleanupDev(t *testing.T, env []string, task string) result {
	t.Helper()
	return runEvolve(t, h.runtime, env, "worktree", "cleanup", "--dev", task, "--project-root", h.runtime)
}

func (h *hub) assertDevKept(t *testing.T, why string) {
	t.Helper()
	if !pathExists(t, h.devDir(devTask)) {
		t.Errorf("%s: dev/%s was removed", why, devTask)
	}
	if !branchExists(h.store, devBranch) {
		t.Errorf("%s: branch %s was deleted", why, devBranch)
	}
}

func (h *hub) assertDevRemoved(t *testing.T, res result, ghLog string) {
	t.Helper()
	if pathExists(t, h.devDir(devTask)) {
		t.Errorf("dev/%s must be removed: %s\ngh: %s", devTask, res, ghInvocations(ghLog))
	}
	if registeredWorktreeLeaves(t, h.store)[devTask] {
		t.Errorf("dev/%s is still registered as a worktree: %s", devTask, res)
	}
	if branchExists(h.store, devBranch) {
		t.Errorf("branch %s must be removed together with its tree: %s\ngh: %s", devBranch, res, ghInvocations(ghLog))
	}
}

func (h *hub) assertRefusedAsUnmerged(t *testing.T, res result, ghLog, unmergedCommit, why string) {
	t.Helper()
	if res.code != 1 {
		t.Errorf("%s: cleanup --dev must refuse the unmerged work with exit 1: %s\ngh: %s", why, res, ghInvocations(ghLog))
	}
	if !unmergedCause.MatchString(res.combined()) {
		t.Errorf("%s: cleanup --dev must name the unmerged branch as the cause: %s", why, res)
	}
	h.assertDevKept(t, why)
	if !gitSucceeds(h.store, "merge-base", "--is-ancestor", unmergedCommit, "refs/heads/"+devBranch) {
		t.Errorf("%s: no branch holds %s any more, so the unmerged work is lost", why, unmergedCommit)
	}
	if t.Failed() {
		t.FailNow()
	}
}

func (h *hub) mergeAnEarlierPRUnderTheDevBranchName(t *testing.T) string {
	t.Helper()
	h.seed.Git("checkout", "-q", "-b", "earlier-use-of-the-name")
	commitIn(t, h.seed.Dir, "earlier.txt", "earlier work\n", "earlier: work under the same branch name")
	earlierPRHead := h.seed.Git("rev-parse", "HEAD")
	h.seed.Git("push", "-q", "origin", "HEAD:refs/heads/"+devBranch)
	h.seed.Git("checkout", "-q", "main")
	h.landOnOrigin(t, "earlier.txt", "earlier work\n", "earlier (#7) squashed")
	h.seed.Git("push", "-q", "origin", ":refs/heads/"+devBranch)
	return earlierPRHead
}

func TestC1799_006_WorktreeCreateDevMakesDevTaskOnBranchAtFetchedOriginMain(t *testing.T) {
	h := newHub(t)
	staleLocalMain := gitIn(t, h.runtime, "rev-parse", "HEAD")
	originTip := h.landOnOrigin(t, "ahead.txt", "only on origin\n", "origin moves on after the hub was cloned")
	env, _ := cliEnv(t, false)
	res := h.createDev(t, env, devTask, devBranch)
	if res.code != 0 {
		t.Fatalf("`evolve worktree create --dev %s --branch %s` must exit 0: %s", devTask, devBranch, res)
	}
	dev := h.devDir(devTask)
	if !pathExists(t, dev) {
		t.Fatalf("create --dev must place the tree at <hub>/dev/%s: %s", devTask, res)
	}
	if got := gitIn(t, dev, "rev-parse", "--abbrev-ref", "HEAD"); got != devBranch {
		t.Errorf("dev/%s must be on branch %s, got %q", devTask, devBranch, got)
	}
	if got := gitIn(t, dev, "rev-parse", "HEAD"); got != originTip {
		t.Errorf("dev/%s must start at the fetched origin/main %s, got %s (stale local main is %s)", devTask, originTip, got, staleLocalMain)
	}
	if got := gitIn(t, dev, "rev-parse", "--path-format=absolute", "--git-common-dir"); resolved(t, got) != resolved(t, h.store) {
		t.Errorf("dev/%s must be a worktree of the hub store %s, its common dir is %s", devTask, h.store, got)
	}
}

func TestC1799_007_WorktreeCreateDevRefusesAnExistingTaskOrBranchWithExitOne(t *testing.T) {
	h := newHub(t)
	h.mustCreateDev(t)
	env, _ := cliEnv(t, false)
	if res := h.createDev(t, env, devTask, "dev-other1799"); res.code != 1 {
		t.Errorf("create --dev for an existing task must exit 1: %s", res)
	}
	if branchExists(h.store, "dev-other1799") {
		t.Errorf("a refused create left a new branch dev-other1799 behind")
	}
	gitIn(t, h.store, "branch", "parked-b1799", "main")
	if res := h.createDev(t, env, "t1799-second", "parked-b1799"); res.code != 1 {
		t.Errorf("create --dev with an existing branch must exit 1: %s", res)
	}
	if pathExists(t, h.devDir("t1799-second")) {
		t.Errorf("a refused create left dev/t1799-second behind")
	}
}

func TestC1799_008_WorktreeCreateDevExitsTwoOnAGitIOFailure(t *testing.T) {
	h := newHub(t)
	gitIn(t, h.store, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "vanished.git"))
	env, _ := cliEnv(t, false)
	res := h.createDev(t, env, devTask, devBranch)
	if res.code != 2 {
		t.Errorf("create --dev whose fetch of origin fails must exit 2 (git I/O): %s", res)
	}
	if pathExists(t, h.devDir(devTask)) {
		t.Errorf("a failed create left dev/%s behind", devTask)
	}
	if branchExists(h.store, devBranch) {
		t.Errorf("a failed create left branch %s behind", devBranch)
	}
}

func TestC1799_009_WorktreeCleanupDevRefusesADirtyTreeWithExitOne(t *testing.T) {
	h := newHub(t)
	dev := h.mustCreateDev(t)
	writeFile(t, filepath.Join(dev, "README.md"), "uncommitted edit\n")
	env, ghLog := cliEnv(t, true)
	res := h.cleanupDev(t, env, devTask)
	if res.code != 1 {
		t.Errorf("cleanup --dev of a dirty tree must exit 1: %s\ngh: %s", res, ghInvocations(ghLog))
	}
	if !dirtyCause.MatchString(res.combined()) {
		t.Errorf("cleanup --dev must name the dirty tree as the cause: %s", res)
	}
	h.assertDevKept(t, "dirty tree")
	if pathExists(t, dev) && readFile(t, filepath.Join(dev, "README.md")) != "uncommitted edit\n" {
		t.Errorf("cleanup --dev discarded the uncommitted edit")
	}
}

func TestC1799_010_WorktreeCleanupDevRefusesAnUnmergedBranchWithExitOne(t *testing.T) {
	h := newHub(t)
	dev := h.mustCreateDev(t)
	commitIn(t, dev, "feature.txt", "unmerged work\n", "feature: not merged anywhere")
	env, ghLog := cliEnv(t, true)
	res := h.cleanupDev(t, env, devTask)
	if res.code != 1 {
		t.Errorf("cleanup --dev of an unmerged branch must exit 1: %s\ngh: %s", res, ghInvocations(ghLog))
	}
	if !unmergedCause.MatchString(res.combined()) {
		t.Errorf("cleanup --dev must name the unmerged branch as the cause: %s", res)
	}
	h.assertDevKept(t, "unmerged branch")
}

func TestC1799_011_WorktreeCleanupDevRemovesAMergedTreeAndItsBranch(t *testing.T) {
	h := newHub(t)
	dev := h.mustCreateDev(t)
	commitIn(t, dev, "feature.txt", "merged work\n", "feature: lands on main")
	gitIn(t, dev, "push", "-q", "origin", "HEAD:main")
	env, ghLog := cliEnv(t, true)
	res := h.cleanupDev(t, env, devTask)
	if res.code != 0 {
		t.Fatalf("cleanup --dev of a clean tree whose head is in origin/main must exit 0: %s\ngh: %s", res, ghInvocations(ghLog))
	}
	h.assertDevRemoved(t, res, ghLog)
}

func TestC1799_012_WorktreeCleanupDevCountsASquashMergedPRAsMerged(t *testing.T) {
	h := newHub(t)
	dev := h.mustCreateDev(t)
	commitIn(t, dev, "feature.txt", "draft\n", "feature: draft")
	commitIn(t, dev, "feature.txt", "final\n", "feature: final")
	gitIn(t, dev, "push", "-q", "origin", devBranch)
	mergedPRHead := gitIn(t, dev, "rev-parse", "HEAD")
	h.landOnOrigin(t, "feature.txt", "final\n", "feature (#7) squashed")
	h.landOnOrigin(t, "feature.txt", "final\nfollow-up on main\n", "follow-up edit after the squash")

	noGH, _ := cliEnv(t, false)
	if res := h.cleanupDev(t, noGH, devTask); res.code == 0 {
		t.Errorf("without gh nothing proves the squash-merged branch merged, so cleanup --dev must refuse: %s", res)
	}
	h.assertDevKept(t, "no gh to prove the squash merge")

	env, ghLog := cliEnv(t, true, mergedPR{devBranch, mergedPRHead})
	res := h.cleanupDev(t, env, devTask)
	if res.code != 0 {
		t.Fatalf("a squash-merged PR (gh reports it MERGED) must count as merged, exit 0: %s\ngh: %s", res, ghInvocations(ghLog))
	}
	h.assertDevRemoved(t, res, ghLog)
}

func TestC1799_013_WorktreeCleanupDevRefusesWorkCommittedAfterItsPRMerged(t *testing.T) {
	h := newHub(t)
	dev := h.mustCreateDev(t)
	commitIn(t, dev, "feature.txt", "final\n", "feature: final")
	gitIn(t, dev, "push", "-q", "origin", devBranch)
	mergedPRHead := gitIn(t, dev, "rev-parse", "HEAD")
	h.landOnOrigin(t, "feature.txt", "final\n", "feature (#7) squashed")
	commitIn(t, dev, "follow-up.txt", "work after the merge\n", "follow-up: committed after the PR merged")
	postMergeCommit := gitIn(t, dev, "rev-parse", "HEAD")
	env, ghLog := cliEnv(t, true, mergedPR{devBranch, mergedPRHead})

	res := h.cleanupDev(t, env, devTask)
	h.assertRefusedAsUnmerged(t, res, ghLog, postMergeCommit, "an unpushed commit after the merged PR's head")

	gitIn(t, dev, "push", "-q", "origin", devBranch)
	res = h.cleanupDev(t, env, devTask)
	h.assertRefusedAsUnmerged(t, res, ghLog, postMergeCommit, "a pushed commit after the merged PR's head, in no merged PR")
}

func TestC1799_014_WorktreeCleanupDevRefusesABranchNameReusedFromAnEarlierMergedPR(t *testing.T) {
	h := newHub(t)
	earlierPRHead := h.mergeAnEarlierPRUnderTheDevBranchName(t)
	dev := h.mustCreateDev(t)
	commitIn(t, dev, "reuse.txt", "new work under a reused name\n", "reuse: new work, never merged")
	newWork := gitIn(t, dev, "rev-parse", "HEAD")

	env, ghLog := cliEnv(t, true, mergedPR{devBranch, earlierPRHead})
	res := h.cleanupDev(t, env, devTask)
	h.assertRefusedAsUnmerged(t, res, ghLog, newWork, "only an earlier PR under the reused branch name merged")

	gitIn(t, dev, "push", "-q", "origin", devBranch)
	h.landOnOrigin(t, "reuse.txt", "new work under a reused name\n", "reuse (#8) squashed")
	env, ghLog = cliEnv(t, true, mergedPR{devBranch, newWork}, mergedPR{devBranch, earlierPRHead})
	res = h.cleanupDev(t, env, devTask)
	if res.code != 0 {
		t.Fatalf("once a merged PR's head is the local head, the squash merge must count as merged, exit 0: %s\ngh: %s", res, ghInvocations(ghLog))
	}
	h.assertDevRemoved(t, res, ghLog)
}
