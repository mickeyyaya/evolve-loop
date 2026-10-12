//go:build acs

package cycle1863

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedTest(t *testing.T, pattern string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", pattern, "./cmd/evolve")
	if err != nil || code != 0 || strings.Contains(stdout, "no tests to run") {
		t.Errorf("go test -run %s ./cmd/evolve exit=%d err=%v\n%s%s", pattern, code, err, stdout, stderr)
	}
}

func TestC1863_001_ListJSONPrintsEveryTodoWithAgeAndCyclesUnpicked(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList$")
}

func TestC1863_002_ListSortAndLimitOrderTheRows(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_SortAndLimit$")
}

func TestC1863_003_ListRejectsAnUnknownSortAndANegativeLimit(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_RejectsBadFlags$")
}

func TestC1863_004_ListedIdsAreAcceptedVerbatimByApplyDecisions(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_AcceptanceWithApplyDecisions$")
}

func TestC1863_005_AnUnreadableOrMalformedStateExitsTwoWithNoOutput(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_UnreadableStateExits2$")
}

func TestC1863_006_AnEmptyOrAbsentQueueListsAsAnEmptyArrayWithExitZero(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_EmptyCarryover$")
}

func TestC1863_007_ListNeverMutatesStateJSON(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_ListIsReadOnly$")
}

func TestC1863_008_TheTableFormNamesEveryId(t *testing.T) {
	runNamedTest(t, "^TestCarryoverList_TableOutputNamesEveryID$")
}

func TestC1863_009_RuntimeReferenceDocumentsCarryoverList(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "evolve carryover list") {
		t.Errorf("runtime-reference.md does not document `evolve carryover list`")
	}
}

func TestC1863_010_TheChangeAddsNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("git merge-base exit=%d err=%v", code, err)
	}
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", strings.TrimSpace(base), "--", "go/cmd/evolve/cmd_carryover.go", "go/cmd/evolve/cmd_carryover_test.go")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d err=%v %s", code, err, stderr)
	}
	for _, line := range strings.Split(diff, "\n") {
		added := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && (strings.HasPrefix(added, "//") || strings.HasPrefix(added, "/*")) {
			t.Errorf("added comment: %s", line)
		}
	}
}
