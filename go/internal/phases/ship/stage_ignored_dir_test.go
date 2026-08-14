package ship

// stage_ignored_dir_test.go — layer 4 of the staging onion (after cycle-1098
// rc=128 absolute-pathspec, cycle-1101 rc=1 ignored-path, cycle-1108
// quotepath): the 2026-08-14 batch halt, fingerprint ship|unknown|99c38818.
//
// Defect, proven against real git: `git check-ignore` reports NOTHING for a
// DIRECTORY path (either slash form) when the ignore rule is the `dir/` form
// (.gitignore `.evolve/inbox/processed/`), so dropIgnoredPaths keeps the
// declared directory and `git add -A -- <dir>` refuses rc=1 ("The following
// paths are ignored…"). Three lanes hit the identical refusal → identical
// fingerprint ×3 → pipeline-blocker halt.
//
// The fix refuses to re-implement ignore semantics a third time: git's own
// refusal stderr NAMES the offending pathspecs verbatim — parse them, drop
// them (loudly), retry the add ONCE. Git stays the single source of truth for
// what is ignored, for every rule form, forever.

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestIgnoredPathsFromAddRefusal_ParsesTheOffenderList(t *testing.T) {
	t.Parallel()
	stderr := "The following paths are ignored by one of your .gitignore files:\n" +
		".evolve/inbox/processed\n" +
		".evolve/inbox/rejected\n" +
		"hint: Use -f if you really want to add them.\n" +
		"hint: Disable this message with \"git config set advice.addIgnoredFile false\"\n"
	got := ignoredPathsFromAddRefusal(stderr)
	if len(got) != 2 || got[0] != ".evolve/inbox/processed" || got[1] != ".evolve/inbox/rejected" {
		t.Fatalf("offenders = %v, want the two paths git named", got)
	}
}

func TestIgnoredPathsFromAddRefusal_NoHeaderMeansNoOffenders(t *testing.T) {
	t.Parallel()
	for _, stderr := range []string{
		"",
		"fatal: Invalid path '/go'\n",
		"hint: Use -f if you really want to add them.\n", // hint without header
	} {
		if got := ignoredPathsFromAddRefusal(stderr); len(got) != 0 {
			t.Errorf("stderr %q must yield no offenders, got %v — a fuzzy parse would silently under-stage unrelated failures", stderr, got)
		}
	}
}

// Quoted (non-ASCII) offender lines decode through the same unquoteGitPath the
// rest of the staging onion uses (cycle-1108 contract holds here too).
func TestIgnoredPathsFromAddRefusal_DecodesQuotedPaths(t *testing.T) {
	t.Parallel()
	stderr := "The following paths are ignored by one of your .gitignore files:\n" +
		`"caf\303\251-dir"` + "\n" +
		"hint: Use -f if you really want to add them.\n"
	got := ignoredPathsFromAddRefusal(stderr)
	if len(got) != 1 || got[0] != "café-dir" {
		t.Fatalf("quoted offender must decode to the on-disk path: %v", got)
	}
}

func TestIgnoredPathsFromAddRefusal_StopsAtTheFirstHint(t *testing.T) {
	t.Parallel()
	stderr := "The following paths are ignored by one of your .gitignore files:\n" +
		"real-offender\n" +
		"hint: Use -f if you really want to add them.\n" +
		"not-an-offender\n"
	got := ignoredPathsFromAddRefusal(stderr)
	if len(got) != 1 || got[0] != "real-offender" {
		t.Fatalf("parse must stop at the hint boundary: %v", got)
	}
}

// TestShipDirect_CycleClass_RetriesAfterGitNamesAnIgnoredPathspec — the
// behavioral crux of layer 4: a pathspec the check-ignore probe is BLIND to
// (directory-form rule) but git add refuses must not kill the ship. Git names
// it in the refusal; the stager drops exactly that, retries once, and the
// legit path lands. Modeled through the fake runner so the contract holds for
// ANY producer that puts a probe-blind path into the pathspec.
func TestShipDirect_CycleClass_RetriesAfterGitNamesAnIgnoredPathspec(t *testing.T) {
	root := stageExplicitTree(t)
	blind := ".evolve/inbox/processed"
	ws := writeWorkspaceReports(t,
		"go/internal/phases/ship/gitops.go",
		blind)
	cap := &porcelainCapture{
		// The blind path arrives via the changed set (mirrors the live shape:
		// upstream filters passed it; only git's own add refused it).
		porcelain:      " M go/internal/phases/ship/gitops.go\n?? " + blind + "\n",
		refuseAddPaths: []string{blind},
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect must survive a probe-blind ignored pathspec via git's own refusal: %v (2026-08-14 ship|unknown|99c38818 class)", err)
	}

	var adds [][]string
	for _, call := range cap.calls {
		if call[0] == "git" && slices.Contains(call, "add") {
			adds = append(adds, call)
		}
	}
	if len(adds) != 2 {
		t.Fatalf("want exactly ONE retry after the refusal (2 add calls), got %d: %v", len(adds), adds)
	}
	if !slices.Contains(adds[0], blind) {
		t.Fatalf("first add must have carried the blind path (that IS the refusal): %v", adds[0])
	}
	if slices.Contains(adds[1], blind) {
		t.Errorf("retry still carries the refused path — no progress, infinite refusal: %v", adds[1])
	}
	if !slices.Contains(adds[1], "go/internal/phases/ship/gitops.go") {
		t.Errorf("legit path missing from the retry: %v", adds[1])
	}
	var loud bool
	for _, l := range res.Logs {
		if strings.Contains(l, "refused") && strings.Contains(l, blind) {
			loud = true
		}
	}
	if !loud {
		t.Errorf("the git-named drop must be LOUD in ship logs; logs=%v", res.Logs)
	}
}

// A refusal whose named offenders match NOTHING in the pathspec (form
// mismatch, foreign path) must fail honestly after exactly ONE add — the
// progress guard, pinned so it cannot be simplified away into a doomed
// identical retry.
func TestShipDirect_CycleClass_ForeignOffenderMeansNoRetry(t *testing.T) {
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
	cap := &porcelainCapture{
		porcelain:       " M go/internal/phases/ship/gitops.go\n",
		refuseAddAlways: ".evolve/somewhere/else",
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())
	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err == nil {
		t.Fatal("a refusal naming only foreign paths must stay an honest ship error")
	}
	var adds int
	for _, call := range cap.calls {
		if call[0] == "git" && slices.Contains(call, "add") {
			adds++
		}
	}
	if adds != 1 {
		t.Fatalf("no progress possible ⇒ exactly ONE add call (no doomed identical retry), got %d", adds)
	}
}
