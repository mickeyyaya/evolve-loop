package failurelog

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const previewState = `{"failedApproaches":[` +
	`{"cycle":1,"expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":2,"expiresAt":"2020-01-01T00:00:00Z"},` +
	`{"cycle":3,"recordedAt":"2020-01-01T00:00:00Z"},` +
	`{"cycle":4},` +
	`{"cycle":5,"expiresAt":"not-a-time"},` +
	`"opaque"],` +
	`"carryoverTodos":[` +
	`{"id":"live","expiresAt":"2099-01-01T00:00:00Z","cycles_unpicked":2},` +
	`{"id":"expired","expiresAt":"2020-01-01T00:00:00Z","cycles_unpicked":5},` +
	`{"id":"untimed"}]}`

func entryKeys(t *testing.T, entries []any, key string) []string {
	t.Helper()
	var out []string
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			out = append(out, toKey(m[key]))
		}
	}
	sort.Strings(out)
	return out
}

func toKey(v any) string { return fmt.Sprint(v) }

func expiredKeys(p PrunePreview, key string) []string {
	out := []string{}
	for _, m := range p.Expired {
		out = append(out, toKey(m[key]))
	}
	sort.Strings(out)
	return out
}

func removedKeys(t *testing.T, before, after map[string]any, arr, key string) []string {
	t.Helper()
	kept := map[string]bool{}
	afterEntries, _ := after[arr].([]any)
	for _, k := range entryKeys(t, afterEntries, key) {
		kept[k] = true
	}
	out := []string{}
	beforeEntries, _ := before[arr].([]any)
	for _, k := range entryKeys(t, beforeEntries, key) {
		if !kept[k] {
			out = append(out, k)
		}
	}
	return out
}

func TestPreviewExpired_NamesExactlyWhatThePrunesRemoveAndWritesNothing(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	path := writeState(t, previewState)
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	seeded := readState(t, path)

	failed, carryover, err := PreviewExpired(path, now)

	if err != nil {
		t.Fatalf("PreviewExpired: %v", err)
	}
	if got := mustRead(t, path); got != previewState {
		t.Fatalf("PreviewExpired rewrote state.json: %q", got)
	}
	if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("PreviewExpired touched state.json mtime (info=%v err=%v)", info, err)
	}
	if want := (PruneResult{Before: 6, After: 4, Removed: 2}); failed.PruneResult != want {
		t.Errorf("failed preview counts = %+v, want %+v", failed.PruneResult, want)
	}
	if want := (PruneResult{Before: 3, After: 2, Removed: 1}); carryover.PruneResult != want {
		t.Errorf("carryover preview counts = %+v, want %+v", carryover.PruneResult, want)
	}
	if _, err := PruneExpired(path, now); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneExpiredCarryoverTodos(path, now); err != nil {
		t.Fatal(err)
	}
	pruned := readState(t, path)
	if got, want := expiredKeys(failed, "cycle"), removedKeys(t, seeded, pruned, "failedApproaches", "cycle"); !reflect.DeepEqual(got, want) || len(got) != 2 {
		t.Errorf("preview named failedApproaches %v, the prune removed %v", got, want)
	}
	if got, want := expiredKeys(carryover, "id"), removedKeys(t, seeded, pruned, "carryoverTodos", "id"); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got, []string{"expired"}) {
		t.Errorf("preview named carryoverTodos %v, the prune removed %v", got, want)
	}
}

func TestPreviewExpired_MissingAndUnreadableState(t *testing.T) {
	failed, carryover, err := PreviewExpired(t.TempDir()+"/absent.json", time.Time{})
	if err != nil || failed.Removed != 0 || carryover.Before != 0 || failed.Expired == nil || carryover.Expired == nil {
		t.Errorf("missing state: failed=%+v carryover=%+v err=%v, want zero previews with empty non-nil Expired and nil", failed, carryover, err)
	}

	path := writeState(t, "{not json")
	if _, _, err := PreviewExpired(path, time.Time{}); err == nil || !strings.Contains(err.Error(), "failurelog: parse state") {
		t.Errorf("unparseable state err = %v, want a parse-state error", err)
	}
	if got := mustRead(t, path); got != "{not json" {
		t.Errorf("PreviewExpired rewrote an unparseable state.json: %q", got)
	}
	if _, _, err := PreviewExpired(t.TempDir(), time.Time{}); err == nil || !strings.Contains(err.Error(), "failurelog: read state") {
		t.Errorf("unreadable state err = %v, want a read-state error", err)
	}
}
