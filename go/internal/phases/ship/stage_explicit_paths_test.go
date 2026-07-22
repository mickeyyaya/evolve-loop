// stage_explicit_paths_test.go — RED contract for inbox item
// `ship-stage-explicit-paths` (cycle-1067).
//
// Defect: shipDirect (gitops.go:228) and shipFromWorktree (gitops.go:374)
// stage with `git add -A` for the non-release classes. That sweeps whatever
// happens to be dirty in the tree — under a fleet, typically a sibling lane's
// untracked leak (cycle-645) — into the ship commit, and it violates the
// standing repo convention `git_add_explicit_paths`. The release class already
// does the right thing (stageReleaseSet, gitops.go:707: `git add -- <paths>`);
// the cycle/manual paths were simply never migrated.
//
// Contract pinned here:
//  1. Neither shipDirect nor shipFromWorktree invokes `git add -A` for
//     ClassCycle / ClassManual — staging is an explicit `git add -- <paths>`.
//  2. The staged path list is the DECLARED manifest (build-report.md +
//     test-report.md, the set declaredManifest already computes for the
//     manifest gate) when the workspace has readable phase reports.
//  3. When no manifest is readable (no workspace, or no reports), staging
//     falls back to the porcelain-status changed set — it must NOT silently
//     skip staging (that would produce a false clean-exit / empty ship) and it
//     must NOT fall back to `add -A`.
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// porcelainCapture is stagingCapture plus canned `git status --porcelain`
// output, so the manifest-empty fallback has a changed set to discover.
// `git diff --cached --quiet` reports staged changes present (exit 1) so
// shipDirect proceeds past its clean-exit check.
type porcelainCapture struct {
	calls     [][]string
	porcelain string
}

func (c *porcelainCapture) runner() CmdRunner {
	return func(_ context.Context, name, _ string, args, _ []string,
		_ io.Reader, stdout, _ io.Writer) (int, error) {
		c.calls = append(c.calls, append([]string{name}, args...))
		if name != "git" {
			return 0, nil
		}
		switch {
		case slices.Contains(args, "diff") && slices.Contains(args, "--cached") &&
			slices.Contains(args, "--quiet"):
			return 1, nil // staged changes exist
		case slices.Contains(args, "status") && slices.Contains(args, "--porcelain"):
			if stdout != nil {
				_, _ = io.WriteString(stdout, c.porcelain)
			}
			return 0, nil
		}
		return 0, nil
	}
}

// gitCallWith returns the first recorded git call whose args contain every
// needle, or nil.
// unscopedAddAll reports an `add -A` invocation WITHOUT an explicit pathspec
// after `--` — the banned repo-wide sweep. `add -A -- <paths>` is the allowed
// scoped form (stages adds/mods/deletions within the named paths only; plain
// `add -- <staged-deleted>` is fatal rc=128 — the operator boundary-flow pin).
func (c *porcelainCapture) unscopedAddAll() []string {
	for _, call := range c.calls {
		if len(call) < 2 || call[0] != "git" {
			continue
		}
		args := call[1:]
		hasAdd, hasA, pathAfterDashDash := false, false, false
		for i, a := range args {
			switch a {
			case "add":
				hasAdd = true
			case "-A", "--all":
				hasA = true
			case "--":
				if i+1 < len(args) {
					pathAfterDashDash = true
				}
			}
		}
		if hasAdd && hasA && !pathAfterDashDash {
			return call
		}
	}
	return nil
}

func (c *porcelainCapture) gitCallWith(needles ...string) []string {
	for _, call := range c.calls {
		if call[0] != "git" {
			continue
		}
		ok := true
		for _, n := range needles {
			if !slices.Contains(call[1:], n) {
				ok = false
				break
			}
		}
		if ok {
			return call
		}
	}
	return nil
}

// addPathspec returns the pathspec arguments of the first `git add` call
// (everything after the `--` separator), and whether an add call was seen.
func (c *porcelainCapture) addPathspec() ([]string, bool) {
	for _, call := range c.calls {
		if call[0] != "git" || !slices.Contains(call[1:], "add") {
			continue
		}
		for i, a := range call {
			if a == "--" {
				return call[i+1:], true
			}
		}
		return nil, true // an add call with no `--` separator (e.g. add -A)
	}
	return nil, false
}

// writeWorkspaceReports lays down a workspace whose phase reports declare
// exactly the given paths, and returns the workspace directory.
func writeWorkspaceReports(t *testing.T, declared ...string) string {
	t.Helper()
	ws := t.TempDir()
	var b strings.Builder
	b.WriteString("# Build Report\n\n## Files Changed\n\n")
	for _, p := range declared {
		b.WriteString("- `" + p + "`\n")
	}
	mustWrite(t, filepath.Join(ws, "build-report.md"), b.String())
	mustWrite(t, filepath.Join(ws, "test-report.md"), "# TDD Report\n\nno additional paths\n")
	return ws
}

// stageExplicitTree lays out a tree with two declared files plus an undeclared
// untracked stray (the cross-lane leak `add -A` currently sweeps in).
func stageExplicitTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		"go/internal/phases/ship/gitops.go",
		"go/internal/phases/ship/manifest.go",
		"sibling-lane-leak.txt", // undeclared: must never be staged
	} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "content of "+rel)
	}
	return root
}

func stageExplicitOpts(root, workspace string, class Class, runner CmdRunner) *Options {
	return &Options{
		Class:         class,
		ProjectRoot:   root,
		PluginRoot:    root,
		WorkspacePath: workspace,
		CommitMessage: "feat: explicit staging",
		Runner:        runner,
		Stderr:        io.Discard,
	}
}

// TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll — the crux: a cycle
// ship stages exactly the paths the phase reports declared, via an explicit
// `git add -- <paths>`, and leaves the undeclared sibling-lane stray alone.
func TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll(t *testing.T) {
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t,
		"go/internal/phases/ship/gitops.go",
		"go/internal/phases/ship/manifest.go")
	cap := &porcelainCapture{porcelain: " M go/internal/phases/ship/gitops.go\n?? sibling-lane-leak.txt\n"}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}

	if call := cap.unscopedAddAll(); call != nil {
		t.Errorf("RED: cycle ship staged with an UNSCOPED `git add -A` (%v) — must stage the declared path set explicitly (git_add_explicit_paths); `-A -- <paths>` is the allowed SCOPED sweep (staged-deletion handling)", call)
	}
	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd {
		t.Fatal("cycle ship never invoked git add at all — staging must never be silently skipped")
	}
	for _, want := range []string{
		"go/internal/phases/ship/gitops.go",
		"go/internal/phases/ship/manifest.go",
	} {
		if !slices.Contains(pathspec, want) {
			t.Errorf("declared path %q missing from staging pathspec %v", want, pathspec)
		}
	}
	if slices.Contains(pathspec, "sibling-lane-leak.txt") {
		t.Errorf("undeclared stray staged: %v — explicit staging must bind only declared paths", pathspec)
	}
}

// TestShipDirect_ManualClass_EmptyManifestFallsBackToChangedSet — H2: with no
// readable phase reports the declared manifest is empty; staging must fall
// back to the porcelain changed set, never to `add -A` and never to nothing
// (a silent skip produces a false clean exit / empty ship).
func TestShipDirect_ManualClass_EmptyManifestFallsBackToChangedSet(t *testing.T) {
	root := stageExplicitTree(t)
	emptyWS := t.TempDir() // no build-report.md / test-report.md
	cap := &porcelainCapture{porcelain: " M go/internal/phases/ship/gitops.go\n?? docs/new-note.md\n"}
	opts := stageExplicitOpts(root, emptyWS, ClassManual, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(manual): %v", err)
	}

	if call := cap.unscopedAddAll(); call != nil {
		t.Errorf("RED: manifest-empty fallback used `git add -A` (%v) — must fall back to the porcelain changed set", call)
	}
	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd {
		t.Fatal("manifest-empty ship never invoked git add — staging must not be silently skipped (false clean exit)")
	}
	if len(pathspec) == 0 {
		t.Fatal("manifest-empty fallback staged an EMPTY pathspec — that ships nothing while reporting success")
	}
	for _, want := range []string{"go/internal/phases/ship/gitops.go", "docs/new-note.md"} {
		if !slices.Contains(pathspec, want) {
			t.Errorf("changed path %q missing from fallback pathspec %v", want, pathspec)
		}
	}
}

// TestShipDirect_NoWorkspacePath_StillStagesExplicitly — edge case: an
// operator manual ship carries no WorkspacePath at all. The manifest cannot
// even be attempted; staging must still be explicit and non-empty.
func TestShipDirect_NoWorkspacePath_StillStagesExplicitly(t *testing.T) {
	root := stageExplicitTree(t)
	cap := &porcelainCapture{porcelain: " M README.md\n"}
	opts := stageExplicitOpts(root, "", ClassManual, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(manual, no workspace): %v", err)
	}

	if call := cap.unscopedAddAll(); call != nil {
		t.Errorf("RED: workspace-less manual ship staged with `git add -A` (%v)", call)
	}
	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd || len(pathspec) == 0 {
		t.Fatalf("workspace-less manual ship must still stage the changed set explicitly (sawAdd=%v pathspec=%v)", sawAdd, pathspec)
	}
	if !slices.Contains(pathspec, "README.md") {
		t.Errorf("changed path README.md missing from pathspec %v", pathspec)
	}
}

// TestShipDirect_NonReleaseClasses_NeverAddAll — the anti-no-op negative: for
// EVERY non-release class, `git add -A` must be absent from the recorded git
// calls. A partial fix that migrates only the cycle path fails here.
func TestShipDirect_NonReleaseClasses_NeverAddAll(t *testing.T) {
	for _, class := range []Class{ClassCycle, ClassManual, ClassTrivial} {
		t.Run(string(class), func(t *testing.T) {
			root := stageExplicitTree(t)
			ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
			cap := &porcelainCapture{porcelain: " M go/internal/phases/ship/gitops.go\n"}
			opts := stageExplicitOpts(root, ws, class, cap.runner())

			if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
				t.Fatalf("shipDirect(%s): %v", class, err)
			}
			if call := cap.unscopedAddAll(); call != nil {
				t.Errorf("class %s still stages with an UNSCOPED `git add -A`: %v", class, call)
			}
			if _, sawAdd := cap.addPathspec(); !sawAdd {
				t.Errorf("class %s never staged anything", class)
			}
		})
	}
}

// TestShipDirect_CycleClass_KeepsChurnDiscard — the surviving half of the
// former TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll discriminator
// (release_staging_test.go): the `add -A` half is gone with this cycle's fix,
// but the churn-discard half still guards cycle commits against unaudited
// binary churn, so it is pinned here rather than deleted.
func TestShipDirect_CycleClass_KeepsChurnDiscard(t *testing.T) {
	root := initReleaseStagingTree(t)
	ws := writeWorkspaceReports(t, "go/evolve", "CHANGELOG.md")
	cap := &porcelainCapture{porcelain: " M CHANGELOG.md\n"}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())
	opts.ShipBinaryPath = filepath.Join(root, "go", "evolve")

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}
	// With the capture runner reporting go/evolve as untracked (empty ls-files
	// output), discardBinaryChurn removes the file — the observable proof the
	// discard still runs for cycle ships.
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); !os.IsNotExist(err) {
		t.Error("cycle ship skipped discardBinaryChurn — unaudited binary churn would ride into cycle commits")
	}
}
