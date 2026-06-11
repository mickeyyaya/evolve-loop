// release_staging_test.go — RED contract for inbox defects
// release-rebuild-binary-not-committed (2026-06-10T10-50Z, recurred v18.3.0 →
// v18.5.0) and release-stage-sweeps-untracked-root-logs (2026-06-10T10-51Z).
//
// Root cause (v18.5.0 forensic, commit d93e9f02): shipDirect runs
// discardBinaryChurn before `git add -A` for EVERY class. For cycle/manual
// that is correct (unaudited binary churn must not ride along); for
// --class release it throws away the binary the pipeline's rebuild-binary
// step produced ONE STEP EARLIER, so every release commit ships without
// go/evolve and the freshly-pinned expected_ship_sha guarantees
// SELF_SHA_TAMPERED on the next ship. Meanwhile `git add -A` sweeps
// untracked operator files (evolve.log, release-*.log) into release commits.
//
// Contract pinned here:
//  1. ClassRelease staging is an EXPLICIT pathspec — the versionbump marker
//     set (SSOT: versionbump.DefaultPaths) + CHANGELOG.md + the tracked
//     binary — never `add -A`, and never a churn discard of go/evolve.
//  2. ClassCycle keeps today's behavior byte-for-byte: churn discard, then
//     `git add -A` (the discriminator that keeps the fix class-scoped).
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

// stagingCapture records every git invocation; `git diff --cached --quiet`
// reports staged changes present (exit 1) so shipDirect proceeds, everything
// else succeeds with empty output.
type stagingCapture struct {
	calls [][]string
}

func (c *stagingCapture) runner() CmdRunner {
	return func(_ context.Context, name string, args, _ []string, _ string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		c.calls = append(c.calls, append([]string{name}, args...))
		if name == "git" && len(args) >= 3 && args[0] == "diff" && args[1] == "--cached" && args[2] == "--quiet" {
			return 1, nil // staged changes exist
		}
		return 0, nil
	}
}

// gitCallWith returns the first recorded git call whose args contain every
// needle, or nil.
func (c *stagingCapture) gitCallWith(needles ...string) []string {
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

// initReleaseStagingTree lays out the release file set plus an untracked
// stray log (the v18.4.0 sweep victim) and the freshly rebuilt binary.
func initReleaseStagingTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		"skills/loop/SKILL.md",
		"README.md",
		"CHANGELOG.md",
		"go/evolve",
		"evolve.log", // untracked stray — must never be staged in a release
	} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "content of "+rel)
	}
	return root
}

func releaseStagingOpts(root string, class Class, runner CmdRunner) *Options {
	return &Options{
		Class:          class,
		ProjectRoot:    root,
		PluginRoot:     root,
		CommitMessage:  "release: v9.9.9",
		Runner:         runner,
		Stderr:         io.Discard,
		ShipBinaryPath: filepath.Join(root, "go", "evolve"),
	}
}

// TestShipDirect_ReleaseClass_StagesExplicitSetKeepsBinary — RED today:
// shipDirect discards the rebuilt binary and stages with `add -A` for every
// class. The release class must stage exactly the known release set.
func TestShipDirect_ReleaseClass_StagesExplicitSetKeepsBinary(t *testing.T) {
	cap := &stagingCapture{}
	root := initReleaseStagingTree(t)
	opts := releaseStagingOpts(root, ClassRelease, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(release): %v", err)
	}

	// 1. No churn discard of the freshly rebuilt binary.
	if call := cap.gitCallWith("checkout", "--", "go/evolve"); call != nil {
		t.Errorf("RED (v18.5.0): release ship discarded the rebuilt binary via %v — the rebuild-binary step is its audited producer; the release commit must include it", call)
	}
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); err != nil {
		t.Errorf("rebuilt binary removed from disk during release ship: %v", err)
	}

	// 2. Staging is the explicit release set, never add -A.
	if call := cap.gitCallWith("add", "-A"); call != nil {
		t.Errorf("RED (v18.4.0 sweep): release ship staged with `git add -A` (%v) — untracked operator files ride into release commits; must stage the explicit release set", call)
	}
	addCall := cap.gitCallWith("add")
	if addCall == nil {
		t.Fatal("release ship never invoked git add at all")
	}
	joined := strings.Join(addCall, " ")
	for _, want := range []string{
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		"skills/loop/SKILL.md",
		"README.md",
		"CHANGELOG.md",
		"go/evolve",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("release staging missing %s (got: %v)", want, addCall)
		}
	}
	if strings.Contains(joined, "evolve.log") {
		t.Errorf("release staging swept the untracked stray evolve.log: %v", addCall)
	}
}

// TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll — the discriminator:
// cycle ships keep today's exact behavior (churn discard + add -A), so the
// release fix cannot leak unaudited binaries into cycle commits.
func TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll(t *testing.T) {
	cap := &stagingCapture{}
	root := initReleaseStagingTree(t)
	opts := releaseStagingOpts(root, ClassCycle, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}

	if call := cap.gitCallWith("add", "-A"); call == nil {
		t.Error("cycle ship must keep `git add -A` staging")
	}
	// The churn-discard path engaged: with the capture runner reporting
	// go/evolve as untracked (empty ls-files output), discardBinaryChurn
	// removes the file — the observable proof the discard ran for cycle.
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); !os.IsNotExist(err) {
		t.Error("cycle ship skipped discardBinaryChurn — unaudited binary churn would ride into cycle commits")
	}
}
