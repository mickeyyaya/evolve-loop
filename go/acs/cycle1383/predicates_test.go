//go:build acs

// Package cycle1383 materializes the acceptance criteria of this lane's sole
// fleet-scoped todo-id `triage-protected-surface-admission-wire-verdict`
// (triage top_n — "retire-stale-protected-surface-admission-inbox-item").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout verified the substantive fix the inbox item asks for already shipped
// in cycle-1312 (commit 0d07b200): `go/internal/phases/triage/triage.go`'s
// `Classify` runs `protectedTopNViolation` (triage.go:311), a second,
// commit-time admission check that FAILs any `## top_n` card whose
// `files=` / `files={}` segment names a `guards.IsProtectedSurface` path —
// independent of the prompt-side `inboxbatch.PartitionConsole` screen.
// `go/internal/phases/triage/protected_surface_admission_test.go` (5 tests)
// pins that contract and is GREEN; §F4 of
// `docs/operations/batch-integrity-review-2026-08-04.md` documents it.
//
// The only remaining work is deterministic bookkeeping: the inbox item that
// requested the fix was never retired, so it keeps resurfacing as live
// backlog. This cycle removes it. The predicates below therefore split into
// two groups:
//
//   - 001/002/003 — RED at authoring time: the retirement itself (file gone
//     from disk, git sees a tracked-file deletion, and no renamed/suffixed
//     survivor variant is left behind under .evolve/inbox/).
//   - 004/005 — pre-existing GREEN, and deliberately so: they are the
//     anti-over-deletion and no-regression guards. AC3 is "the shipped fix
//     is verified UNTOUCHED, not re-implemented", so its predicate must be
//     green both before and after; it goes RED only if the builder damages
//     the admission contract or mass-deletes sibling backlog while retiring
//     one item.
package cycle1383

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// staleInboxItem is the backlog entry this cycle retires. Relative to the
// repo root so both the worktree (where the deletion is authored and
// committed) and a post-merge main resolve it identically.
const staleInboxItem = ".evolve/inbox/2026-08-04T05-04-00Z-triage-protected-surface-admission.json"

// TestC1383_001_StaleInboxItemRemovedFromDisk is AC1's primary predicate: the
// retired backlog entry must be gone from the working tree, not blanked,
// emptied, or commented out in place — a zero-byte or `{}` stub still gets
// enumerated by the inbox reader and still resurfaces as live backlog, which
// is the exact staleness this cycle exists to end.
func TestC1383_001_StaleInboxItemRemovedFromDisk(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, staleInboxItem)

	if _, err := os.Stat(path); err == nil {
		t.Errorf("stale inbox item still present on disk: %s — the fix it requests shipped in cycle-1312 (0d07b200); retire the file, do not blank it", staleInboxItem)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

// TestC1383_002_RemovalIsATrackedGitDeletion is AC1's durability half. The
// item is a TRACKED file (`git ls-files` lists it), so an untracked-style
// `rm` that git never records would vanish from this worktree and come back
// on the next checkout — the backlog would be un-retired the moment the lane
// merged. Accepts the deletion in either git column (staged `D ` or
// worktree-only ` D`): predicates run at audit, before ship stages the tree,
// so demanding a staged deletion here would false-RED correct work.
func TestC1383_002_RemovalIsATrackedGitDeletion(t *testing.T) {
	root := acsassert.RepoRoot(t)

	out, err := gitPorcelain(root, staleInboxItem)
	if err != nil {
		t.Fatalf("git status --porcelain -- %s: %v", staleInboxItem, err)
	}
	if !strings.Contains(out, "D") {
		t.Errorf("git does not report a deletion for %s (porcelain=%q) — remove the tracked file so the retirement survives merge, e.g. `git rm`", staleInboxItem, out)
	}
}

// TestC1383_003_NoSurvivingVariantUnderInbox is the anti-gaming edge case.
// Retiring an item by RENAMING it (`.bak`, `.disabled`, a re-datestamped
// copy) leaves the same payload discoverable under .evolve/inbox/ and defeats
// the point: the next scout picks it up again under a new name. Nothing whose
// filename carries the item's slug may remain anywhere under the inbox tree.
func TestC1383_003_NoSurvivingVariantUnderInbox(t *testing.T) {
	root := acsassert.RepoRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")

	var survivors []string
	err := filepath.Walk(inbox, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), "triage-protected-surface-admission") {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			survivors = append(survivors, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", inbox, err)
	}
	if len(survivors) != 0 {
		t.Errorf("item survives under .evolve/inbox/ by rename/suffix: %v — retire it, do not relocate it", survivors)
	}
}

// TestC1383_004_SiblingBacklogPreserved is the over-deletion negative: this
// cycle retires exactly ONE backlog entry. A wildcard sweep of .evolve/inbox/
// would also satisfy 001–003 while silently destroying ~76 unrelated queued
// items, so pin the directory's floor plus two long-lived siblings and the
// `.keep` sentinel that holds the directory in git.
//
// Pre-existing GREEN by construction (see package doc) — it fails only on
// collateral damage.
func TestC1383_004_SiblingBacklogPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")

	for _, keep := range []string{
		".keep",
		"2026-07-05T15-20-00Z-sleep-time-kb-consolidation.json",
		"2026-07-08T00-50-00Z-dead-api-sweep.json",
	} {
		if !acsassert.FileExists(t, filepath.Join(inbox, keep)) {
			t.Errorf("unrelated inbox entry destroyed: .evolve/inbox/%s — retire only %s", keep, filepath.Base(staleInboxItem))
		}
	}

	entries, err := filepath.Glob(filepath.Join(inbox, "*.json"))
	if err != nil {
		t.Fatalf("glob %s: %v", inbox, err)
	}
	// 77 queued items at authoring time; 76 must remain after the single
	// retirement. A floor of 60 catches a mass sweep without going brittle
	// on legitimate concurrent consumption by other lanes.
	if len(entries) < 60 {
		t.Errorf("inbox collapsed to %d items — a single retirement must leave the rest of the backlog intact", len(entries))
	}
}

// TestC1383_005_ProtectedSurfaceAdmissionStillEnforced is AC3: the shipped
// cycle-1312 admission check must be verified UNTOUCHED, not re-implemented.
// Behavioral, not a source grep — it drives the real
// `hooks.Classify` -> `protectedTopNViolation` path through the contract
// suite that pins it, so it goes RED if the builder edits triage.go or
// weakens the test file while doing the bookkeeping.
//
// Shape rules: ONE named package, narrowed with -run (never a `/...` sweep),
// cmd.Dir set explicitly (never a cwd-relative `go test`), and a PASS-count
// floor so a -run pattern that matches nothing cannot report a hollow `ok`.
//
// Pre-existing GREEN by construction (see package doc).
func TestC1383_005_ProtectedSurfaceAdmissionStillEnforced(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// Name all five cycle-1312 cases explicitly rather than a loose prefix:
	// a renamed or deleted case must show up as a missing PASS, not silently
	// drop out of a fuzzy pattern.
	pattern := "TestTriageClassify_(" + strings.Join([]string{
		"RejectsProtectedSurfaceTopNCard_BraceSyntax",
		"RejectsProtectedSurfaceTopNCard_BareSyntax",
		"RejectsAmongMultipleCards_NamesOffendingIdOnly",
		"AllowsNonProtectedTopNCard",
		"NoFilesSegmentIsUnaffected",
	}, "|") + ")$"

	cmd := exec.Command("go", "test", "-count=1", "-v",
		"-run", pattern, "./internal/phases/triage")
	cmd.Dir = filepath.Join(root, "go")
	raw, err := cmd.CombinedOutput()
	out := string(raw)

	if err != nil {
		t.Errorf("protected-surface admission suite is no longer green: %v\n%s", err, out)
	}

	passes := strings.Count(out, "--- PASS: TestTriageClassify_")
	if passes < 5 {
		t.Errorf("expected all 5 cycle-1312 admission cases to pass, got %d — the admission contract was weakened, renamed, or deleted\n%s", passes, out)
	}
}

// gitPorcelain returns `git status --porcelain` for a single repo-relative
// path, scoped with -C so it resolves against root and never against the
// process cwd (which differs between the main tree, this worktree, and every
// other fleet lane).
func gitPorcelain(root, relPath string) (string, error) {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "--", relPath)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
