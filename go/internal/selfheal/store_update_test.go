package selfheal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

func TestUpdateIncidents_HoldsTheStoreLockAndWritesOnlyTheChangedRecords(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "incidents")
	kept := Incident{Schema: RecordSchema, ID: "inc-kept", Fingerprint: "stall:x", State: StateOpen}
	if err := WriteIncident(dir, kept); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(filepath.Join(dir, "inc-kept.json"))

	err := UpdateIncidents(dir, func(current []Incident) ([]Incident, error) {
		release, held, lerr := flock.TryLock(dir + ".lock")
		if release != nil {
			release()
		}
		if lerr != nil || !held {
			t.Errorf("the store lock is free inside the update (held=%v, err=%v)", held, lerr)
		}
		return append(current, Incident{Schema: RecordSchema, ID: "inc-new", Fingerprint: "pipeline-defect:c", State: StateOpen}), nil
	})

	records, lerr := LoadIncidents(dir)
	if err != nil || lerr != nil || len(records) != 2 {
		t.Fatalf("UpdateIncidents = %v; records %+v, %v; want the new record beside the kept one", err, records, lerr)
	}
	if after, _ := os.Stat(filepath.Join(dir, "inc-kept.json")); !after.ModTime().Equal(before.ModTime()) {
		t.Error("the unchanged record was rewritten")
	}
}

func TestUpdateIncidents_ReturnsTheFaults(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "incidents")
	refused := errors.New("refused")
	if err := UpdateIncidents(dir, func([]Incident) ([]Incident, error) { return nil, refused }); !errors.Is(err, refused) {
		t.Errorf("err = %v; want the update error", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inc-bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateIncidents(dir, func(c []Incident) ([]Incident, error) { return c, nil }); err == nil {
		t.Error("err = nil; want the load error of a corrupt record")
	}
	readOnly := filepath.Join(t.TempDir(), "incidents")
	if err := os.MkdirAll(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })
	if err := UpdateIncidents(readOnly, func([]Incident) ([]Incident, error) { return []Incident{{ID: "inc-y"}}, nil }); err == nil {
		t.Error("err = nil; want the write error of a read-only store")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateIncidents(filepath.Join(blocked, "incidents"), func(c []Incident) ([]Incident, error) {
		return []Incident{{ID: "inc-x", OpenedAt: time.Now()}}, nil
	}); err == nil {
		t.Error("err = nil; want the lock error under a file")
	}
}
