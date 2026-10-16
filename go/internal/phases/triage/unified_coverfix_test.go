package triage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func replaceWithFile(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func replaceWithBusyDir(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lockDirAgainstWrites(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestProcessUnifiedCommitment_MalformedDecisionIsLeftToTheDigest(t *testing.T) {
	t.Parallel()
	req := writeUnifiedFixture(t, nil, nil, nil, false)
	decision := filepath.Join(req.Workspace, "triage-decision.json")
	if err := os.WriteFile(decision, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := processUnifiedCommitment(req)
	if err != nil || len(diags) != 0 {
		t.Fatalf("a malformed decision is not this step's to report: diags=%+v err=%v", diags, err)
	}
	if raw, _ := os.ReadFile(decision); string(raw) != "not json" {
		t.Errorf("the malformed decision must stay as written, got %q", raw)
	}
}

func TestProcessUnifiedCommitment_ClaimThatIsNotAnObjectIsRejected(t *testing.T) {
	t.Parallel()
	req := writeUnifiedFixture(t, nil, nil, nil, false)
	if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), []byte(`{"top_n":[],"unified_commitment":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := processUnifiedCommitment(req)
	if err != nil {
		t.Fatal(err)
	}
	assertUnifiedDiagnostics(t, diags, "invalid JSON")
}

func TestProcessUnifiedCommitment_UnreadableInboxIsAnError(t *testing.T) {
	t.Parallel()
	small := unifiedItems(2, "seam")
	req := writeUnifiedFixture(t, small, itemIDs(small), itemIDs(small), false)
	replaceWithFile(t, filepath.Join(req.ProjectRoot, ".evolve", "inbox"))
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "load inbox for unified_commitment") {
		t.Fatalf("want the inbox load error, got %v", err)
	}
}

func TestProcessUnifiedCommitment_UnreadableClaimedInboxIsAnError(t *testing.T) {
	t.Parallel()
	small := unifiedItems(2, "seam")
	req := writeUnifiedFixture(t, small, itemIDs(small), itemIDs(small), false)
	replaceWithFile(t, filepath.Join(req.ProjectRoot, ".evolve", "inbox", "processing", "cycle-1720"))
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "load claimed inbox for unified_commitment") {
		t.Fatalf("want the claimed inbox load error, got %v", err)
	}
}

func TestProcessUnifiedCommitment_UnreadableTopNRejectsTheClaim(t *testing.T) {
	t.Parallel()
	small := unifiedItems(2, "seam")
	req := writeUnifiedFixture(t, small, itemIDs(small), itemIDs(small), false)
	fields := readDecisionFields(t, req.Workspace)
	fields["top_n"] = []byte(`"oops"`)
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := processUnifiedCommitment(req)
	if err != nil {
		t.Fatal(err)
	}
	assertUnifiedDiagnostics(t, diags, "top_n is unreadable")
}

func TestProcessUnifiedCommitment_LargeClaimWiderThanTheCampaignCapIsRejected(t *testing.T) {
	t.Parallel()
	wide := unifiedItems(campaign.MaxWaveWidth+1, "seam")
	req := writeUnifiedFixture(t, wide, itemIDs(wide), itemIDs(wide), false)
	diags, err := processUnifiedCommitment(req)
	if err != nil {
		t.Fatal(err)
	}
	assertUnifiedDiagnostics(t, diags, "campaign projection is invalid")
	assertCampaignPlan(t, req.Workspace, false, 0)
}

func TestProcessUnifiedCommitment_PlanThatCannotBeWrittenIsAnError(t *testing.T) {
	t.Parallel()
	large := unifiedItems(inboxbatch.DefaultMaxItems+1, "seam")
	req := writeUnifiedFixture(t, large, itemIDs(large), itemIDs(large), false)
	replaceWithBusyDir(t, filepath.Join(req.Workspace, "campaign-plan.json"))
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "write unified campaign plan") {
		t.Fatalf("want the plan write error, got %v", err)
	}
}

func TestProcessUnifiedCommitment_StalePlanThatCannotBeRemovedIsAnError(t *testing.T) {
	t.Parallel()
	small := unifiedItems(2, "seam")
	req := writeUnifiedFixture(t, small, itemIDs(small), itemIDs(small), false)
	replaceWithBusyDir(t, filepath.Join(req.Workspace, "campaign-plan.json"))
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "remove stale unified campaign plan") {
		t.Fatalf("want the stale plan removal error, got %v", err)
	}
}

func TestProcessUnifiedCommitment_RejectionWithAnUnremovablePlanIsAnError(t *testing.T) {
	t.Parallel()
	mixed := []unifiedFixtureItem{{id: "unified-member-0", campaign: "seam"}, {id: "unified-member-1", campaign: "other"}}
	req := writeUnifiedFixture(t, mixed, itemIDs(mixed), itemIDs(mixed), false)
	replaceWithBusyDir(t, filepath.Join(req.Workspace, "campaign-plan.json"))
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "remove stale unified campaign plan") {
		t.Fatalf("want the stale plan removal error, got %v", err)
	}
}

func TestProcessUnifiedCommitment_ProjectionThatCannotBeWrittenIsAnError(t *testing.T) {
	t.Parallel()
	small := unifiedItems(2, "seam")
	req := writeUnifiedFixture(t, small, itemIDs(small), itemIDs(small), false)
	if err := os.Remove(filepath.Join(req.Workspace, "campaign-plan.json")); err != nil {
		t.Fatal(err)
	}
	lockDirAgainstWrites(t, req.Workspace)
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "write validated unified projection") {
		t.Fatalf("want the projection write error, got %v", err)
	}
}

func TestProcessUnifiedCommitment_SpoofedProjectionThatCannotBeClearedIsAnError(t *testing.T) {
	t.Parallel()
	small := unifiedItems(2, "seam")
	req := writeUnifiedFixture(t, small, itemIDs(small), nil, true)
	lockDirAgainstWrites(t, req.Workspace)
	if _, err := processUnifiedCommitment(req); err == nil || !strings.Contains(err.Error(), "clear invalid unified projection") {
		t.Fatalf("want the projection clear error, got %v", err)
	}
}

func TestRepoPaths_SplitsInRepoFromOutsideAndKeepsTheDirectoryMarker(t *testing.T) {
	t.Parallel()
	inRepo, outside := repoPaths([]string{"go/internal/ship/", "a/./b.go", "../escape.go", "/abs/path.go", ".."})
	if want := []string{"go/internal/ship/", "a/b.go"}; !reflect.DeepEqual(inRepo, want) {
		t.Errorf("inRepo = %v, want %v", inRepo, want)
	}
	if want := []string{"../escape.go", "/abs/path.go", ".."}; !reflect.DeepEqual(outside, want) {
		t.Errorf("outside = %v, want %v", outside, want)
	}
}
