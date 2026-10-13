package retrofile

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var nowF4 = time.Date(2026, 10, 1, 2, 3, 4, 0, time.UTC)

func TestFileActions_NoActionsTouchesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inbox")

	written, err := FileActions(dir, 3, nil, 1.0, nowF4)

	if err != nil || written != nil {
		t.Fatalf("written=%v err=%v", written, err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("an empty action list must not create the inbox: %v", statErr)
	}
}

func TestFileActions_FaultsAreNamedAndEarlierWritesAreStillReported(t *testing.T) {
	t.Run("an existing item that cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		locked := filepath.Join(dir, "locked.json")
		if err := os.WriteFile(locked, []byte(`{"id":"x"}`), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
		if f, err := os.Open(locked); err == nil {
			f.Close()
			t.Skip("file permissions are not enforced for this user")
		}

		written, err := FileActions(dir, 3, []PreventiveAction{{ID: "new", PriorityClass: "correctness"}}, 1.0, nowF4)

		if err == nil || !strings.Contains(err.Error(), "read existing item") || written != nil {
			t.Fatalf("written=%v err=%v", written, err)
		}
	})
	t.Run("a weight that cannot be encoded", func(t *testing.T) {
		dir := t.TempDir()

		written, err := FileActions(dir, 3, []PreventiveAction{{ID: "nan", PriorityClass: "correctness"}}, math.NaN(), nowF4)

		if err == nil || !strings.Contains(err.Error(), "encode item nan") || len(written) != 0 {
			t.Fatalf("written=%v err=%v", written, err)
		}
	})
	t.Run("the destination path is already a directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "auto-retro-3-second.json"), 0o755); err != nil {
			t.Fatal(err)
		}

		written, err := FileActions(dir, 3, []PreventiveAction{
			{ID: "first", PriorityClass: "correctness"},
			{ID: "second", PriorityClass: "correctness"},
		}, 1.0, nowF4)

		if err == nil || !strings.Contains(err.Error(), "write item second") {
			t.Fatalf("err = %v", err)
		}
		if len(written) != 1 || filepath.Base(written[0]) != "auto-retro-3-first.json" {
			t.Errorf("the item filed before the fault must still be reported: %v", written)
		}
	})
}

func TestFileActions_SkipsKnownAndRepeatedIDsAndUsesTheWeightHint(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "queued.json"), []byte(`{"id":"queued"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "processed", "cycle-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "processed", "cycle-1", "done.json"), []byte(`{"id":"done"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.json"), []byte(`not json`), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := FileActions(dir, 8, []PreventiveAction{
		{ID: "queued", PriorityClass: "correctness"},
		{ID: "done", PriorityClass: "correctness"},
		{ID: "", PriorityClass: "correctness"},
		{ID: "fresh", PriorityClass: "correctness", WeightHint: 7.5},
		{ID: "fresh", PriorityClass: "correctness"},
	}, 1.0, nowF4)

	if err != nil || len(written) != 1 || filepath.Base(written[0]) != "auto-retro-8-fresh.json" {
		t.Fatalf("written=%v err=%v", written, err)
	}
	body, _ := os.ReadFile(written[0])
	if !strings.Contains(string(body), `"weight": 7.5`) || !strings.Contains(string(body), `"injected_at": "2026-10-01T02:03:04Z"`) {
		t.Errorf("item = %s", body)
	}
}
