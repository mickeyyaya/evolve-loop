//go:build acs

package cycle1529

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	staleItemID   = "completion-contract-cancel-parity"
	staleItemFile = "2026-07-16T10-30-00Z-completion-contract-cancel-parity.json"
)

func gitTracked(root, rel string) bool {
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	return err == nil && code == 0
}

func TestC1529_001_StaleInboxItemRetiredFromLiveBacklog(t *testing.T) {
	root := acsassert.RepoRoot(t)
	inboxDir := filepath.Join(root, ".evolve", "inbox")

	items, warns, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		t.Fatalf("RED: inboxbatch.LoadDir(%s): %v", inboxDir, err)
	}
	for _, w := range warns {
		t.Logf("inbox load warning: %s", w)
	}
	if len(items) == 0 {
		t.Fatalf("RED: loader returned 0 items for %s — predicate would pass vacuously", inboxDir)
	}
	for _, it := range items {
		if it.ID == staleItemID {
			t.Errorf("RED: %q is still in the live backlog (%d items loaded) — the closure must move the file out of .evolve/inbox/, not annotate it in place", staleItemID, len(items))
		}
	}
	live := filepath.Join(inboxDir, staleItemFile)
	if _, err := os.Stat(live); err == nil {
		t.Errorf("RED: %s still present on disk", live)
	}
	if gitTracked(root, filepath.Join(".evolve", "inbox", staleItemFile)) {
		t.Errorf("RED: .evolve/inbox/%s is still git-tracked at the live path", staleItemFile)
	}
}

func TestC1529_002_ConsumedRecordCitesParityEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join(".evolve", "inbox", "consumed", staleItemFile)
	abs := filepath.Join(root, rel)

	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("RED: consumed record unreadable at %s: %v", rel, err)
	}
	var item struct {
		ID       string `json:"id"`
		Consumed struct {
			At         string `json:"at"`
			Cycle      string `json:"cycle"`
			Resolution string `json:"resolution"`
		} `json:"consumed"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("RED: %s is not valid JSON: %v", rel, err)
	}
	if item.ID != staleItemID {
		t.Errorf("RED: consumed record id = %q, want %q (identity must survive the move)", item.ID, staleItemID)
	}
	if strings.TrimSpace(item.Consumed.At) == "" {
		t.Errorf("RED: consumed.at is empty in %s", rel)
	}
	if item.Consumed.Cycle != "1529" {
		t.Errorf("RED: consumed.cycle = %q, want \"1529\"", item.Consumed.Cycle)
	}
	res := item.Consumed.Resolution
	for _, needle := range []string{
		"completion_cancel_parity_test.go",
		"not-observed",
	} {
		if !strings.Contains(res, needle) {
			t.Errorf("RED: consumed.resolution does not cite %q — got %q", needle, res)
		}
	}
	if !gitTracked(root, rel) {
		t.Errorf("RED: %s is untracked — it would be dropped at ship", rel)
	}
}

func TestC1529_003_CancelParityTestsRemainGreen(t *testing.T) {
	_ = acsassert.RepoRoot(t)
	const pkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	const run = "TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout|TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout|TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout|TestArtifactDetector_CtxCancelledShortCircuitsDebounce"

	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", run, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: parity suite not green (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	for _, name := range []string{
		"TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout",
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
	} {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("RED: %s did not report PASS — the closure evidence is incomplete", name)
		}
	}
}

const (
	closureBase = "19b427c4214e1ad6f84239cd1781f592b0faec22"
	closureShip = "57e227c1e36f33562c922dcdf2546b160739e45d"
)

func TestC1529_004_ClosureStaysDocOnly(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "--name-only", closureBase, closureShip, "--", "go/internal/bridge")
	if err != nil || code != 0 {
		t.Fatalf("RED: git diff %s..%s failed (code=%d err=%v): %s", closureBase[:8], closureShip[:8], code, err, stderr)
	}
	if changed := strings.TrimSpace(stdout); changed != "" {
		t.Errorf("RED: closure task modified bridge production code (must stay doc-only):\n%s", changed)
	}
}
