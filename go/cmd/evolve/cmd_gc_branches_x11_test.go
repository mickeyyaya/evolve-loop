package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestX11_CheckPlansTheMergedAndTheClosedBranchAndKeepsEveryOther(t *testing.T) {
	w := newWorldX11(t)
	w.standard(true)
	w.publish()

	rep := w.plan()

	requireDeletedByClassX11(t, rep, "console/merged", "B1")
	requireDeletedByClassX11(t, rep, "console/closed-old", "B3")
	requireKeptX11(t, rep, "console/closed-young", "console/open", "console/checked-out", "stray/unowned", "cycle-x11-merged", "cycle-x11-closed", "console/empty-new", "main")
	if calls := w.ghCalls(); strings.Contains(calls, "--head") {
		t.Errorf("a lookup under its limit must not fall back to a per-branch lookup:\n%s", calls)
	}
	for _, b := range []string{"console/merged", "console/closed-old", "console/open", "stray/unowned"} {
		if !w.localExists(b) || w.remoteTip(b) == "" {
			t.Errorf("the plan verb must delete nothing: %s is gone (local=%v)", b, w.localExists(b))
		}
	}
}

func TestX11_CheckPrintsTheAllowListWithTipPullRequestAndClass(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.publish()

	rc, out, errOut := w.run("check", "--only", "branches")
	if rc != cliExitOK {
		t.Fatalf("rc=%d stderr=%s", rc, errOut)
	}
	prNumber := regexp.MustCompile(`(^|[^0-9a-fA-F])#?11([^0-9a-fA-F]|$)`)
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "console/merged") || !strings.Contains(strings.ToUpper(line), "DELETE") {
			continue
		}
		if !strings.Contains(line, w.tips["console/merged"][:12]) || !strings.Contains(strings.ToUpper(line), "B1") || !prNumber.MatchString(line) {
			t.Errorf("an allow-list line must carry the tip, the pull request and the class: %q", line)
		}
		return
	}
	t.Errorf("no delete line for console/merged in the text report:\n%s", out)
}

func TestX11_CheckOnlyTakesTheNamesOfTheRegistryAndGCOnlyKeepsItsScopes(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.publish()

	if rc, _, errOut := w.run("check", "--only", "bogus"); rc != cliExitUsage || !strings.Contains(errOut, "bogus") || !strings.Contains(errOut, "branches") {
		t.Errorf("an unknown check name: rc=%d stderr=%q, want usage naming the valid checks", rc, errOut)
	}
	if got := w.checkNames("check", "--only", "worktrees", "--json"); !slices.Equal(got, []string{"worktrees"}) {
		t.Errorf("--only worktrees ran %v", got)
	}
	if got := w.checkNames("check", "--json"); !slices.Contains(got, "branches") || !slices.Contains(got, "worktrees") || !slices.Contains(got, "temp") {
		t.Errorf("no --only must run every registered check, ran %v", got)
	}
	if rc, _, errOut := w.run("--only", "branches", "--dry-run"); rc != cliExitUsage || !strings.Contains(errOut, "unknown --only scope") {
		t.Errorf("evolve gc --only keeps its sweep scopes: rc=%d stderr=%q", rc, errOut)
	}
}

func TestX11_ACycleBranchIsNeverPlannedByThisCheck(t *testing.T) {
	w := newWorldX11(t)
	w.standard(true)
	w.branch("cycle-x11-landed", false)
	w.hub.Git("merge", "-q", "--no-ff", "-m", "land", "cycle-x11-landed")
	w.hub.Git("push", "-q", "origin", "main")
	w.publish()

	rep := w.plan()

	for _, it := range rep.Checks[0].Items {
		if strings.Contains(it.Subject, "cycle-") {
			t.Errorf("a cycle-* branch appears in the allow-list: %+v", it)
		}
	}
	requireKeptX11(t, rep, "cycle-x11-merged", "cycle-x11-closed", "cycle-x11-landed")
}

func TestX11_AClosedBranchIsPlannedPastTheGraceAndKeptBefore(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.branch("console/closed-2d", true)
	w.pr("console/closed-2d", "CLOSED", 48*time.Hour)
	w.branch("console/closed-4d", true)
	w.pr("console/closed-4d", "CLOSED", 96*time.Hour)
	w.publish()

	rep := w.plan()
	requireDeletedByClassX11(t, rep, "console/closed-4d", "B3")
	requireDeletedByClassX11(t, rep, "console/closed-old", "B3")
	requireKeptX11(t, rep, "console/closed-2d", "console/closed-young")

	w.setPolicy(map[string]any{"branches": map[string]any{"closed_grace_days": 1}}, false)
	rep = w.plan()
	requireDeletedByClassX11(t, rep, "console/closed-2d", "B3")
	requireDeletedByClassX11(t, rep, "console/closed-young", "B3")
}

func TestX11_AClosedBranchThatMovedPastItsPullRequestIsKept(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.branch("console/closed-moved", true)
	w.pr("console/closed-moved", "CLOSED", 10*24*time.Hour)
	moved := w.advance("console/closed-moved")
	w.publish()

	rep := w.plan()

	requireKeptX11(t, rep, "console/closed-moved")
	requireDeletedByClassX11(t, rep, "console/closed-old", "B3")
	w.setPolicy(nil, true)
	if rc, _, errOut := w.run(); rc != cliExitOK {
		t.Fatalf("evolve gc rc=%d stderr=%s", rc, errOut)
	}
	if !w.localExists("console/closed-moved") || w.remoteTip("console/closed-moved") != moved {
		t.Errorf("evolve gc deleted or moved a closed branch that is past its pull request")
	}
}

func TestX11_ABranchWhoseTipMovesAfterThePlanStaysAfterGC(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.publish()
	planned := w.plan()
	requireDeletedByClassX11(t, planned, "console/merged", "B1")
	moved := w.advance("console/merged")
	w.setPolicy(nil, true)

	if rc, _, errOut := w.run(); rc != cliExitOK {
		t.Fatalf("evolve gc rc=%d stderr=%s", rc, errOut)
	}

	if !w.localExists("console/merged") || w.remoteTip("console/merged") != moved {
		t.Errorf("a branch whose tip moved after the plan must stay: local=%v remote=%q want %s", w.localExists("console/merged"), w.remoteTip("console/merged"), moved)
	}
	if w.localExists("console/closed-old") || w.remoteTip("console/closed-old") != "" {
		t.Errorf("evolve gc did not apply the plan for the branch that did not move")
	}
}

func TestX11_TheDefaultBranchThatOriginHeadNamesStays(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.branch("console/trunk", true)
	w.pr("console/trunk", "MERGED", 24*time.Hour)
	w.pointOriginHeadAt("console/trunk")
	w.publish()

	requireKeptX11(t, w.plan(), "console/trunk", "main")
	w.setPolicy(nil, true)
	if rc, _, errOut := w.run(); rc != cliExitOK {
		t.Fatalf("evolve gc rc=%d stderr=%s", rc, errOut)
	}

	if !w.localExists("console/trunk") || w.remoteTip("console/trunk") != w.tips["console/trunk"] {
		t.Errorf("the branch that origin/HEAD names was deleted")
	}
	if w.localExists("console/merged") || w.remoteTip("console/merged") != "" {
		t.Errorf("evolve gc did not sweep the landed branch next to the default branch")
	}
}

func TestX11_AFullPullRequestLookupFallsBackToOneLookupPerCandidate(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.publish()
	w.truncateLookups()

	rep := w.plan()

	calls := w.ghCalls()
	for _, b := range []string{"console/merged", "console/closed-old", "console/open"} {
		if !strings.Contains(calls, "--head "+b) {
			t.Errorf("no per-branch lookup for the candidate %s:\n%s", b, calls)
		}
	}
	if !strings.Contains(calls, "--state all") {
		t.Errorf("the per-branch lookup must ask for every state:\n%s", calls)
	}
	requireDeletedByClassX11(t, rep, "console/merged", "B1")
	requireDeletedByClassX11(t, rep, "console/closed-old", "B3")
	requireKeptX11(t, rep, "console/open", "stray/unowned", "console/closed-young")
}

func TestX11_GCRecordsEachDeleteAndThePrintedRestoreRebuildsBothSides(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.publish()
	w.setPolicy(nil, true)

	rc, out, errOut := w.run()
	if rc != cliExitOK {
		t.Fatalf("evolve gc rc=%d stderr=%s", rc, errOut)
	}
	printed := out + errOut

	for _, b := range []string{"console/merged", "console/closed-old"} {
		if w.localExists(b) || w.remoteTip(b) != "" {
			t.Fatalf("%s survived evolve gc", b)
		}
	}
	for _, b := range []string{"console/open", "console/closed-young", "stray/unowned", "console/checked-out", "console/empty-new"} {
		if !w.localExists(b) {
			t.Errorf("evolve gc deleted the kept branch %s", b)
		}
	}
	mergedTip, closedTip := w.tips["console/merged"], w.tips["console/closed-old"]
	requireTsvRowsX11(t, w, "console/merged", mergedTip, "B1")
	requireBundleHoldingX11(t, w, mergedTip)
	requireArchiveRefX11(t, w, "console/closed-old", closedTip)

	for _, b := range []string{"console/merged", "console/closed-old"} {
		tip := w.tips[b]
		local := "git branch " + b + " " + tip
		remote := "git push origin " + tip + ":refs/heads/" + b
		for _, cmd := range []string{local, remote} {
			if !strings.Contains(printed, cmd) {
				t.Fatalf("the output prints no restore command %q:\n%s", cmd, printed)
			}
		}
		w.hub.Git(strings.Fields(local)[1:]...)
		w.hub.Git(strings.Fields(remote)[1:]...)
		if got := w.hub.Git("rev-parse", "refs/heads/"+b); got != tip {
			t.Errorf("restored local %s at %s, want %s", b, got, tip)
		}
		if got := w.remoteTip(b); got != tip {
			t.Errorf("restored remote %s at %s, want %s", b, got, tip)
		}
	}
}

func requireTsvRowsX11(t *testing.T, w *worldX11, branch, tip, class string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(w.evolveDir, "operator-salvage", "branch-gc", "*.tsv"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no branch-gc tsv record under operator-salvage (err=%v)", err)
	}
	sides := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			cols := strings.Split(line, "\t")
			if len(cols) >= 5 && cols[1] == branch && cols[2] == tip && strings.EqualFold(cols[4], class) {
				sides[cols[0]] = true
			}
		}
	}
	if len(sides) != 2 {
		t.Errorf("the tsv record needs one side,branch,tip,pr,class row for each of the local and the remote delete of %s, got sides %v", branch, sides)
	}
}

func requireBundleHoldingX11(t *testing.T, w *worldX11, tip string) {
	t.Helper()
	var bundles []string
	root := filepath.Join(w.evolveDir, "operator-salvage", "branch-gc")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".bundle") {
			bundles = append(bundles, path)
		}
		return nil
	})
	for _, b := range bundles {
		w.hub.Git("bundle", "verify", b)
		if strings.Contains(w.hub.Git("bundle", "list-heads", b), tip) {
			return
		}
	}
	t.Errorf("no verified bundle under branch-gc/<stamp>/ holds the tip %s of a branch that is ahead of origin/main (found %v)", tip, bundles)
}

func requireArchiveRefX11(t *testing.T, w *worldX11, branch, tip string) {
	t.Helper()
	refs := w.hub.Git("for-each-ref", "--format=%(refname) %(objectname)", "refs/archive/branches/")
	want := regexp.MustCompile(`^refs/archive/branches/[^/ ]+/` + regexp.QuoteMeta(branch) + ` ` + tip + `$`)
	for _, line := range strings.Split(refs, "\n") {
		if want.MatchString(line) {
			return
		}
	}
	t.Errorf("no refs/archive/branches/<stamp>/%s at %s, refs: %q", branch, tip, refs)
}

func TestX11_ShadowPrintsWouldDeleteAndDeletesNothing(t *testing.T) {
	for name, run := range map[string]struct {
		enforce bool
		args    []string
	}{
		"stage absent":         {false, nil},
		"dry run over enforce": {true, []string{"--dry-run"}},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorldX11(t)
			w.standard(false)
			w.publish()
			w.setPolicy(nil, run.enforce)

			rc, out, errOut := w.run(run.args...)
			if rc != cliExitOK {
				t.Fatalf("rc=%d stderr=%s", rc, errOut)
			}

			for _, b := range []string{"console/merged", "console/closed-old"} {
				if !wouldDeleteLineX11(out+errOut, b) {
					t.Errorf("no WOULD-DELETE line for %s:\n%s", b, out)
				}
				if !w.localExists(b) || w.remoteTip(b) != w.tips[b] {
					t.Errorf("shadow deleted %s", b)
				}
			}
			for _, b := range []string{"console/open", "console/closed-young", "stray/unowned", "console/checked-out", "console/empty-new"} {
				if wouldDeleteLineX11(out+errOut, b) {
					t.Errorf("WOULD-DELETE line for the kept branch %s", b)
				}
			}
			if left, _ := filepath.Glob(filepath.Join(w.evolveDir, "operator-salvage", "branch-gc", "*")); len(left) != 0 {
				t.Errorf("shadow wrote a record: %v", left)
			}
			if refs := w.hub.Git("for-each-ref", "refs/archive/branches/"); refs != "" {
				t.Errorf("shadow wrote an archive ref: %s", refs)
			}
		})
	}
}

func wouldDeleteLineX11(output, branch string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "WOULD-DELETE") && strings.Contains(line, branch) {
			return true
		}
	}
	return false
}

func TestX11_AFailedRemoteDeleteIsAWarnAndTheExitCodeStays(t *testing.T) {
	baseline := newWorldX11(t)
	baseline.standard(false)
	baseline.publish()
	baseline.setPolicy(nil, true)
	wantRC, _, _ := baseline.run()

	w := newWorldX11(t)
	w.standard(false)
	w.publish()
	w.setPolicy(nil, true)
	w.failRemoteDeleteOf("console/merged")

	rc, _, errOut := w.run()

	if rc != wantRC {
		t.Errorf("a failed remote delete changed the gc exit code: rc=%d, baseline=%d\n%s", rc, wantRC, errOut)
	}
	warned := false
	for _, line := range strings.Split(errOut, "\n") {
		if strings.Contains(line, "WARN") && strings.Contains(line, "console/merged") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("no WARN line names console/merged:\n%s", errOut)
	}
	if w.remoteTip("console/merged") == "" {
		t.Errorf("the failing remote delete removed the branch anyway")
	}
	if w.remoteTip("console/closed-old") != "" {
		t.Errorf("one failed remote delete stopped the next one")
	}
}

func TestX11_B2PlansALandedBranchWithOwnWorkAndKeepsAnEmptyOneAndARemoteOnlyOne(t *testing.T) {
	w := newWorldX11(t)
	w.branch("console/landed-b2", false)
	w.hub.Git("merge", "-q", "--no-ff", "-m", "land b2", "console/landed-b2")
	w.branch("console/remote-only-landed", true)
	w.hub.Git("merge", "-q", "--no-ff", "-m", "land remote only", "console/remote-only-landed")
	w.hub.Git("branch", "-D", "console/remote-only-landed")
	w.hub.Git("push", "-q", "origin", "main")
	w.hub.Git("branch", "console/empty-new", "main~2")
	w.hub.Git("remote", "set-head", "origin", "main")
	w.publish()

	rep := w.plan()

	requireDeletedByClassX11(t, rep, "console/landed-b2", "B2")
	requireKeptX11(t, rep, "console/empty-new", "console/remote-only-landed")
}

func TestX11_OwnedPrefixesComeFromPolicy(t *testing.T) {
	w := newWorldX11(t)
	w.standard(false)
	w.publish()
	w.setPolicy(map[string]any{"branches": map[string]any{"owned_prefixes": []string{"stray/"}}}, false)

	rep := w.plan()

	requireDeletedByClassX11(t, rep, "stray/unowned", "B1")
	requireKeptX11(t, rep, "console/merged", "console/closed-old")
}
