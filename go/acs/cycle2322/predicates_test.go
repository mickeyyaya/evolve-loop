//go:build acs

package cycle2322

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type hub struct {
	t       *testing.T
	store   *gittest.Repo
	root    string
	runtime string
}

func newHub(t *testing.T) *hub {
	t.Helper()
	src := gittest.Fixture(t)
	src.Write("base.txt", "base\n")
	src.Git("add", "-A")
	src.Git("commit", "-q", "-m", "base")
	store := gittest.Bare(t)
	src.Git("push", "-q", store.Dir, "main")
	store.Git("update-ref", "refs/remotes/origin/main", "main")
	root := filepath.Dir(store.Dir)
	runtime := filepath.Join(root, "runtime")
	store.Git("worktree", "add", "-q", "--detach", runtime, "main")
	return &hub{t: t, store: store, root: root, runtime: runtime}
}

func (h *hub) addDevWithWork(name string) string {
	h.t.Helper()
	dir := filepath.Join(h.root, "dev", name)
	h.store.Git("worktree", "add", "-q", "-b", "br-"+name, dir, "main")
	h.commitIn(dir, name+".txt")
	return dir
}

func (h *hub) commitIn(dir, rel string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(rel+"\n"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	h.store.Git("-C", dir, "add", "-A")
	h.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "work "+rel)
}

func (h *hub) tip(dir string) string {
	return h.store.Git("-C", dir, "rev-parse", "HEAD")
}

func (h *hub) plan(merged gc.MergedPRLookup) gc.DevManifest {
	h.t.Helper()
	m, err := gc.PlanDevWorktrees(context.Background(), gc.DevOptions{
		ProjectRoot: h.runtime,
		EvolveDir:   filepath.Join(h.runtime, ".evolve"),
		Now:         time.Now,
		Exec:        sysexec.DefaultRunner,
		Processes:   func(context.Context) ([]gc.Process, error) { return nil, nil },
		OpenPRs:     func(context.Context, string) (map[string][]time.Time, error) { return nil, nil },
		MergedPRs:   merged,
	})
	if err != nil {
		h.t.Fatalf("PlanDevWorktrees: %v", err)
	}
	return m
}

func mergedAt(branch string, oids ...string) gc.MergedPRLookup {
	return func(context.Context, string) (map[string][]string, error) {
		return map[string][]string{branch: oids}, nil
	}
}

func itemAt(t *testing.T, m gc.DevManifest, dir string) gc.DevItem {
	t.Helper()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range m.Items {
		if got, err := filepath.EvalSymlinks(it.Path); err == nil && got == want {
			return it
		}
	}
	t.Fatalf("no manifest item for %s in %+v", dir, m.Items)
	return gc.DevItem{}
}

func TestC2322_001_ABranchWhoseMergedPullRequestSitsAtItsTipIsLanded(t *testing.T) {
	h := newHub(t)
	dir := h.addDevWithWork("tip")
	it := itemAt(t, h.plan(mergedAt("br-tip", h.tip(dir))), dir)
	if it.Action != gc.DevActionArchive || !strings.Contains(it.Reason, "landed early: branch br-tip is in main") {
		t.Fatalf("RED: a clean fresh branch whose merged pull request headRefOid is its tip = %+v, want archived as landed early", it)
	}
}

func TestC2322_002_ALaterCommitOnAMergedBranchIsNotLanded(t *testing.T) {
	h := newHub(t)
	dir := h.addDevWithWork("later")
	merged := h.tip(dir)
	h.commitIn(dir, "after-merge.txt")
	it := itemAt(t, h.plan(mergedAt("br-later", merged)), dir)
	if it.Action != gc.DevActionKeep {
		t.Fatalf("RED: a commit after the merged headRefOid was read as landed: %+v", it)
	}
}

func TestC2322_003_AMergedPullRequestOfAnotherBranchOrAnEmptyOidLandsNothing(t *testing.T) {
	cases := map[string]func(tip string) gc.MergedPRLookup{
		"another branch at the same oid": func(tip string) gc.MergedPRLookup { return mergedAt("br-other", tip) },
		"an empty oid":                   func(string) gc.MergedPRLookup { return mergedAt("br-case", "") },
		"no merged pull request":         func(string) gc.MergedPRLookup { return mergedAt("br-case") },
	}
	for name, lookup := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHub(t)
			dir := h.addDevWithWork("case")
			if it := itemAt(t, h.plan(lookup(h.tip(dir))), dir); it.Action != gc.DevActionKeep {
				t.Fatalf("RED: %s read as landed: %+v", name, it)
			}
		})
	}
}

func TestC2322_004_TheMergedPullRequestSourceIsReadOnceForTheRun(t *testing.T) {
	h := newHub(t)
	first, second := h.addDevWithWork("one"), h.addDevWithWork("two")
	reads := 0
	m := h.plan(func(context.Context, string) (map[string][]string, error) {
		reads++
		return map[string][]string{"br-one": {h.tip(first)}, "br-two": {h.tip(second)}}, nil
	})
	if reads != 1 {
		t.Fatalf("RED: the merged pull request source was read %d times for two branches, want once", reads)
	}
	for _, dir := range []string{first, second} {
		if it := itemAt(t, m, dir); it.Action != gc.DevActionArchive {
			t.Errorf("a branch of the one read is not landed: %+v", it)
		}
	}
}

func TestC2322_005_AFailedSourceFallsBackToGitEvidenceAndTheReportSaysSo(t *testing.T) {
	for _, failure := range []error{gc.ErrPRLookupUnavailable, errors.New("gh exploded")} {
		h := newHub(t)
		byGit := h.addDevWithWork("git")
		h.store.Git("update-ref", "refs/remotes/origin/main", h.tip(byGit))
		byPR := h.addDevWithWork("pr")
		m := h.plan(func(context.Context, string) (map[string][]string, error) { return nil, failure })
		if it := itemAt(t, m, byGit); it.Action != gc.DevActionArchive {
			t.Errorf("RED: git evidence stopped landing a branch when the source failed (%v): %+v", failure, it)
		}
		if it := itemAt(t, m, byPR); it.Action != gc.DevActionKeep || it.Failed {
			t.Errorf("RED: a failed source landed or failed a branch (%v): %+v", failure, it)
		}
		said := false
		for _, n := range m.Notes {
			said = said || strings.Contains(n, "merged pull request source") && strings.Contains(n, "git evidence")
		}
		if !said {
			t.Errorf("RED: the report does not say the sweep fell back to git evidence (%v): notes %q", failure, m.Notes)
		}
	}
}

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	mod, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", append([]string{"test", "-count=1"}, args...)...)
	cmd.Dir = mod
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	if err != nil {
		t.Fatalf("go test did not start: %v", err)
	}
	return string(out), 0
}

func TestC2322_006_TheProductionEntryWiresTheGhSourceAndLandsByTheMergedTip(t *testing.T) {
	names := []string{
		"TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookupKeepsOnlyTheMergedHeadsOfEachBranch",
		"TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookupSaysUnavailableWithoutGh",
		"TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookupSurfacesAFailingGh",
		"TestRunGC_DryRunLandsADevWorktreeByItsMergedPullRequestTipAndNotAfterALaterCommit",
	}
	out, code := goTest(t, "-v", "-run", "^("+strings.Join(names, "|")+")$", "./cmd/evolve")
	if code != 0 {
		t.Fatalf("RED: the production caller path through evolve gc fails (exit %d):\n%s", code, acsassert.OutputTail(out, 25))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("RED: %s did not run and pass", n)
		}
	}
}

func TestC2322_007_EveryChangedFunctionOfTheLandedPathIsFullyCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	if out, code := goTest(t, "-coverprofile", profile, "./internal/gc"); code != 0 {
		t.Fatalf("RED: the gc package tests fail (exit %d):\n%s", code, acsassert.OutputTail(out, 25))
	}
	mod, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "tool", "cover", "-func="+profile)
	cmd.Dir = mod
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("go tool cover: %v", err)
	}
	for _, fn := range []string{"landed", "mergedTips"} {
		if pct, err := acsassert.FuncCoverage(string(raw), fn); err != nil || pct != 100 {
			t.Errorf("RED: %s coverage = %.1f%%, err = %v, want 100%%", fn, pct, err)
		}
	}
}
