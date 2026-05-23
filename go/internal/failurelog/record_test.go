package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeState seeds <dir>/state.json with the given top-level shape and
// returns its path. Used by Record + Prune tests.
func writeState(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return path
}

// readState parses state.json from disk for assertions.
func readState(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	return m
}

func TestRecord_AppendsToEmpty(t *testing.T) {
	t.Parallel()
	path := writeState(t, `{"lastCycleNumber": 4}`)
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	rec, err := Record(path, "", RecordRequest{
		Cycle:          5,
		Classification: "infrastructure",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.Classification != InfrastructureTransient {
		t.Fatalf("class=%s want infrastructure-transient (normalized)", rec.Classification)
	}
	if rec.ExpiresAt != "2026-05-24T12:00:00Z" {
		t.Fatalf("expiresAt=%s want 1d after now", rec.ExpiresAt)
	}

	state := readState(t, path)
	entries := state["failedApproaches"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
	if state["lastCycleNumber"].(float64) != 5 {
		t.Fatalf("lastCycleNumber=%v want 5", state["lastCycleNumber"])
	}
}

func TestRecord_FIFOTrim(t *testing.T) {
	t.Parallel()
	// Seed state.json with 50 existing entries — appending one more
	// must drop the oldest.
	var entries []any
	for i := 0; i < 50; i++ {
		entries = append(entries, map[string]any{
			"cycle":          float64(i),
			"classification": "code-audit-fail",
		})
	}
	seed := map[string]any{
		"lastCycleNumber":  float64(50),
		"failedApproaches": entries,
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	raw, _ := json.Marshal(seed)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Record(path, "", RecordRequest{
		Cycle:          51,
		Classification: "audit-fail",
		Now:            time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	final := readState(t, path)
	finalEntries := final["failedApproaches"].([]any)
	if len(finalEntries) != 50 {
		t.Fatalf("entries=%d want 50 (FIFO cap)", len(finalEntries))
	}
	// The first (oldest, cycle=0) must be dropped; new entry (cycle=51) appended.
	first := finalEntries[0].(map[string]any)
	last := finalEntries[len(finalEntries)-1].(map[string]any)
	if first["cycle"].(float64) != 1 {
		t.Fatalf("first cycle=%v want 1 (cycle 0 should be trimmed)", first["cycle"])
	}
	if last["cycle"].(float64) != 51 {
		t.Fatalf("last cycle=%v want 51 (newest)", last["cycle"])
	}
}

func TestRecord_StateMissing(t *testing.T) {
	t.Parallel()
	_, err := Record(filepath.Join(t.TempDir(), "no-such-state.json"), "",
		RecordRequest{Cycle: 1, Classification: "audit-fail"})
	if err == nil {
		t.Fatalf("expected error when state.json missing")
	}
}

func TestRecord_StateMalformed(t *testing.T) {
	t.Parallel()
	path := writeState(t, `{not json`)
	_, err := Record(path, "", RecordRequest{Cycle: 1, Classification: "audit-fail"})
	if err == nil {
		t.Fatalf("expected error on bad JSON")
	}
}

func TestRecord_SummaryFromReport(t *testing.T) {
	t.Parallel()
	path := writeState(t, `{}`)
	runsDir := t.TempDir()
	cycleDir := filepath.Join(runsDir, "cycle-7")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	report := `# Cycle 7

## Failure Root Cause
Builder timed out after 600s.
The test suite has flaky races.
Three retries all failed.

## Verdict
**FAIL** — manual triage required.
`
	if err := os.WriteFile(filepath.Join(cycleDir, "orchestrator-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	rec, err := Record(path, runsDir, RecordRequest{
		Cycle:          7,
		Classification: "build-fail",
		Now:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.Summary == "" {
		t.Fatalf("summary should be non-empty when report exists")
	}
	if !contains(rec.Summary, "Builder timed out") {
		t.Fatalf("summary should include Failure section: %q", rec.Summary)
	}
}

func TestRecord_AtomicWriteFailure(t *testing.T) {
	// NOT t.Parallel — mutates package-level atomicWriteJSON.
	prev := atomicWriteJSON
	defer func() { atomicWriteJSON = prev }()
	atomicWriteJSON = func(string, map[string]any) error {
		return errors.New("synthetic write error")
	}
	path := writeState(t, `{}`)
	_, err := Record(path, "", RecordRequest{Cycle: 1, Classification: "audit-fail"})
	if err == nil {
		t.Fatalf("expected error from atomic write")
	}
}

func TestExtractSummary_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if s := extractSummary(path); s != "" {
		t.Fatalf("summary=%q want empty", s)
	}
}

func TestExtractSummary_MissingFile(t *testing.T) {
	t.Parallel()
	if s := extractSummary(filepath.Join(t.TempDir(), "nope.md")); s != "" {
		t.Fatalf("missing file should return empty: %q", s)
	}
}

func TestExtractSummary_NoSectionMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	body := "# Cycle 1\n\nNo recognized section markers here.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if s := extractSummary(path); s != "" {
		t.Fatalf("no-marker report should return empty: %q", s)
	}
}

func TestExtractSummary_StopsAtNextSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	body := `# Cycle 1
## Failure Root Cause
Line A
Line B
## Next Section
Should not appear.
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := extractSummary(path)
	if !contains(s, "Line A") || !contains(s, "Line B") {
		t.Fatalf("summary should include failure lines: %q", s)
	}
	if contains(s, "Should not appear") {
		t.Fatalf("summary should stop at next section: %q", s)
	}
}

func TestExtractSummary_HighlyVerboseTruncatedTo400(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	// Build a long single-line summary that exceeds 400 chars.
	long := ""
	for i := 0; i < 100; i++ {
		long += fmt.Sprintf("word%d ", i)
	}
	body := "## Failure Root Cause\n" + long + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := len(extractSummary(path)); got > 400 {
		t.Fatalf("summary len=%d want <=400", got)
	}
}

// contains is a tiny strings.Contains shim so the test reads naturally.
func contains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
