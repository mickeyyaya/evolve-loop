package gc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTempCheck_ScanClassifiesEntriesWithoutRemoving(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-30 * time.Hour)
	fresh := now.Add(-1 * time.Hour)

	for name, mtime := range map[string]time.Time{
		"go-build1234567": stale,
		"cycle9-held":     stale,
		"cycle9-unknown":  stale,
		"acs1-fresh":      fresh,
		"notes.txt":       stale,
	} {
		p := filepath.Join(dir, name)
		writeFile(t, filepath.Join(p, "payload"), "0123456789")
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	const probeFailure = "lsof unavailable"
	chk := tempCheck{
		dir: dir,
		now: func() time.Time { return now },
		held: func(context.Context) func(string) (bool, error) {
			return func(p string) (bool, error) {
				switch filepath.Base(p) {
				case "cycle9-held":
					return true, nil
				case "cycle9-unknown":
					return false, errors.New(probeFailure)
				default:
					return false, nil
				}
			}
		},
	}

	items, err := chk.Scan(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	byName := map[string]CheckItem{}
	for _, it := range items {
		byName[filepath.Base(it.Subject)] = it
		if !strings.HasPrefix(it.Rule, "temp.") || it.Evidence == "" || it.Evidence == it.Rule || it.Evidence == it.Action {
			t.Errorf("invalid item shape: %+v", it)
		}
	}
	if len(byName) != 4 {
		t.Fatalf("items = %+v, want 4 pipeline entries", items)
	}
	if _, ok := byName["notes.txt"]; ok {
		t.Errorf("unexpected item for notes.txt: %+v", byName["notes.txt"])
	}
	wantActions := map[string]string{
		"go-build1234567": "remove",
		"cycle9-held":     "spare_held",
		"cycle9-unknown":  "spare_unknown",
		"acs1-fresh":      "keep_fresh",
	}
	for name, want := range wantActions {
		if got := byName[name].Action; got != want {
			t.Errorf("%s: action = %q, want %q", name, got, want)
		}
	}
	if !strings.Contains(byName["cycle9-unknown"].Evidence, probeFailure) {
		t.Errorf("unknown evidence = %q, want %q", byName["cycle9-unknown"].Evidence, probeFailure)
	}
	for name := range wantActions {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("Scan removed %s: %v", name, err)
		}
	}
}

func TestTempCheck_ScanEdgeCasesAndApply(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-30 * time.Hour)

	emptyChk := tempCheck{dir: t.TempDir(), now: func() time.Time { return now }}
	items, err := emptyChk.Scan(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("empty Scan = %v, %v, want non-nil empty slice", items, err)
	}

	missingDir := filepath.Join(t.TempDir(), "absent")
	missingChk := tempCheck{dir: missingDir, now: func() time.Time { return now }}
	if _, err := missingChk.Scan(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}}); err == nil {
		t.Fatal("expected Scan error for unreadable temp dir")
	}
	if err := missingChk.Apply(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}}); err == nil {
		t.Fatal("expected Apply error for unreadable temp dir")
	}

	dir := t.TempDir()
	stalePath := filepath.Join(dir, "go-build999")
	heldPath := filepath.Join(dir, "cycle9-held")
	for _, p := range []string{stalePath, heldPath} {
		writeFile(t, filepath.Join(p, "payload"), "0123456789")
		if err := os.Chtimes(p, stale, stale); err != nil {
			t.Fatal(err)
		}
	}
	chk := tempCheck{
		dir: dir,
		now: func() time.Time { return now },
		held: func(context.Context) func(string) (bool, error) {
			return func(p string) (bool, error) {
				return filepath.Base(p) == "cycle9-held", nil
			}
		},
	}

	offItems, err := chk.Scan(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 0}})
	if err != nil || len(offItems) != 0 {
		t.Fatalf("unset TTL Scan = %v, %v, want empty", offItems, err)
	}
	if err := chk.Apply(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 0}}); err != nil {
		t.Fatalf("unset TTL Apply: %v", err)
	}
	if _, err := os.Stat(stalePath); err != nil {
		t.Fatalf("unset TTL Apply removed %s", stalePath)
	}

	if err := chk.Apply(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Errorf("Apply did not remove stale unheld %s", stalePath)
	}
	if _, err := os.Stat(heldPath); err != nil {
		t.Errorf("Apply removed held %s: %v", heldPath, err)
	}
}

func TestTempCheck_DefaultsUseOsTempDirAndTimeNow(t *testing.T) {
	dir := t.TempDir()
	staleDir := filepath.Join(dir, "go-build1234567")
	writeFile(t, filepath.Join(staleDir, "payload"), "0123456789")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(staleDir, old, old); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", dir)

	chk := tempCheck{
		held: func(context.Context) func(string) (bool, error) {
			return func(string) (bool, error) { return false, nil }
		},
	}
	items, err := chk.Scan(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 || items[0].Action != "remove" || items[0].Subject != staleDir {
		t.Fatalf("items = %+v, want single remove item for %s", items, staleDir)
	}
}

func TestClassifyTempDir_SkipsUnstatableEntry(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("permission-denied entries require non-root euid")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go-build1234567", "payload"), "0123456789")
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	items, err := classifyTempDir(dir, time.Now(), tempConfig{})
	if err != nil {
		t.Fatalf("classifyTempDir: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want unstatable entry skipped", items)
	}
}
