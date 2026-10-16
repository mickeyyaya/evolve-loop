//go:build acs

package cycle1905

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/opscmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitIntact   = 0
	exitBroken   = 1
	exitGitIO    = 2
	exitBadUsage = 10
)

var hubWorktrees = []string{"console", "runtime", "dev/task"}

type hubFixture struct {
	git  *gittest.Repo
	root string
}

func (h hubFixture) store() string { return filepath.Join(h.root, ".repo.git") }

func (h hubFixture) path(worktree string) string {
	return filepath.Join(h.root, filepath.FromSlash(worktree))
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	return dir
}

func newHub(t *testing.T) hubFixture {
	t.Helper()
	seed := gittest.Fixture(t)
	seed.Write("README.md", "seed\n")
	seed.Git("add", "README.md")
	seed.Git("commit", "-q", "-m", "seed")
	hub := hubFixture{git: seed, root: filepath.Join(canonicalTempDir(t), "hub")}
	seed.Git("clone", "-q", "--bare", seed.Dir, hub.store())
	storeGit := "--git-dir=" + hub.store()
	seed.Git(storeGit, "config", "gc.auto", "0")
	seed.Git(storeGit, "config", "maintenance.auto", "false")
	seed.Git(storeGit, "worktree", "add", "-q", "--detach", hub.path("console"), "main")
	seed.Git(storeGit, "worktree", "add", "-q", hub.path("runtime"), "main")
	seed.Git(storeGit, "worktree", "add", "-q", "-b", "task", hub.path("dev/task"), "main")
	return hub
}

func moveHub(t *testing.T, hub hubFixture) hubFixture {
	t.Helper()
	moved := hubFixture{git: hub.git, root: filepath.Join(canonicalTempDir(t), "moved-hub")}
	if err := os.Rename(hub.root, moved.root); err != nil {
		t.Fatalf("move hub: %v", err)
	}
	return moved
}

func doctorPlane(args ...string) (stdout, stderr string, code int) {
	var out, errb bytes.Buffer
	code = opscmd.RunDoctor(append([]string{"plane"}, args...), nil, &out, &errb)
	return out.String(), errb.String(), code
}

func lines(s string) []string {
	return strings.Split(strings.TrimSpace(s), "\n")
}

func hasLine(stdout, want string) bool {
	for _, l := range lines(stdout) {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

func prefixedLines(stdout, prefix string) []string {
	var found []string
	for _, l := range lines(stdout) {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			found = append(found, strings.TrimSpace(l))
		}
	}
	return found
}

func gitdirPointer(t *testing.T, worktreePath string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(worktreePath, ".git"))
	if err != nil {
		t.Fatalf("read %s/.git: %v", worktreePath, err)
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(raw)), "gitdir:"))
}

func storeRecord(t *testing.T, hub hubFixture, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(hub.store(), "worktrees", name, "gitdir"))
	if err != nil {
		t.Fatalf("read store record of %s: %v", name, err)
	}
	return strings.TrimSpace(string(raw))
}

func assertLinksResolve(t *testing.T, hub hubFixture) {
	t.Helper()
	for _, wt := range hubWorktrees {
		top := hub.git.Git("-C", hub.path(wt), "rev-parse", "--show-toplevel")
		if resolved, _ := filepath.EvalSymlinks(top); resolved != hub.path(wt) {
			t.Errorf("worktree %s resolves to %q after repair, want %q", wt, top, hub.path(wt))
		}
		if ptr := gitdirPointer(t, hub.path(wt)); !strings.HasPrefix(ptr, hub.store()) {
			t.Errorf("worktree %s .git still points at %q, want a record under %s", wt, ptr, hub.store())
		}
		name := filepath.Base(gitdirPointer(t, hub.path(wt)))
		if rec := storeRecord(t, hub, name); rec != filepath.Join(hub.path(wt), ".git") {
			t.Errorf("store record %s points at %q, want %q", name, rec, filepath.Join(hub.path(wt), ".git"))
		}
	}
}

func TestC1905_001_RepairRestoresMovedHubLinksAndPrintsEachRepairedPath(t *testing.T) {
	hub := moveHub(t, newHub(t))
	stdout, stderr, code := doctorPlane("--repair", hub.path("runtime"))
	if code != exitIntact {
		t.Fatalf("RED: doctor plane --repair on a moved hub exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitIntact, stdout, stderr)
	}
	for _, wt := range hubWorktrees {
		if want := "repaired: " + hub.path(wt); !hasLine(stdout, want) {
			t.Errorf("RED: stdout lacks the line %q\nstdout=%s", want, stdout)
		}
	}
	assertLinksResolve(t, hub)
	after, afterErr, afterCode := doctorPlane(hub.path("runtime"))
	if afterCode != exitIntact || len(prefixedLines(after, "broken:")) != 0 {
		t.Errorf("RED: inspection after repair exit=%d, want %d with no broken lines\nstdout=%s\nstderr=%s", afterCode, exitIntact, after, afterErr)
	}
}

func TestC1905_002_InspectionReportsBrokenLinksExit1AndRepairsNothing(t *testing.T) {
	cases := []struct {
		name       string
		breakHub   func(t *testing.T, hub hubFixture) (hubFixture, []string)
		rootPlane  string
		wantIntact []string
	}{
		{
			name: "hub directory moved breaks both link directions",
			breakHub: func(t *testing.T, hub hubFixture) (hubFixture, []string) {
				return moveHub(t, hub), hubWorktrees
			},
			rootPlane: "console",
		},
		{
			name: "dev worktree renamed inside the hub breaks only the store side",
			breakHub: func(t *testing.T, hub hubFixture) (hubFixture, []string) {
				if err := os.Rename(hub.path("dev/task"), hub.path("dev/task-renamed")); err != nil {
					t.Fatalf("rename dev worktree: %v", err)
				}
				return hub, []string{"dev/task-renamed"}
			},
			rootPlane:  "runtime",
			wantIntact: []string{"console", "runtime"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hub, broken := tc.breakHub(t, newHub(t))
			pointersBefore := map[string]string{}
			for _, wt := range broken {
				pointersBefore[wt] = gitdirPointer(t, hub.path(wt))
			}
			stdout, stderr, code := doctorPlane(hub.path(tc.rootPlane))
			if code != exitBroken {
				t.Fatalf("RED: doctor plane on a hub with broken links exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitBroken, stdout, stderr)
			}
			for _, wt := range broken {
				if want := "broken: " + hub.path(wt); !hasLine(stdout, want) {
					t.Errorf("RED: stdout lacks the line %q\nstdout=%s", want, stdout)
				}
				if got := gitdirPointer(t, hub.path(wt)); got != pointersBefore[wt] {
					t.Errorf("inspection without --repair rewrote %s/.git: %q -> %q", wt, pointersBefore[wt], got)
				}
			}
			for _, wt := range tc.wantIntact {
				if hasLine(stdout, "broken: "+hub.path(wt)) {
					t.Errorf("intact worktree %s reported broken\nstdout=%s", wt, stdout)
				}
			}
			if got := prefixedLines(stdout, "repaired:"); len(got) != 0 {
				t.Errorf("inspection without --repair printed repaired lines: %v", got)
			}
		})
	}
}

func TestC1905_003_GitFailureExits2UnknownFlagExitsUsageMissingPathErrors(t *testing.T) {
	t.Run("git worktree failure during repair exits 2", func(t *testing.T) {
		hub := moveHub(t, newHub(t))
		gittest.FaultOn(t, "worktree")
		stdout, stderr, code := doctorPlane("--repair", hub.path("runtime"))
		if code != exitGitIO {
			t.Fatalf("RED: --repair with a failing git worktree exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitGitIO, stdout, stderr)
		}
		if got := prefixedLines(stdout, "repaired:"); len(got) != 0 {
			t.Errorf("a failed repair claimed repaired paths: %v", got)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Errorf("a failed repair must name the git failure on stderr")
		}
	})
	t.Run("unknown flag exits usage", func(t *testing.T) {
		hub := newHub(t)
		stdout, stderr, code := doctorPlane("--no-such-flag", hub.path("runtime"))
		if code != exitBadUsage {
			t.Errorf("RED: unknown flag exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitBadUsage, stdout, stderr)
		}
	})
	t.Run("missing checkout path errors under --repair", func(t *testing.T) {
		missing := filepath.Join(canonicalTempDir(t), "absent")
		stdout, stderr, code := doctorPlane("--repair", missing)
		if code == exitIntact {
			t.Errorf("--repair on a missing path exit=0, want an error exit\nstdout=%s", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Errorf("--repair on a missing path must explain the error on stderr")
		}
	})
}

func TestC1905_004_HealthyHubExits0UnderInspectionAndRepair(t *testing.T) {
	hub := newHub(t)
	stdout, stderr, code := doctorPlane(hub.path("runtime"))
	if code != exitIntact {
		t.Fatalf("doctor plane on a healthy hub exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitIntact, stdout, stderr)
	}
	if !strings.Contains(stdout, "plane: linked worktree") {
		t.Errorf("healthy report lost the plane line\nstdout=%s", stdout)
	}
	if got := prefixedLines(stdout, "broken:"); len(got) != 0 {
		t.Errorf("healthy hub reported broken links: %v", got)
	}
	for _, args := range [][]string{{"--repair", hub.path("runtime")}, {hub.path("runtime"), "--repair"}} {
		repairOut, repairErr, repairCode := doctorPlane(args...)
		if repairCode != exitIntact {
			t.Fatalf("RED: doctor plane %v on a healthy hub exit=%d, want %d\nstdout=%s\nstderr=%s", args, repairCode, exitIntact, repairOut, repairErr)
		}
		if got := prefixedLines(repairOut, "repaired:"); len(got) != 0 {
			t.Errorf("doctor plane %v on a healthy hub claimed repairs: %v", args, got)
		}
	}
	assertLinksResolve(t, hub)
}

// acs-predicate: config-check
func TestC1905_005_WorkspaceLayoutDocNamesDoctorPlaneRepair(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "workspace-layout.md")
	if !acsassert.FileContains(t, doc, "evolve doctor plane --repair") {
		t.Errorf("RED: workspace-layout.md does not name evolve doctor plane --repair")
	}
	if !acsassert.FileNotContains(t, doc, "--git-dir=.repo.git worktree repair") {
		t.Errorf("RED: workspace-layout.md still tells the operator to run raw git worktree repair")
	}
}

const nestedLane = "runtime/.evolve/worktrees/cycle-lane"

func addNestedLane(t *testing.T, hub hubFixture) {
	t.Helper()
	hub.git.Git("--git-dir="+hub.store(), "worktree", "add", "-q", "-b", "lane", hub.path(nestedLane), "main")
}

func TestC1905_006_WorktreeUnderDotDirectoryIsInspectedAndRepairedAfterHubMove(t *testing.T) {
	hub := newHub(t)
	addNestedLane(t, hub)
	hub = moveHub(t, hub)
	lane := hub.path(nestedLane)
	pointerBefore := gitdirPointer(t, lane)

	stdout, stderr, code := doctorPlane(hub.path("console"))
	if code != exitBroken {
		t.Fatalf("RED: inspection of a moved hub exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitBroken, stdout, stderr)
	}
	if want := "broken: " + lane; !hasLine(stdout, want) {
		t.Errorf("RED: inspection skipped the worktree under a dot-directory: stdout lacks %q\nstdout=%s", want, stdout)
	}
	if got := gitdirPointer(t, lane); got != pointerBefore {
		t.Errorf("inspection without --repair rewrote %s/.git: %q -> %q", nestedLane, pointerBefore, got)
	}

	repairOut, repairErr, repairCode := doctorPlane("--repair", hub.path("console"))
	if repairCode != exitIntact {
		t.Fatalf("RED: --repair on a moved hub exit=%d, want %d\nstdout=%s\nstderr=%s", repairCode, exitIntact, repairOut, repairErr)
	}
	if want := "repaired: " + lane; !hasLine(repairOut, want) {
		t.Errorf("RED: stdout lacks %q\nstdout=%s", want, repairOut)
	}
	top := hub.git.Git("-C", lane, "rev-parse", "--show-toplevel")
	if resolved, _ := filepath.EvalSymlinks(top); resolved != lane {
		t.Errorf("RED: nested lane resolves to %q after repair, want %q", top, lane)
	}
	if rec := storeRecord(t, hub, filepath.Base(gitdirPointer(t, lane))); rec != filepath.Join(lane, ".git") {
		t.Errorf("RED: store record of the nested lane points at %q, want %q", rec, filepath.Join(lane, ".git"))
	}
	after, _, afterCode := doctorPlane(hub.path("console"))
	if afterCode != exitIntact || len(prefixedLines(after, "broken:")) != 0 {
		t.Errorf("RED: inspection after repair exit=%d, want %d with no broken lines\nstdout=%s", afterCode, exitIntact, after)
	}
}

type foreignCheckouts struct {
	linkedWorktree string
	separateGitDir string
}

func addForeignCheckouts(t *testing.T, hub hubFixture) foreignCheckouts {
	t.Helper()
	other := gittest.Fixture(t)
	other.Write("OTHER.md", "other\n")
	other.Git("add", "OTHER.md")
	other.Git("commit", "-q", "-m", "other")
	foreign := foreignCheckouts{
		linkedWorktree: hub.path("scratch/foreign-worktree"),
		separateGitDir: hub.path("runtime/vendor/separate"),
	}
	other.Git("worktree", "add", "-q", "--detach", foreign.linkedWorktree)
	other.Git("init", "-q", "--separate-git-dir", filepath.Join(canonicalTempDir(t), "separate.git"), foreign.separateGitDir)
	return foreign
}

func TestC1905_007_GitFilesOfOtherRepositoriesAreNeitherReportedNorRepaired(t *testing.T) {
	t.Run("healthy hub with foreign checkouts inspects clean", func(t *testing.T) {
		hub := newHub(t)
		foreign := addForeignCheckouts(t, hub)
		stdout, stderr, code := doctorPlane(hub.path("runtime"))
		if code != exitIntact {
			t.Errorf("RED: healthy hub holding foreign checkouts exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitIntact, stdout, stderr)
		}
		for _, p := range []string{foreign.linkedWorktree, foreign.separateGitDir} {
			if hasLine(stdout, "broken: "+p) {
				t.Errorf("RED: a checkout of another repository was reported broken: %s\nstdout=%s", p, stdout)
			}
		}
	})
	t.Run("moved hub repair touches only the store's own worktrees", func(t *testing.T) {
		hub := newHub(t)
		addForeignCheckouts(t, hub)
		hub = moveHub(t, hub)
		movedForeign := foreignCheckouts{
			linkedWorktree: hub.path("scratch/foreign-worktree"),
			separateGitDir: hub.path("runtime/vendor/separate"),
		}
		pointersBefore := map[string]string{}
		for _, p := range []string{movedForeign.linkedWorktree, movedForeign.separateGitDir} {
			pointersBefore[p] = gitdirPointer(t, p)
		}
		stdout, stderr, code := doctorPlane("--repair", hub.path("runtime"))
		if code != exitIntact {
			t.Fatalf("RED: --repair on a moved hub holding foreign checkouts exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitIntact, stdout, stderr)
		}
		for p, before := range pointersBefore {
			if hasLine(stdout, "repaired: "+p) || hasLine(stdout, "broken: "+p) {
				t.Errorf("RED: a checkout of another repository was claimed by the hub repair: %s\nstdout=%s", p, stdout)
			}
			if got := gitdirPointer(t, p); got != before {
				t.Errorf("RED: --repair rewrote a foreign .git %s: %q -> %q", p, before, got)
			}
		}
		for _, wt := range hubWorktrees {
			if want := "repaired: " + hub.path(wt); !hasLine(stdout, want) {
				t.Errorf("RED: stdout lacks %q\nstdout=%s", want, stdout)
			}
		}
	})
}

func TestC1905_008_RegisteredWorktreeOutsideTheHubTreeIsInspected(t *testing.T) {
	hub := newHub(t)
	outside := filepath.Join(canonicalTempDir(t), "outside-lane")
	hub.git.Git("--git-dir="+hub.store(), "worktree", "add", "-q", "-b", "outside", outside, "main")
	hub = moveHub(t, hub)

	stdout, stderr, code := doctorPlane(hub.path("runtime"))
	if code != exitBroken {
		t.Fatalf("RED: inspection of a moved hub exit=%d, want %d\nstdout=%s\nstderr=%s", code, exitBroken, stdout, stderr)
	}
	if want := "broken: " + outside; !hasLine(stdout, want) {
		t.Errorf("RED: worktree enumeration comes from a filesystem walk, not the store's records: stdout lacks %q\nstdout=%s", want, stdout)
	}
	repairOut, repairErr, repairCode := doctorPlane("--repair", hub.path("runtime"))
	if repairCode != exitIntact || !hasLine(repairOut, "repaired: "+outside) {
		t.Errorf("RED: --repair exit=%d, want %d with %q\nstdout=%s\nstderr=%s", repairCode, exitIntact, "repaired: "+outside, repairOut, repairErr)
	}
	if ptr := gitdirPointer(t, outside); !strings.HasPrefix(ptr, hub.store()) {
		t.Errorf("RED: outside worktree .git still points at %q, want a record under %s", ptr, hub.store())
	}
}
