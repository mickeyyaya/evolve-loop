//go:build acs

package cycle536

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const defaultTTL = 30 * 24 * time.Hour

func writeState(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readTodos(t *testing.T, statePath string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json is not valid JSON after the pass: %v", err)
	}
	entries, _ := state["carryoverTodos"].([]any)
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func findTodo(t *testing.T, todos []map[string]any, id string) map[string]any {
	t.Helper()
	for _, m := range todos {
		if s, _ := m["id"].(string); s == id {
			return m
		}
	}
	t.Fatalf("carryover todo %q not found on disk", id)
	return nil
}

func TestC536_001_BackfillStampsLegacyLeavesStampedUntouched(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	preStamped := now.Add(72 * time.Hour).Format(time.RFC3339)
	statePath := writeState(t, `{"carryoverTodos":[`+
		`{"id":"cycle-366-legacy-no-ttl","action":"legacy"},`+
		`{"id":"cycle-528-already-stamped","expiresAt":"`+preStamped+`"}]}`)

	stamped, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now)
	if err != nil {
		t.Fatalf("BackfillLegacyCarryoverExpiry: %v", err)
	}
	if stamped != 1 {
		t.Errorf("exactly one legacy entry should be stamped; got stamped=%d", stamped)
	}

	todos := readTodos(t, statePath)

	legacy := findTodo(t, todos, "cycle-366-legacy-no-ttl")
	got, _ := legacy["expiresAt"].(string)
	if got == "" {
		t.Fatalf("legacy entry must gain an expiresAt after backfill; got none")
	}
	parsed, perr := time.Parse(time.RFC3339, got)
	if perr != nil {
		t.Fatalf("backfilled expiresAt must be RFC3339; got %q err=%v", got, perr)
	}
	if want := now.Add(defaultTTL); !parsed.Equal(want) {
		t.Errorf("legacy entry expiresAt = %s, want now+TTL = %s", parsed, want)
	}

	stampedEntry := findTodo(t, todos, "cycle-528-already-stamped")
	if got := stampedEntry["expiresAt"]; got != preStamped {
		t.Errorf("already-stamped entry must be left untouched; expiresAt = %v, want %q", got, preStamped)
	}
}

func TestC536_002_BackfillIsIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	statePath := writeState(t, `{"carryoverTodos":[`+
		`{"id":"cycle-400-legacy-a","action":"legacy"},`+
		`{"id":"cycle-450-legacy-b","action":"legacy"}]}`)

	first, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now)
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if first != 2 {
		t.Fatalf("first pass should stamp both legacy entries; got %d", first)
	}
	firstDisk, _ := os.ReadFile(statePath)

	second, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if second != 0 {
		t.Errorf("second pass must stamp zero already-stamped entries (idempotent); got %d", second)
	}
	secondDisk, _ := os.ReadFile(statePath)
	if string(firstDisk) != string(secondDisk) {
		t.Errorf("idempotent backfill must not rewrite already-stamped expiresAt:\nfirst:  %s\nsecond: %s", firstDisk, secondDisk)
	}
}

func TestC536_003_BackfilledEntryPrunesOncePastTTL(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	statePath := writeState(t, `{"carryoverTodos":[{"id":"cycle-372-legacy","action":"legacy"}]}`)

	if _, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if pr, err := failurelog.PruneExpiredCarryoverTodos(statePath, now.Add(defaultTTL-time.Hour)); err != nil || pr.Removed != 0 {
		t.Fatalf("backfilled entry must survive until its TTL; pr=%+v err=%v", pr, err)
	}
	pr, err := failurelog.PruneExpiredCarryoverTodos(statePath, now.Add(defaultTTL+time.Hour))
	if err != nil {
		t.Fatalf("prune past TTL: %v", err)
	}
	if pr.Removed != 1 || pr.After != 0 {
		t.Errorf("backfilled entry must be pruned once past its TTL; got %+v", pr)
	}
	if len(readTodos(t, statePath)) != 0 {
		t.Error("state.json must have zero carryoverTodos after the entry is pruned")
	}
}

func TestC536_004_BackfillMissingOrEmptyIsSafeNoOp(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	missing := filepath.Join(t.TempDir(), "nope.json")
	if n, err := failurelog.BackfillLegacyCarryoverExpiry(missing, defaultTTL, now); err != nil || n != 0 {
		t.Fatalf("missing state must be a safe no-op; got stamped=%d err=%v", n, err)
	}

	empty := writeState(t, `{"failedApproaches":[]}`)
	if n, err := failurelog.BackfillLegacyCarryoverExpiry(empty, defaultTTL, now); err != nil || n != 0 {
		t.Fatalf("state with no carryoverTodos must be a no-op; got stamped=%d err=%v", n, err)
	}
}

func TestC536_005_IncrementBumpsEverySurvivorByExactlyOne(t *testing.T) {
	statePath := writeState(t, `{"carryoverTodos":[`+
		`{"id":"a","cycles_unpicked":0},`+
		`{"id":"b","cycles_unpicked":2},`+
		`{"id":"c","cycles_unpicked":5}]}`)

	n, err := failurelog.IncrementCarryoverUnpicked(statePath)
	if err != nil {
		t.Fatalf("IncrementCarryoverUnpicked: %v", err)
	}
	if n != 3 {
		t.Errorf("all three surviving todos should be incremented; got incremented=%d", n)
	}

	want := map[string]float64{"a": 1, "b": 3, "c": 6}
	for _, m := range readTodos(t, statePath) {
		id, _ := m["id"].(string)
		got, ok := m["cycles_unpicked"].(float64)
		if !ok {
			t.Errorf("todo %q lost its cycles_unpicked field", id)
			continue
		}
		if got != want[id] {
			t.Errorf("todo %q cycles_unpicked = %v, want %v (exactly +1)", id, got, want[id])
		}
	}
}

func TestC536_006_IncrementMissingOrEmptyIsSafeNoOp(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.json")
	if n, err := failurelog.IncrementCarryoverUnpicked(missing); err != nil || n != 0 {
		t.Fatalf("missing state must be a safe no-op; got incremented=%d err=%v", n, err)
	}

	empty := writeState(t, `{"failedApproaches":[]}`)
	if n, err := failurelog.IncrementCarryoverUnpicked(empty); err != nil || n != 0 {
		t.Fatalf("state with no carryoverTodos must be a no-op; got incremented=%d err=%v", n, err)
	}
}

func writeInboxTodo(t *testing.T, evolveDir, id string, weight float64, files []string) {
	t.Helper()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"id": id, "weight": weight, "files": files}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, id+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func repIDs(reps []triagecap.FleetCandidate) []string {
	out := make([]string, len(reps))
	for i, r := range reps {
		out[i] = r.ID
	}
	return out
}

func TestC536_009_TouchedPackagesBuildAndVetClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	failurelogPkg := filepath.Join(root, "go", "internal", "failurelog")
	triagecapPkg := filepath.Join(root, "go", "internal", "triagecap")
	cmdPkg := filepath.Join(root, "go", "cmd", "evolve")

	for _, pkg := range []string{failurelogPkg, triagecapPkg, cmdPkg} {
		_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", os.DevNull, pkg)
		if err != nil {
			t.Fatalf("failed to launch go build %s: %v", pkg, err)
		}
		if code != 0 {
			t.Errorf("go build %s must be clean; exit=%d stderr:\n%s", pkg, code, stderr)
		}
	}

	for _, pkg := range []string{failurelogPkg, triagecapPkg} {
		_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", pkg)
		if err != nil {
			t.Fatalf("failed to launch go vet %s: %v", pkg, err)
		}
		if code != 0 {
			t.Errorf("go vet %s must be clean; exit=%d stderr:\n%s", pkg, code, stderr)
		}
	}
}
