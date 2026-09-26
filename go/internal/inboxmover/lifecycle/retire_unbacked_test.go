package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func unbackedMover(t *testing.T) (inbox string, m *Mover, rec *recordingAppender) {
	t.Helper()
	inbox = filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	rec = &recordingAppender{}
	return inbox, New(inbox, rec, WithNow(func() time.Time { return fixedClock })), rec
}

func TestRetireUnbacked_WritesARecordEveryRetirementReaderFinds(t *testing.T) {
	t.Parallel()
	inbox, m, rec := unbackedMover(t)
	reason := "ship-promote-processed: no inbox item backs the id"

	path, err := m.RetireUnbacked("ghost", "processed", PromoteOpts{Cycle: "7", CommitSHA: "51edfb7fabcd"}, reason)

	if err != nil {
		t.Fatalf("RetireUnbacked: %v", err)
	}
	cycleDir := filepath.Join(inbox, "processed", "cycle-7")
	if want := filepath.Join(cycleDir, "51edfb7f-ghost.json"); path != want {
		t.Fatalf("path = %s, want %s", path, want)
	}
	if found, err := FindFileByTaskID(cycleDir, "ghost"); err != nil || found != path {
		t.Errorf("FindFileByTaskID = %q, %v; want the record", found, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["id"] != "ghost" || got["unbacked"] != true || got["retired_reason"] != reason || got["retired_cycle"] != float64(7) || got["git_sha"] != "51edfb7fabcd" {
		t.Errorf("record = %v", got)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "retire-unbacked" || rec.records[0].TaskID != "ghost" ||
		rec.records[0].Cycle != 7 || rec.records[0].GitHead != "51edfb7fabcd" || !strings.Contains(rec.records[0].Message, reason) {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestRetireUnbacked_RejectedRecordCarriesNoShaPrefix(t *testing.T) {
	t.Parallel()
	inbox, m, _ := unbackedMover(t)

	path, err := m.RetireUnbacked("ghost", "rejected", PromoteOpts{Cycle: "7"}, "planned no-work")

	if err != nil {
		t.Fatalf("RetireUnbacked: %v", err)
	}
	if want := filepath.Join(inbox, "rejected", "cycle-7", "ghost.json"); path != want {
		t.Errorf("path = %s, want %s", path, want)
	}
}

func TestRetireUnbacked_LeavesAnExistingRecordAlone(t *testing.T) {
	t.Parallel()
	_, m, rec := unbackedMover(t)
	first, err := m.RetireUnbacked("ghost", "processed", PromoteOpts{Cycle: "7"}, "first")
	if err != nil {
		t.Fatal(err)
	}

	second, err := m.RetireUnbacked("ghost", "processed", PromoteOpts{Cycle: "7"}, "second")

	if err != nil || second != first {
		t.Fatalf("second = %s, %v; want the first record %s", second, err, first)
	}
	body, _ := os.ReadFile(first)
	if !strings.Contains(string(body), `"first"`) || len(rec.records) != 1 {
		t.Errorf("the first record and its one ledger line stand: %s %d", body, len(rec.records))
	}
}

func TestRetireUnbacked_RefusesABadIDOrState(t *testing.T) {
	t.Parallel()
	inbox, m, rec := unbackedMover(t)
	for name, tc := range map[string]struct {
		id, state string
		want      error
	}{
		"empty id":         {"", "processed", ErrBadArgs},
		"path id":          {"a/b", "processed", ErrBadArgs},
		"escaping id":      {"../escape", "processed", ErrBadArgs},
		"dot id":           {"..", "processed", ErrBadArgs},
		"retry state":      {"ghost", "retry", ErrBadState},
		"consumed state":   {"ghost", "consumed", ErrBadState},
		"quarantine state": {"ghost", "quarantine", ErrBadState},
		"empty state":      {"ghost", "", ErrBadState},
	} {
		if _, err := m.RetireUnbacked(tc.id, tc.state, PromoteOpts{Cycle: "7"}, "r"); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	var files []string
	_ = filepath.WalkDir(inbox, func(p string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if len(files) != 0 || len(rec.records) != 0 {
		t.Errorf("a refusal writes nothing: %v %d", files, len(rec.records))
	}
}

func TestRetireUnbacked_ReportsAWriteFaultAndWritesNothing(t *testing.T) {
	t.Parallel()
	for name, block := range map[string]func(t *testing.T, inbox string){
		"the state dir is a file": func(t *testing.T, inbox string) {
			if err := os.WriteFile(filepath.Join(inbox, "rejected"), []byte("not a dir"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"the cycle dir is read-only": func(t *testing.T, inbox string) {
			if os.Geteuid() == 0 {
				t.Skip("root writes anywhere")
			}
			dir := filepath.Join(inbox, "rejected", "cycle-7")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		},
	} {
		t.Run(name, func(t *testing.T) {
			inbox, m, rec := unbackedMover(t)
			block(t, inbox)

			path, err := m.RetireUnbacked("ghost", "rejected", PromoteOpts{Cycle: "7"}, "r")

			if err == nil || path != "" || len(rec.records) != 0 {
				t.Fatalf("path = %q, err = %v, ledger = %d; want the fault and no record", path, err, len(rec.records))
			}
			if _, err := FindFileByTaskID(filepath.Join(inbox, "rejected", "cycle-7"), "ghost"); err == nil {
				t.Error("a failed write leaves no record behind")
			}
		})
	}
}

func TestWriteRecord_RefusesADirectoryAsTheDestination(t *testing.T) {
	t.Parallel()
	dest := filepath.Join(t.TempDir(), "taken")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	err := writeRecord(dest, []byte(`{"id":"x"}`))

	if err == nil {
		t.Fatal("a directory at the destination is a fault")
	}
	if entries, _ := os.ReadDir(filepath.Dir(dest)); len(entries) != 1 {
		t.Errorf("the temp file is removed after the failed commit: %v", entries)
	}
}
