package gc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var (
	_ CandidatePRAPI = GhRemoteAPI{}
	_ CandidatePRAPI = (*x11CandidateAPI)(nil)
)

type x11CandidateAPI struct {
	openRows   []RemotePR
	openTrunc  bool
	openErr    error
	allRows    []RemotePR
	allTrunc   bool
	allErr     error
	branchRows map[string][]RemotePR
	branchErr  map[string]error
	branchTr   map[string]bool
	protected  map[string]bool
	ruleErr    map[string]error
	deleted    []string
	deleteErr  error
}

func (a *x11CandidateAPI) PullRequests(context.Context) ([]RemotePR, error) {
	return a.allRows, a.allErr
}

func (a *x11CandidateAPI) OpenPullRequests(context.Context) ([]RemotePR, bool, error) {
	return a.openRows, a.openTrunc, a.openErr
}

func (a *x11CandidateAPI) AllPullRequests(context.Context) ([]RemotePR, bool, error) {
	return a.allRows, a.allTrunc, a.allErr
}

func (a *x11CandidateAPI) BranchPullRequests(_ context.Context, branch string) ([]RemotePR, bool, error) {
	return a.branchRows[branch], a.branchTr[branch], a.branchErr[branch]
}

func (a *x11CandidateAPI) RulesetProtects(_ context.Context, branch string) (bool, error) {
	return a.protected[branch], a.ruleErr[branch]
}

func (a *x11CandidateAPI) DeleteBranch(_ context.Context, branch string) error {
	a.deleted = append(a.deleted, branch)
	return a.deleteErr
}

func TestX11_SelectChecksAndBranchCheckScanApply(t *testing.T) {
	if all, err := SelectChecks(""); err != nil || len(all) != 3 {
		t.Fatalf("SelectChecks(\"\") = %v, %v", all, err)
	}
	if sel, err := SelectChecks("branches"); err != nil || len(sel) != 1 || sel[0].Name() != "branches" {
		t.Fatalf("SelectChecks(\"branches\") = %v, %v", sel, err)
	}
	if _, err := SelectChecks("bogus"); err == nil {
		t.Fatal("SelectChecks(\"bogus\") succeeded")
	}

	e := newBranchSweepEnv(t)
	origin := gittest.Bare(t)
	e.repo.Git("remote", "add", "origin", origin.Dir)
	e.repo.Git("push", "-q", "origin", "main")
	mergedTip := e.commitOn("console/b1", "b1.txt")
	e.repo.Git("push", "-q", "origin", "console/b1")
	closedTip := e.commitOn("console/b3", "b3.txt")
	e.repo.Git("push", "-q", "origin", "console/b3")
	openTip := e.commitOn("console/open", "open.txt")
	e.repo.Git("push", "-q", "origin", "console/open")
	now := time.Now().UTC()
	api := &x11CandidateAPI{
		openRows: []RemotePR{
			{Number: 3, Branch: "console/open", State: "OPEN", HeadRefOid: openTip},
		},
		allRows: []RemotePR{
			{Number: 1, Branch: "console/b1", State: "MERGED", HeadRefOid: mergedTip},
			{Number: 2, Branch: "console/b3", State: "CLOSED", HeadRefOid: closedTip, ClosedAt: now.Add(-10 * 24 * time.Hour)},
			{Number: 4, Branch: "console/open", State: "MERGED", HeadRefOid: openTip},
		},
	}
	chk := branchCheck{api: api, now: func() time.Time { return now }}
	cc := CheckContext{
		ProjectRoot: e.repo.Dir,
		Policy:      Policy{RemoteSweep: gcpolicy.RemoteSweepPolicy{Stage: "enforce"}},
	}
	items, err := chk.Scan(context.Background(), cc)
	if err != nil || len(items) != 4 {
		t.Fatalf("Scan = %+v, %v; want 4 items", items, err)
	}
	if err := chk.Apply(context.Background(), cc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if e.tip("console/b1") != "" || e.tip("console/b3") != "" {
		t.Errorf("branches survived branchCheck.Apply")
	}
	if e.tip("console/open") != openTip {
		t.Errorf("console/open with open PR was deleted or moved: got %q, want %q", e.tip("console/open"), openTip)
	}

	offline := branchCheck{
		api: &x11CandidateAPI{openErr: errors.New("offline")},
		now: func() time.Time { return now },
	}
	e.commitOn("console/offline", "off.txt")
	if items, err := offline.Scan(context.Background(), CheckContext{
		ProjectRoot: e.repo.Dir,
		Worktrees: WorktreeOptions{Exec: failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
			if len(args) > 0 && args[0] == "ls-remote" {
				return errors.New("offline")
			}
			return nil
		})},
	}); err != nil || len(items) != 0 {
		t.Fatalf("offline Scan = %+v, %v; want empty nil", items, err)
	}
	if _, err := offline.Scan(context.Background(), CheckContext{ProjectRoot: t.TempDir()}); err == nil {
		t.Fatal("Scan on non-repo succeeded")
	}
	if err := offline.Apply(context.Background(), CheckContext{ProjectRoot: t.TempDir()}); err == nil {
		t.Fatal("Apply on non-repo succeeded")
	}

	bp := BranchesPolicy{OwnedPrefixes: []string{"custom/"}}
	if !slices.Equal(bp.OwnedPrefixList(), []string{"custom/"}) || bp.IsOwned("cycle-1") || bp.IsOwned("stray/x") || !bp.IsOwned("custom/a") {
		t.Errorf("BranchesPolicy check failed")
	}
	var action BranchAction = BranchActionDelete
	manifest := BranchSweepManifest{
		Items: []BranchSweepItem{
			{Side: BranchSideRemote, Branch: "console/b1", Action: action},
			{Side: BranchSideLocal, Branch: "console/b1", Action: action},
			{Side: BranchSideRemote, Branch: "console/keep", Action: BranchAction("keep")},
		},
		RemoteKept: 2,
	}
	if planned, kept := manifest.RemoteCounts(); planned != 1 || kept != 2 {
		t.Errorf("RemoteCounts = (%d, %d), want (1, 2)", planned, kept)
	}
}

func TestX11_PlanAndApplyBranchesEdgeAndErrorPaths(t *testing.T) {
	e := newBranchSweepEnv(t)
	origin := gittest.Bare(t)
	e.repo.Git("remote", "add", "origin", origin.Dir)
	e.repo.Git("push", "-q", "origin", "main")
	now := time.Now().UTC()

	truncOK := e.commitOn("console/trunc-ok", "tok.txt")
	e.repo.Git("push", "-q", "origin", "console/trunc-ok")
	truncFail := e.commitOn("console/trunc-fail", "tf.txt")
	ruled := e.commitOn("console/ruled", "r.txt")
	e.repo.Git("push", "-q", "origin", "console/ruled")
	ruleErr := e.commitOn("console/rule-err", "re.txt")
	e.repo.Git("push", "-q", "origin", "console/rule-err")
	closedTrunc := e.commitOn("console/closed-trunc", "ct.txt")
	e.repo.Git("push", "-q", "origin", "console/closed-trunc")
	young := e.commitOn("console/young", "y.txt")
	e.repo.Git("push", "-q", "origin", "console/young")
	_ = truncFail

	api := &x11CandidateAPI{
		openTrunc: true,
		branchRows: map[string][]RemotePR{
			"console/trunc-ok": {{Branch: "console/trunc-ok", State: "MERGED", HeadRefOid: truncOK}},
			"console/ruled":    {{Number: 11, Branch: "console/ruled", State: "MERGED", HeadRefOid: ruled}},
			"console/rule-err": {{Number: 12, Branch: "console/rule-err", State: "MERGED", HeadRefOid: ruleErr}},
			"console/closed-trunc": {
				{Number: 13, Branch: "console/closed-trunc", State: "CLOSED", HeadRefOid: closedTrunc, ClosedAt: now.Add(-10 * 24 * time.Hour)},
				{Number: 14, Branch: "console/closed-trunc", State: "CLOSED", HeadRefOid: closedTrunc, ClosedAt: now.Add(-5 * 24 * time.Hour)},
			},
			"console/young": {{Number: 15, Branch: "console/young", State: "CLOSED", HeadRefOid: young, ClosedAt: now.Add(-2 * time.Hour)}},
		},
		branchErr: map[string]error{"console/trunc-fail": errors.New("branch lookup failed")},
		protected: map[string]bool{"console/ruled": true},
		ruleErr:   map[string]error{"console/rule-err": errors.New("rules api down")},
	}
	var log bytes.Buffer
	opts := BranchSweepOptions{
		ProjectRoot: e.repo.Dir,
		EvolveDir:   e.opts.EvolveDir,
		Exec:        sysexec.DefaultRunner,
		API:         api,
		Now:         func() time.Time { return now },
		Log:         &log,
		Stage:       RemoteStageEnforce,
	}
	m, err := PlanBranchSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanBranchSweep: %v", err)
	}
	if _, ok := branchSweepPlanItem(m, BranchSideRemote, "console/ruled"); ok {
		t.Error("ruleset-protected branch planned on remote")
	}
	if _, ok := branchSweepPlanItem(m, BranchSideRemote, "console/rule-err"); ok {
		t.Error("ruleset-error branch planned on remote")
	}
	if _, ok := branchSweepPlanItem(m, BranchSideLocal, "console/closed-trunc"); !ok {
		t.Error("closed-trunc local B3 not planned")
	}

	trunkTip := e.commitOn("console/trunk", "trunk.txt")
	e.repo.Git("push", "-q", "origin", "console/trunk")
	e.repo.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/console/trunk")
	if got := opts.defaultBranch(context.Background()); got != "console/trunk" {
		t.Errorf("defaultBranch = %q, want console/trunk", got)
	}
	trunkAPI := &x11CandidateAPI{
		allRows: []RemotePR{{Number: 99, Branch: "console/trunk", State: "MERGED", HeadRefOid: trunkTip}},
	}
	trunkOpts := opts
	trunkOpts.API = trunkAPI
	if tm, err := PlanBranchSweep(context.Background(), trunkOpts); err != nil {
		t.Fatalf("PlanBranchSweep trunk: %v", err)
	} else if _, ok := branchSweepPlanItem(tm, BranchSideLocal, "console/trunk"); ok {
		t.Error("defaultBranch console/trunk was planned")
	}
	e.repo.Git("symbolic-ref", "--delete", "refs/remotes/origin/HEAD")

	ruleCache := map[string]bool{}
	if !opts.rulesetAllows(context.Background(), "console/trunc-ok", ruleCache) || !opts.rulesetAllows(context.Background(), "console/trunc-ok", ruleCache) {
		t.Error("rulesetAllows failed on initial or cached lookup")
	}

	var shadowLog bytes.Buffer
	shadowOpts := opts
	shadowOpts.Stage = RemoteStageShadow
	shadowOpts.Log = &shadowLog
	shadowManifest := BranchSweepManifest{
		Items: []BranchSweepItem{
			{Side: BranchSideLocal, Branch: "console/trunc-ok", Tip: truncOK, Class: "B1", Action: BranchAction("keep")},
			{Side: BranchSideLocal, Branch: "console/trunc-ok", Tip: truncOK, Class: "B1", Action: BranchActionDelete, Reason: "shadow"},
			{Side: BranchSideRemote, Branch: "console/trunc-ok", Tip: truncOK, Class: "B1", Action: BranchActionDelete, Reason: "shadow"},
		},
	}
	if err := ApplyBranchSweep(context.Background(), shadowOpts, shadowManifest); err != nil {
		t.Fatalf("shadow ApplyBranchSweep: %v", err)
	}
	if !strings.Contains(shadowLog.String(), "gc: local WOULD-DELETE") || !strings.Contains(shadowLog.String(), "gc: remote WOULD-DELETE") {
		t.Errorf("shadow log missing WOULD-DELETE: %q", shadowLog.String())
	}

	failRemoteOpts := opts
	failRemoteOpts.API = &x11CandidateAPI{deleteErr: errors.New("422")}
	failRemoteManifest := BranchSweepManifest{
		Items: []BranchSweepItem{
			{Side: BranchSideRemote, Branch: "console/trunc-ok", Tip: truncOK, Class: "B1", Action: BranchActionDelete, Reason: "delete"},
		},
	}
	if err := ApplyBranchSweep(context.Background(), failRemoteOpts, failRemoteManifest); !errors.Is(err, ErrRemoteDeleteFailed) {
		t.Errorf("ApplyBranchSweep remote delete err = %v, want ErrRemoteDeleteFailed", err)
	}

	if err := ApplyBranchSweep(context.Background(), opts, BranchSweepManifest{}); err != nil {
		t.Fatalf("empty ApplyBranchSweep: %v", err)
	}

	badOpen := opts
	badOpen.API = &x11CandidateAPI{openErr: errors.New("open boom")}
	if _, err := PlanBranchSweep(context.Background(), badOpen); err == nil {
		t.Fatal("PlanBranchSweep ignored openErr")
	}
	badAll := opts
	badAll.API = &x11CandidateAPI{allErr: errors.New("all boom")}
	if _, err := PlanBranchSweep(context.Background(), badAll); err == nil {
		t.Fatal("PlanBranchSweep ignored allErr")
	}
	badPlain := opts
	badPlain.API = &apiRS{prsErr: errors.New("prs boom")}
	if _, err := PlanBranchSweep(context.Background(), badPlain); err == nil {
		t.Fatal("PlanBranchSweep ignored PullRequests err")
	}

	moved := BranchSweepManifest{Items: []BranchSweepItem{{Side: BranchSideLocal, Branch: "console/trunc-ok", Tip: strings.Repeat("0", 40), Class: "B1", Action: BranchActionDelete}}}
	if err := ApplyBranchSweep(context.Background(), opts, moved); err == nil {
		t.Fatal("ApplyBranchSweep deleted moved tip")
	}

	heldPath := filepath.Join(t.TempDir(), "held")
	e.repo.Git("worktree", "add", "-q", heldPath, "console/trunc-ok")
	heldItem := BranchSweepManifest{Items: []BranchSweepItem{{Side: BranchSideLocal, Branch: "console/trunc-ok", Tip: truncOK, Class: "B1", Action: BranchActionDelete}}}
	if err := ApplyBranchSweep(context.Background(), opts, heldItem); err == nil {
		t.Fatal("ApplyBranchSweep deleted checked-out branch")
	}
	e.repo.Git("worktree", "remove", heldPath)

	badFileOpts := opts
	badFileOpts.EvolveDir = filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(badFileOpts.EvolveDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyBranchSweep(context.Background(), badFileOpts, heldItem); err == nil {
		t.Fatal("ApplyBranchSweep succeeded when evolve dir is a file")
	}

	failUpdateRef := opts
	failUpdateRef.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 0 && args[0] == "update-ref" {
			return errors.New("update-ref fail")
		}
		return nil
	})
	b3Item := BranchSweepManifest{Items: []BranchSweepItem{{Side: BranchSideLocal, Branch: "console/closed-trunc", Tip: closedTrunc, Class: "B3", Action: BranchActionDelete}}}
	if err := ApplyBranchSweep(context.Background(), failUpdateRef, b3Item); err == nil {
		t.Fatal("ApplyBranchSweep succeeded on update-ref error")
	}

	failArchiveVerify := opts
	failArchiveVerify.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--verify" && strings.HasPrefix(args[2], "refs/archive/") {
			return errors.New("verify fail")
		}
		return nil
	})
	if err := ApplyBranchSweep(context.Background(), failArchiveVerify, b3Item); err == nil {
		t.Fatal("ApplyBranchSweep succeeded on archive verify fail")
	}

	failArchiveMismatch := opts
	failArchiveMismatch.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 2 && args[0] == "rev-parse" && args[1] == "--verify" && strings.HasPrefix(args[2], "refs/archive/") {
			_, _ = io.WriteString(out, strings.Repeat("d", 40)+"\n")
			return 0, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if err := ApplyBranchSweep(context.Background(), failArchiveMismatch, b3Item); !errors.Is(err, errArchiveTipMismatch) {
		t.Fatalf("ApplyBranchSweep mismatch err = %v, want errArchiveTipMismatch", err)
	}

	failDelRef := opts
	failDelRef.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 1 && args[0] == "update-ref" && args[1] == "-d" {
			return errors.New("delete fail")
		}
		return nil
	})
	if err := ApplyBranchSweep(context.Background(), failDelRef, heldItem); err == nil {
		t.Fatal("ApplyBranchSweep succeeded on local update-ref -d fail")
	}

	failBundle := opts
	failBundle.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if len(args) > 0 && args[0] == "bundle" {
			return errors.New("bundle fail")
		}
		return nil
	})
	if err := ApplyBranchSweep(context.Background(), failBundle, heldItem); err == nil {
		t.Fatal("ApplyBranchSweep succeeded on bundle fail")
	}

	tsvDirOpts := opts
	tsvDirOpts.Stamp = "s-tsv-dir"
	if err := os.MkdirAll(filepath.Join(OperatorSalvageDir(opts.EvolveDir), "branch-gc", "s-tsv-dir.tsv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ApplyBranchSweep(context.Background(), tsvDirOpts, heldItem); err == nil {
		t.Fatal("ApplyBranchSweep succeeded when tsv path is a directory")
	}
}

func TestX11_GhRemoteAPIAndOwnWorkAndRemoteSweepLegacyPaths(t *testing.T) {
	var calls []string
	gh := ghRS(map[string]gitReplyF1{
		"gh pr": {out: `[{"number":5,"headRefName":"console/a","headRefOid":"1","state":"MERGED","closedAt":"2026-11-01T00:00:00Z"}]`},
	}, &calls)
	if prs, err := gh.PullRequests(context.Background()); err != nil || len(prs) != 1 || prs[0].Number != 5 {
		t.Fatalf("PullRequests = %+v, %v", prs, err)
	}
	if prs, trunc, err := gh.OpenPullRequests(context.Background()); err != nil || trunc || len(prs) != 1 || prs[0].Number != 5 {
		t.Fatalf("OpenPullRequests = %+v, trunc=%v, err=%v", prs, trunc, err)
	}
	if prs, trunc, err := gh.BranchPullRequests(context.Background(), "console/a"); err != nil || trunc || len(prs) != 1 || prs[0].Number != 5 {
		t.Fatalf("BranchPullRequests = %+v, trunc=%v, err=%v", prs, trunc, err)
	}
	if _, err := ghRS(map[string]gitReplyF1{"gh pr": {err: errors.New("boom")}}, nil).PullRequests(context.Background()); err == nil {
		t.Fatal("PullRequests ignored exec error")
	}
	if _, err := ghRS(map[string]gitReplyF1{"gh pr": {out: "{"}}, nil).PullRequests(context.Background()); err == nil {
		t.Fatal("PullRequests ignored json error")
	}

	repo := gittest.Fixture(t)
	commitRepoFile(t, repo, "a.txt", "a\n")
	head := repo.Git("rev-parse", "HEAD")
	g := gitexec.Git{Dir: repo.Dir, Exec: sysexec.DefaultRunner}
	if own, err := hasOwnWork(context.Background(), g, head); err != nil || own {
		t.Fatalf("single commit own = %v, %v; want false", own, err)
	}
	emptyDir := t.TempDir()
	if _, err := hasOwnWork(context.Background(), gitexec.Git{Dir: emptyDir, Exec: sysexec.DefaultRunner}, head); err == nil {
		t.Fatal("hasOwnWork succeeded on non-git dir")
	}
	if own, err := branchHasOwnWork(context.Background(), gitexec.Git{
		Dir: repo.Dir,
		Exec: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
			return 0, nil
		},
	}, "HEAD", head); err != nil || own {
		t.Fatalf("empty reflog own = %v, %v; want false", own, err)
	}

	replies := okRepliesRS()
	replies["git bundle list-heads"] = gitReplyF1{out: remoteTipRS + " refs/remotes/origin/console/b1\n"}
	ro := RemoteSweepOptions{
		ProjectRoot: t.TempDir(),
		EvolveDir:   t.TempDir(),
		Remote:      "origin",
		Stage:       RemoteStageEnforce,
		Exec:        remoteScriptRS(replies, nil),
		API:         &x11CandidateAPI{allRows: []RemotePR{{Branch: "console/b1", State: "MERGED", HeadRefOid: remoteTipRS}}, deleteErr: errors.New("422")},
		Log:         io.Discard,
	}
	applyErr := ApplyRemoteSweep(context.Background(), ro, RemoteManifest{Items: []RemoteItem{{Branch: "console/b1", Head: remoteTipRS, Action: RemoteActionDelete}}})
	if !errors.Is(applyErr, ErrRemoteDeleteFailed) {
		t.Fatalf("ApplyRemoteSweep err = %v, want ErrRemoteDeleteFailed", applyErr)
	}
	if _, ok := applyErr.(interface{ Unwrap() []error }); ok {
		var rde remoteDeleteError
		if errors.As(applyErr, &rde) && rde.Unwrap() == nil {
			t.Fatal("remoteDeleteError.Unwrap returned nil")
		}
	}
}
