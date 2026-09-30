//go:build acs

package cycle1383

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const staleInboxItem = ".evolve/inbox/2026-08-04T05-04-00Z-triage-protected-surface-admission.json"

func TestC1383_001_StaleInboxItemRemovedFromDisk(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, staleInboxItem)

	if _, err := os.Stat(path); err == nil {
		t.Errorf("stale inbox item still present on disk: %s — the fix it requests shipped in cycle-1312 (0d07b200); retire the file, do not blank it", staleInboxItem)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

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
	if len(entries) < 60 {
		t.Errorf("inbox collapsed to %d items — a single retirement must leave the rest of the backlog intact", len(entries))
	}
}

func TestC1383_005_ProtectedSurfaceAdmissionStillEnforced(t *testing.T) {
	root := acsassert.RepoRoot(t)

	pattern := "TestTriageClassify_(" + strings.Join([]string{
		"RoutesProtectedSurfaceTopNCard_BraceSyntax",
		"RoutesProtectedSurfaceTopNCard_BareSyntax",
		"RoutesAmongMultipleCards_NamesOffendingIdOnly",
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

func gitPorcelain(root, relPath string) (string, error) {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "--", relPath)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
