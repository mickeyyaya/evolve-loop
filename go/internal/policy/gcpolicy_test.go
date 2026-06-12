package policy

// L3.1: policy.json gains a declarative "gc" block (schema in internal/gc).
// Absent block ⇒ nil ⇒ gc applies its own defaults; a present block parses
// field-for-field. Pinned here so the user-facing file contract can't drift
// from the engine's schema silently.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ParsesGCBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	raw := `{
  "mandatory_phases": ["scout"],
  "gc": {
    "runs": {"keep_full": 5, "archive_after_days": 14, "delete_after_days": 60},
    "salvage_ttl_days": 21,
    "logs_ttl_days": 45,
    "tracker_ttl_days": 3
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.GC == nil {
		t.Fatal("gc block must parse to a non-nil GC policy")
	}
	if p.GC.Runs.KeepFull != 5 || p.GC.Runs.ArchiveAfterDays != 14 || p.GC.Runs.DeleteAfterDays != 60 {
		t.Errorf("runs ladder mismatch: %+v", p.GC.Runs)
	}
	if p.GC.SalvageTTLDays != 21 || p.GC.LogsTTLDays != 45 || p.GC.TrackerTTLDays != 3 {
		t.Errorf("TTLs mismatch: %+v", *p.GC)
	}
}

func TestLoad_AbsentGCBlockIsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte(`{"mandatory_phases":["scout"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.GC != nil {
		t.Errorf("absent gc block must stay nil (gc defaults apply downstream), got %+v", *p.GC)
	}
}
