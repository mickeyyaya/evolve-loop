package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

func countOccur(xs []int, v int) int {
	n := 0
	for _, x := range xs {
		if x == v {
			n++
		}
	}
	return n
}

func TestProgressPath_UnderEvolveDir(t *testing.T) {
	got := ProgressPath("/x/.evolve", "abc123")
	want := "/x/.evolve/campaign-progress-abc123.json"
	if got != want {
		t.Errorf("ProgressPath = %q, want %q", got, want)
	}
}

func TestHashPlan_StableAndContentSensitive(t *testing.T) {
	a := HashPlan([]byte(`{"version":1}`))
	b := HashPlan([]byte(`{"version":1}`))
	c := HashPlan([]byte(`{"version":2}`))
	if a == "" {
		t.Fatal("HashPlan returned empty")
	}
	if a != b {
		t.Errorf("HashPlan not stable: %q vs %q", a, b)
	}
	if a == c {
		t.Error("HashPlan not sensitive to content change")
	}
}

func TestLoadProgress_AbsentReturnsZeroValue(t *testing.T) {
	p, err := LoadProgress(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadProgress(absent): unexpected error %v", err)
	}
	if p == nil {
		t.Fatal("LoadProgress(absent): want non-nil zero progress, got nil")
	}
	if len(p.CompletedWaves) != 0 || p.PlanSHA != "" {
		t.Errorf("LoadProgress(absent): want zero-value, got %+v", p)
	}
}

func TestCampaignProgress_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign-progress-abc.json")
	want := &CampaignProgress{
		PlanSHA:           "deadbeef",
		CompletedWaves:    []int{0, 1},
		CompletedCycleIDs: []string{"a", "b"},
		FailedCycleIDs:    []string{"poison"},
	}
	if err := want.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadProgress(path)
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if got.PlanSHA != want.PlanSHA ||
		len(got.CompletedWaves) != 2 || got.CompletedWaves[1] != 1 ||
		len(got.CompletedCycleIDs) != 2 || got.CompletedCycleIDs[0] != "a" ||
		len(got.FailedCycleIDs) != 1 || got.FailedCycleIDs[0] != "poison" {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestCampaignProgress_SaveLeavesNoTempArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	if err := (&CampaignProgress{PlanSHA: "x"}).Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("Save left a .tmp file (not atomic): stat err=%v", err)
	}
}

func TestCampaignProgress_SaveCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "p.json")
	if err := (&CampaignProgress{PlanSHA: "x"}).Save(path); err != nil {
		t.Fatalf("Save into missing parent dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("progress file not created: %v", err)
	}
}

func TestCampaignProgress_MarkWaveCompleteIdempotent(t *testing.T) {
	p := &CampaignProgress{}
	p.MarkWaveComplete(0, []string{"a", "b"})
	p.MarkWaveComplete(0, []string{"a", "b"}) // repeat must not duplicate
	if !p.IsWaveComplete(0) {
		t.Fatal("IsWaveComplete(0) = false after MarkWaveComplete")
	}
	if n := countOccur(p.CompletedWaves, 0); n != 1 {
		t.Errorf("wave 0 recorded %d times, want 1 (idempotent)", n)
	}
	if p.IsWaveComplete(1) {
		t.Error("IsWaveComplete(1) = true, want false")
	}
	if len(p.CompletedCycleIDs) != 2 {
		t.Errorf("CompletedCycleIDs = %v, want 2 unique ids", p.CompletedCycleIDs)
	}
}
