package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func newLandingFixture(t *testing.T) (*gittest.Repo, *gittest.Repo, string) {
	t.Helper()
	hub := gittest.Fixture(t)
	hub.Write("README.md", "seed\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "seed")
	origin := gittest.Bare(t)
	hub.Git("remote", "add", "origin", origin.Dir)
	hub.Git("push", "-q", "origin", "main")
	return hub, origin, filepath.Join(hub.Dir, ".evolve", "worktrees")
}

func TestDefaultChecks_WorktreesScanAndApply(t *testing.T) {
	hub, _, base := newLandingFixture(t)
	landedWT := filepath.Join(base, "cycle-chk-1")
	hub.Git("worktree", "add", "-q", "-b", "cycle-chk-1", landedWT, "main")
	writeFile(t, filepath.Join(landedWT, "a.txt"), "a\n")
	hub.Git("-C", landedWT, "add", "-A")
	hub.Git("-C", landedWT, "commit", "-q", "-m", "landed")
	hub.Git("merge", "-q", "--no-ff", "cycle-chk-1", "-m", "merge landed")
	hub.Git("push", "-q", "origin", "main")

	unpushedWT := filepath.Join(base, "cycle-chk-2")
	hub.Git("worktree", "add", "-q", "-b", "cycle-chk-2", unpushedWT, "main")
	writeFile(t, filepath.Join(unpushedWT, "b.txt"), "b\n")
	hub.Git("-C", unpushedWT, "add", "-A")
	hub.Git("-C", unpushedWT, "commit", "-q", "-m", "unpushed")
	hub.Git("merge", "-q", "--no-ff", "cycle-chk-2", "-m", "unpushed merge")

	dirtyWT := filepath.Join(base, "cycle-chk-3")
	hub.Git("worktree", "add", "-q", "-b", "cycle-chk-3", dirtyWT, "main")
	writeFile(t, filepath.Join(dirtyWT, "dirty.txt"), "dirty\n")

	checks := DefaultChecks()
	if len(checks) != 2 || checks[0].Name() != "worktrees" || checks[1].Name() != "temp" {
		t.Fatalf("DefaultChecks = %v, want worktrees and temp checks", checks)
	}

	cc := CheckContext{
		ProjectRoot: hub.Dir,
		EvolveDir:   filepath.Join(hub.Dir, ".evolve"),
		Policy:      Policy{},
	}
	report, err := RunChecks(context.Background(), cc, checks)
	if err != nil {
		t.Fatalf("RunChecks: %v", err)
	}
	if len(report.Checks) != 2 || report.Checks[0].Name != "worktrees" || report.Checks[1].Name != "temp" {
		t.Fatalf("unexpected report: %+v", report)
	}
	var got CheckReport = report
	var _ CheckResult = got.Checks[0]
	var _ CheckItem = got.Checks[0].Items[0]

	byAction := map[string]CheckItem{}
	for _, it := range got.Checks[0].Items {
		byAction[it.Action+":"+filepath.Base(it.Subject)] = it
		if it.Rule == "" || it.Evidence == "" || it.Rule == it.Evidence {
			t.Errorf("invalid check item: %+v", it)
		}
	}
	if _, ok := byAction["remove:cycle-chk-1"]; !ok {
		t.Errorf("missing remove for landed cycle-chk-1: %+v", got.Checks[0].Items)
	}
	if _, ok := byAction["delete-branch:cycle-chk-1"]; !ok {
		t.Errorf("missing delete-branch for landed cycle-chk-1: %+v", got.Checks[0].Items)
	}
	if _, ok := byAction["flag-unmerged:cycle-chk-2"]; !ok {
		t.Errorf("missing flag-unmerged for unpushed cycle-chk-2: %+v", got.Checks[0].Items)
	}
	if _, ok := byAction["flag-dirty:cycle-chk-3"]; !ok {
		t.Errorf("missing flag-dirty for dirty cycle-chk-3: %+v", got.Checks[0].Items)
	}

	if err := checks[0].Apply(context.Background(), cc); err != nil {
		t.Fatalf("check.Apply: %v", err)
	}
	if hub.Git("branch", "--list", "cycle-chk-1") != "" {
		t.Errorf("cycle-chk-1 should be deleted after Apply")
	}
	if hub.Git("branch", "--list", "cycle-chk-2") == "" {
		t.Errorf("cycle-chk-2 should remain after Apply")
	}
}

type failingCheck struct {
	name string
	err  error
}

func (f failingCheck) Name() string                                            { return f.name }
func (f failingCheck) Scan(context.Context, CheckContext) ([]CheckItem, error) { return nil, f.err }
func (f failingCheck) Apply(context.Context, CheckContext) error               { return f.err }

type emptyCheck struct{}

func (emptyCheck) Name() string                                            { return "empty" }
func (emptyCheck) Scan(context.Context, CheckContext) ([]CheckItem, error) { return nil, nil }
func (emptyCheck) Apply(context.Context, CheckContext) error               { return nil }

func TestRunChecks_EmptyAndErrorPaths(t *testing.T) {
	t.Parallel()
	rep, err := RunChecks(context.Background(), CheckContext{}, []Check{emptyCheck{}})
	if err != nil || len(rep.Checks) != 1 || rep.Checks[0].Items == nil || len(rep.Checks[0].Items) != 0 {
		t.Fatalf("empty check report = %+v, err = %v", rep, err)
	}
	boom := errors.New("scan failed")
	if _, err := RunChecks(context.Background(), CheckContext{}, []Check{failingCheck{name: "bad", err: boom}}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	cc := CheckContext{
		ProjectRoot: t.TempDir(),
		Worktrees: WorktreeOptions{
			Exec: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
				return 128, nil
			},
		},
	}
	chk := worktreeCheck{}
	if _, err := chk.Scan(context.Background(), cc); err == nil {
		t.Fatal("expected Scan error on git worktree list exit 128")
	}
	if err := chk.Apply(context.Background(), cc); err == nil {
		t.Fatal("expected Apply error on git worktree list exit 128")
	}
	if got := worktreeCheckSubject(WorktreeItem{Branch: "cycle-orphan"}); got != "cycle-orphan" {
		t.Errorf("orphan subject = %q", got)
	}
	if got := worktreeCheckRule(WorktreeActionSalvageRemove); got != "worktree.salvage_expired" {
		t.Errorf("salvage rule = %q", got)
	}
	if got := worktreeCheckRule(WorktreeAction("custom")); got != "worktree.custom" {
		t.Errorf("default rule = %q", got)
	}
	if got := worktreeCheckEvidence(WorktreeItem{Action: WorktreeActionRemove}); got != "remove" {
		t.Errorf("fallback evidence = %q", got)
	}
}

func TestLandingTarget_ValidationAndPlanErrors(t *testing.T) {
	t.Parallel()
	for _, lt := range []LandingTarget{{Remote: "origin"}, {Branch: "main"}} {
		o := WorktreeOptions{ProjectRoot: t.TempDir(), Exec: sysexec.DefaultRunner, Landing: lt}
		if _, err := PlanWorktrees(o); !errors.Is(err, errInvalidLandingTarget) {
			t.Errorf("PlanWorktrees(%+v) err = %v, want errInvalidLandingTarget", lt, err)
		}
		if err := ApplyWorktrees(o, WorktreeManifest{}); !errors.Is(err, errInvalidLandingTarget) {
			t.Errorf("ApplyWorktrees(%+v) err = %v, want errInvalidLandingTarget", lt, err)
		}
	}
}

func TestPlanAndApplyWorktrees_LandingTargetEdgeCases(t *testing.T) {
	hub, _, base := newLandingFixture(t)

	hub.Git("switch", "-q", "-c", "cycle-orph-landed")
	writeFile(t, filepath.Join(hub.Dir, "ol.txt"), "ol\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "ol")
	hub.Git("switch", "-q", "main")
	hub.Git("merge", "-q", "--no-ff", "cycle-orph-landed", "-m", "merge ol")

	hub.Git("switch", "-q", "-c", "cycle-orph-sq")
	writeFile(t, filepath.Join(hub.Dir, "osq.txt"), "osq\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "osq")
	hub.Git("switch", "-q", "main")
	hub.Git("merge", "-q", "--squash", "cycle-orph-sq")
	hub.Git("commit", "-q", "-m", "squash osq")

	hub.Git("switch", "-q", "-c", "cycle-orph-open")
	writeFile(t, filepath.Join(hub.Dir, "oo.txt"), "oo\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "oo")
	hub.Git("switch", "-q", "main")

	hub.Git("push", "-q", "origin", "main")

	hub.Git("switch", "-q", "-c", "cycle-orph-nonet")
	hub.Git("switch", "-q", "main")

	sqWT := filepath.Join(base, "cycle-wt-sq")
	hub.Git("worktree", "add", "-q", "-b", "cycle-wt-sq", sqWT, "main")
	writeFile(t, filepath.Join(sqWT, "wsq.txt"), "wsq\n")
	hub.Git("-C", sqWT, "add", "-A")
	hub.Git("-C", sqWT, "commit", "-q", "-m", "wsq")
	hub.Git("merge", "-q", "--squash", "cycle-wt-sq")
	hub.Git("commit", "-q", "-m", "squash wsq")
	hub.Git("push", "-q", "origin", "main")

	opts := WorktreeOptions{
		ProjectRoot:  hub.Dir,
		WorktreeBase: base,
		EvolveDir:    filepath.Join(hub.Dir, ".evolve"),
		Exec:         sysexec.DefaultRunner,
		Landing:      LandingTarget{Remote: "origin", Branch: "main"},
	}
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	byBranch := map[string]WorktreeItem{}
	for _, it := range m.Items {
		byBranch[it.Branch+":"+string(it.Action)] = it
	}
	if byBranch["cycle-orph-open:flag-unmerged"].Reason != "orphan branch not merged into origin/main (no worktree)" {
		t.Errorf("open orphan reason = %q", byBranch["cycle-orph-open:flag-unmerged"].Reason)
	}
	if byBranch["cycle-orph-landed:delete-branch"].Head == "" {
		t.Errorf("landed orphan missing Head")
	}
	if byBranch["cycle-orph-sq:delete-branch"].Head == "" {
		t.Errorf("squash orphan missing Head")
	}
	if byBranch["cycle-orph-nonet:delete-branch"].Head == "" {
		t.Errorf("nonet orphan missing Head")
	}
	if err := ApplyWorktrees(opts, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}
	for _, gone := range []string{"cycle-orph-landed", "cycle-orph-sq", "cycle-orph-nonet", "cycle-wt-sq"} {
		if hub.Git("branch", "--list", gone) != "" {
			t.Errorf("%s should be deleted", gone)
		}
	}
	if hub.Git("branch", "--list", "cycle-orph-open") == "" {
		t.Errorf("cycle-orph-open should be kept")
	}
}

func TestPlanAndApplyWorktrees_LandingFailureModes(t *testing.T) {
	hub, _, base := newLandingFixture(t)
	hub.Git("switch", "-q", "-c", "cycle-fail-1")
	writeFile(t, filepath.Join(hub.Dir, "f1.txt"), "f1\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "f1")
	hub.Git("switch", "-q", "main")
	hub.Git("merge", "-q", "--no-ff", "cycle-fail-1", "-m", "merge f1")
	hub.Git("push", "-q", "origin", "main")

	opts := WorktreeOptions{
		ProjectRoot:  hub.Dir,
		WorktreeBase: base,
		EvolveDir:    filepath.Join(hub.Dir, ".evolve"),
		Exec:         sysexec.DefaultRunner,
		Landing:      LandingTarget{Remote: "origin", Branch: "main"},
	}

	boomMerged := errors.New("branch merged failed")
	failMerged := opts
	failMerged.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) >= 2 && args[0] == "branch" && args[1] == "--merged" {
			return boomMerged
		}
		return nil
	})
	if _, err := PlanWorktrees(failMerged); !errors.Is(err, boomMerged) {
		t.Errorf("expected branch --merged error, got %v", err)
	}

	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}

	boomFetch := errors.New("network down")
	failFetch := opts
	failFetch.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 0 && args[0] == "fetch" {
			return boomFetch
		}
		return nil
	})
	if err := ApplyWorktrees(failFetch, m); !errors.Is(err, boomFetch) {
		t.Errorf("expected fetch error on ApplyWorktrees, got %v", err)
	}
	if hub.Git("branch", "--list", "cycle-fail-1") == "" {
		t.Fatalf("branch should survive failed fetch")
	}

	boomMergeBase := errors.New("merge-base boom")
	failMergeBase := opts
	failMergeBase.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 0 && args[0] == "merge-base" {
			return boomMergeBase
		}
		return nil
	})
	if err := ApplyWorktrees(failMergeBase, m); !errors.Is(err, errMergeBaseFailed) {
		t.Errorf("expected merge-base error, got %v", err)
	}

	boomUpdateRef := errors.New("ref locked")
	failUpdateRef := opts
	failUpdateRef.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 0 && args[0] == "update-ref" {
			return boomUpdateRef
		}
		return nil
	})
	if err := ApplyWorktrees(failUpdateRef, m); !errors.Is(err, boomUpdateRef) {
		t.Errorf("expected update-ref error, got %v", err)
	}

	moved := WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-fail-1", Head: strings.Repeat("0", 40), Action: WorktreeActionDeleteBranch}}}
	if err := ApplyWorktrees(opts, moved); !errors.Is(err, errBranchTipMoved) {
		t.Errorf("expected moved head refusal, got %v", err)
	}

	missing := WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-missing-999", Action: WorktreeActionDeleteBranch}}}
	if err := ApplyWorktrees(opts, missing); !errors.Is(err, errBranchHeadUnreadable) {
		t.Errorf("expected missing head refusal, got %v", err)
	}

	wt := filepath.Join(base, "cycle-checked-out")
	hub.Git("worktree", "add", "-q", "-b", "cycle-checked-out", wt, "main")
	checkedOut := WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-checked-out", Action: WorktreeActionDeleteBranch}}}
	if err := ApplyWorktrees(opts, checkedOut); !errors.Is(err, errBranchCheckedOut) {
		t.Errorf("expected checked-out refusal, got %v", err)
	}

	if err := ApplyWorktrees(opts, WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-fail-1", Head: m.Items[0].Head, Action: WorktreeActionDeleteBranch}}}); err != nil {
		t.Fatalf("clean apply: %v", err)
	}

	hub.Git("switch", "-q", "-c", "cycle-sq-err")
	writeFile(t, filepath.Join(hub.Dir, "se.txt"), "se\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "se")
	hub.Git("switch", "-q", "main")
	boomSquash := errors.New("squash merge-base boom")
	squashCheckErr := opts
	squashCheckErr.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "merge-base" && len(args) > 1 && args[1] == "--is-ancestor" {
			return 1, nil
		}
		if len(args) > 0 && args[0] == "merge-base" {
			return 0, boomSquash
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if err := ApplyWorktrees(squashCheckErr, WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-sq-err", Action: WorktreeActionDeleteBranch}}}); !errors.Is(err, errMergeBaseFailed) {
		t.Errorf("expected squash check error, got %v", err)
	}

	if err := ApplyWorktrees(opts, WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-sq-err", Action: WorktreeActionDeleteBranch}}}); !errors.Is(err, errBranchNotLanded) {
		t.Errorf("expected not-landed refusal, got %v", err)
	}
}

func TestPlanWorktrees_SalvageAndFailedFetch(t *testing.T) {
	hub, _, base := newLandingFixture(t)
	wt := filepath.Join(base, "cycle-fetch-fail-9")
	hub.Git("worktree", "add", "-q", "-b", "cycle-fetch-fail-9", wt, "main")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(wt, old, old); err != nil {
		t.Fatal(err)
	}
	hub.Git("branch", "cycle-fetch-fail-orph-10", "main")
	hub.Git("remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	opts := WorktreeOptions{
		ProjectRoot:  hub.Dir,
		WorktreeBase: base,
		EvolveDir:    filepath.Join(hub.Dir, ".evolve"),
		Policy:       WorktreesPolicy{SalvageAfterHours: 24},
		Exec:         sysexec.DefaultRunner,
		Landing:      LandingTarget{Remote: "origin", Branch: "main"},
	}
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if len(m.Items) != 2 {
		t.Fatalf("expected 2 flag-unmerged items, got %+v", m.Items)
	}
	for _, it := range m.Items {
		if it.Action != WorktreeActionFlagUnmerged || !strings.Contains(it.Reason, "fetch") {
			t.Errorf("expected flag-unmerged with fetch reason, got %+v", it)
		}
	}
}

func TestGateLeftovers_ExtractedFunction(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	writeFile(t, filepath.Join(r.Dir, "tracked.txt"), "ok\n")
	r.Git("add", "tracked.txt")
	r.Git("commit", "-q", "-m", "init")

	reason, err := gateLeftovers(context.Background(), sysexec.DefaultRunner, r.Dir)
	if err != nil || reason != "" {
		t.Fatalf("clean repo: reason=%q err=%v", reason, err)
	}

	writeFile(t, filepath.Join(r.Dir, ".git", "info", "exclude"), ".env\n")
	writeFile(t, filepath.Join(r.Dir, ".env"), "SECRET=1\n")
	reason, err = gateLeftovers(context.Background(), sysexec.DefaultRunner, r.Dir)
	if err != nil || !strings.HasPrefix(reason, "holds ignored files: ") {
		t.Fatalf("ignored file: reason=%q err=%v", reason, err)
	}
	if err := os.Remove(filepath.Join(r.Dir, ".env")); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(r.Dir, "vendor", "sub")
	if err := os.MkdirAll(filepath.Join(nested, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	reason, err = gateLeftovers(context.Background(), sysexec.DefaultRunner, r.Dir)
	if err != nil || !strings.HasPrefix(reason, "holds a nested git repository: ") {
		t.Fatalf("nested repo: reason=%q err=%v", reason, err)
	}
}
