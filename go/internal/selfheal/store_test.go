package selfheal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestStore_WritesEachRecordAsItsIDFileAndLoadsThemBack(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".evolve", "incidents")
	closedAt := t0.Add(time.Hour)
	records := []Incident{
		{Schema: RecordSchema, ID: "inc-2026-10-27-bbbbbbbb", Fingerprint: "quality:QUALITY_PQ1_BREACH", Class: ClassQuality, State: StateClosed, Severity: signalcenter.SeverityWarn, OpenedAt: t0, ClosedAt: &closedAt, Sources: []string{"signal:1.1"}},
		{Schema: RecordSchema, ID: "inc-2026-10-27-aaaaaaaa", Fingerprint: "main-red:main", Class: ClassMainRed, State: StateOpen, Severity: signalcenter.SeverityIncident, OpenedAt: t0, Sources: []string{"ci:40"}},
	}
	for _, r := range records {
		if err := WriteIncident(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "inc-2026-10-27-aaaaaaaa.json"))
	if err != nil || !strings.Contains(string(raw), `"closed_at": null`) || !strings.Contains(string(raw), `"repro_test": ""`) {
		t.Fatalf("the record names each field of the spec, closed_at null while open: %v\n%s", err, raw)
	}
	got, err := LoadIncidents(dir)
	if err != nil || len(got) != 2 || got[0].ID != "inc-2026-10-27-aaaaaaaa" || got[1].ClosedAt == nil || !got[1].ClosedAt.Equal(closedAt) {
		t.Fatalf("load: %v %+v", err, got)
	}
}

func TestLoadIncidents_AnAbsentDirectoryIsNoRecordAndOtherFilesAreSkipped(t *testing.T) {
	t.Parallel()
	got, err := LoadIncidents(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(got) != 0 {
		t.Fatalf("absent: %v %+v", err, got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "inc-dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadIncidents(dir); err != nil || len(got) != 0 {
		t.Fatalf("other entries: %v %+v", err, got)
	}
}

func TestLoadIncidents_ACorruptRecordFailsLoudlyWithItsPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "inc-bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIncidents(dir); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("err = %v, want the path of the corrupt record", err)
	}
}

func TestLoadIncidents_AnUnreadableDirectoryOrRecordFails(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIncidents(file); err == nil {
		t.Fatal("a file in place of the directory must fail")
	}
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "inc-link.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIncidents(dir); err == nil {
		t.Fatal("a record that cannot be read must fail")
	}
}

func TestWriteIncident_FailsWhenTheDirectoryCannotBeMade(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteIncident(filepath.Join(file, "incidents"), Incident{ID: "inc-x"}); err == nil {
		t.Fatal("want an error")
	}
}

func TestDir_IsTheIncidentsDirectoryOfTheEvolveDir(t *testing.T) {
	t.Parallel()
	if got := Dir("/p/.evolve"); got != filepath.Join("/p/.evolve", "incidents") {
		t.Fatalf("Dir = %q", got)
	}
}

func TestLoadIncidents_TwoRecordsOfOneFingerprintFailLoudly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, id := range []string{"inc-2026-10-26-aaaaaaaa", "inc-2026-10-27-aaaaaaaa"} {
		if err := WriteIncident(dir, Incident{ID: id, Fingerprint: "main-red:main"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LoadIncidents(dir); err == nil || !strings.Contains(err.Error(), "main-red:main") {
		t.Fatalf("err = %v, want the duplicate fingerprint named", err)
	}
}
