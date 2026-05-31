package cyclehealth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeArtifacts seeds a workspace with the three required reports.
// Body is 200-char filler so artifact_substance passes.
func writeArtifacts(t *testing.T, ws string) {
	t.Helper()
	body := strings.Repeat("x", 200)
	for _, name := range []string{"scout-report.md", "build-report.md", "audit-report.md"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeLedger writes a minimal valid ledger.jsonl with the three
// required role entries linked in a hash chain.
func writeLedger(t *testing.T, ws string, cycle int) {
	t.Helper()
	entries := []ledgerEntry{
		{Cycle: cycle, Role: "scout", Phase: "scout", Timestamp: 1000, Token: "tok-s", EntryHash: "h1"},
		{Cycle: cycle, Role: "builder", Phase: "build", Timestamp: 1100, Token: "tok-b", PrevHash: "h1", EntryHash: "h2"},
		{Cycle: cycle, Role: "auditor", Phase: "audit", Timestamp: 1200, Token: "tok-a", PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readReport(t *testing.T, ws string) Report {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, "cycle-health.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func freshWorkspace(t *testing.T, cycle int) string {
	t.Helper()
	ws := t.TempDir()
	writeArtifacts(t, ws)
	writeLedger(t, ws, cycle)
	return ws
}

// TestCheck_HealthyCycle_NoAnomalies — a properly-formed cycle's
// workspace produces zero anomalies and OverallFatal=false.
func TestCheck_HealthyCycle_NoAnomalies(t *testing.T) {
	ws := freshWorkspace(t, 1)
	r, err := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	if r.OverallFatal {
		t.Errorf("OverallFatal=true; anomalies=%+v", r.Anomalies)
	}
	if len(r.Anomalies) != 0 {
		t.Errorf("anomalies=%+v, want none", r.Anomalies)
	}
	// Confirm the report file was written.
	disk := readReport(t, ws)
	if disk.Cycle != 1 {
		t.Errorf("report.Cycle=%d, want 1", disk.Cycle)
	}
}

// TestCheck_MissingArtifact_Fatal — missing scout-report.md trips
// workspace_artifacts + cascades to OverallFatal.
func TestCheck_MissingArtifact_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	if err := os.Remove(filepath.Join(ws, "scout-report.md")); err != nil {
		t.Fatal(err)
	}
	r, _ := Check(Options{Cycle: 1, Workspace: ws})
	if !r.OverallFatal {
		t.Errorf("expected OverallFatal=true on missing artifact; got %+v", r.Anomalies)
	}
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "workspace_artifacts" && strings.Contains(a.Message, "scout-report.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected workspace_artifacts anomaly for scout-report.md; got %+v", r.Anomalies)
	}
}

// TestCheck_ShortArtifact_Fatal — body under 100 chars trips
// artifact_substance.
func TestCheck_ShortArtifact_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, _ := Check(Options{Cycle: 1, Workspace: ws})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "artifact_substance" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected artifact_substance anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_MissingLedgerRole_Fatal — drop auditor from ledger →
// ledger_completeness fires.
func TestCheck_MissingLedgerRole_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	// Rewrite ledger without auditor.
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "h1"},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, PrevHash: "h1", EntryHash: "h2"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "ledger_completeness" && strings.Contains(a.Message, "auditor") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ledger_completeness for auditor; got %+v", r.Anomalies)
	}
}

// TestCheck_FutureTimestamp_Fatal — a ledger entry with a timestamp
// > now+60 trips ledger_timestamps.
func TestCheck_FutureTimestamp_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	r, _ := Check(Options{
		Cycle: 1, Workspace: ws,
		NowFn: func() time.Time { return time.Unix(500, 0) }, // before all ledger entries
	})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "ledger_timestamps" && strings.Contains(a.Message, "future") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ledger_timestamps anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_HashChainBroken_Fatal — corrupt prev_hash on builder entry
// trips hash_chain.
func TestCheck_HashChainBroken_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "h1"},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, PrevHash: "BROKEN", EntryHash: "h2"},
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 1200, PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "hash_chain" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected hash_chain anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_ChecksumMismatch_Fatal — ledger SHA != file SHA →
// artifact_checksums fires.
func TestCheck_ChecksumMismatch_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	// Write a ledger that claims a known SHA, then change the file to
	// produce a different SHA.
	body := strings.Repeat("x", 200)
	h := sha256.Sum256([]byte(body))
	sha := hex.EncodeToString(h[:])
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "h1", SHA: sha},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, PrevHash: "h1", EntryHash: "h2", SHA: "wronghash"},
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 1200, PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "artifact_checksums" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected artifact_checksums anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_ChallengeTokenReused_Fatal — same token across two roles
// trips challenge_tokens.
func TestCheck_ChallengeTokenReused_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, Token: "dup-token", EntryHash: "h1"},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, Token: "dup-token", PrevHash: "h1", EntryHash: "h2"},
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 1200, PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "challenge_tokens" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected challenge_tokens anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_VelocityGap_Warn — phases > 30 min apart warn (not fatal).
func TestCheck_VelocityGap_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "h1"},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 5000, PrevHash: "h1", EntryHash: "h2"}, // 4000s gap
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 5500, PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(10000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "velocity" && a.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected velocity warn anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_CanaryFromOtherCycle_Warn — leftover canary-* file from a
// different cycle trips canary_files (warn).
func TestCheck_CanaryFromOtherCycle_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	if err := os.WriteFile(filepath.Join(ws, "canary-cycle-99"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "canary_files" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected canary_files anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_CostOverrun_Warn — ledger entry with cost > ceiling fires
// cost_envelope as warn.
func TestCheck_CostOverrun_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "h1", CostUSD: 0.5},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, PrevHash: "h1", EntryHash: "h2", CostUSD: 99.0},
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 1200, PrevHash: "h2", EntryHash: "h3", CostUSD: 0.3},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "cost_envelope" && a.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cost_envelope warn; got %+v", r.Anomalies)
	}
}

// TestCheck_DuplicateLedgerEntry_Fatal — two entries with the same
// entry_hash trips duplicate_ledger.
func TestCheck_DuplicateLedgerEntry_Fatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "dup-hash"},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, PrevHash: "dup-hash", EntryHash: "dup-hash"},
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 1200, PrevHash: "dup-hash", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "duplicate_ledger" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate_ledger anomaly; got %+v", r.Anomalies)
	}
}

// TestCheck_MissingCycle_Error — required-field validation.
func TestCheck_MissingCycle_Error(t *testing.T) {
	_, err := Check(Options{Workspace: t.TempDir()})
	if err == nil {
		t.Error("Check with Cycle=0: want error")
	}
}

// TestCheck_MissingWorkspace_Error — required-field validation.
func TestCheck_MissingWorkspace_Error(t *testing.T) {
	_, err := Check(Options{Cycle: 1})
	if err == nil {
		t.Error("Check with empty Workspace: want error")
	}
}

// TestCheck_OverallFatal_TrueOnAnyFatal — confirm OverallFatal flips
// on the first fatal anomaly.
func TestCheck_OverallFatal_TrueOnAnyFatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	os.Remove(filepath.Join(ws, "audit-report.md"))
	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	if !r.OverallFatal {
		t.Errorf("OverallFatal=false despite missing required artifact")
	}
}

// TestSignalNames_ReturnsTwelve — the public contract is "12
// signals"; guard against accidental removal.
func TestSignalNames_ReturnsTwelve(t *testing.T) {
	names := signalNames()
	if len(names) != 12 {
		t.Errorf("signalNames len=%d, want 12; got %v", len(names), names)
	}
}

func TestCheck_PhaseLatency_SlowPhase_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	
	// Write a slow phase entry to phase-timing.json
	entries := []phaseTimingEntry{
		{Phase: "build", DurationMS: 950000, Verdict: "PASS"}, // 950s (exceeds default 900s limit)
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, "phase-timing.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "phase_latency" {
			found = true
			if a.Severity != SeverityWarn {
				t.Errorf("expected SeverityWarn for slow phase, got %s", a.Severity)
			}
			if !strings.Contains(a.Message, "build phase latency 950000ms") {
				t.Errorf("unexpected anomaly message: %s", a.Message)
			}
		}
	}
	if !found {
		t.Error("expected phase_latency warning anomaly, not found")
	}

	// Test with custom ceiling override
	t.Setenv("EVOLVE_PHASE_LATENCY_CEILING_S", "1000") // 1000s ceiling
	r, err = Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r.Anomalies {
		if a.Signal == "phase_latency" {
			t.Errorf("unexpected slow phase anomaly with high ceiling override: %v", a)
		}
	}
}

func TestCheck_PhaseLatency_MissingFile_NoAnomaly(t *testing.T) {
	ws := freshWorkspace(t, 1)
	// phase-timing.json is missing by default
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r.Anomalies {
		if a.Signal == "phase_latency" {
			t.Errorf("unexpected phase_latency anomaly when file is missing: %v", a)
		}
	}
}

// TestSha256File_RoundTrip — direct unit test for the helper.
func TestSha256File_RoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x")
	os.WriteFile(p, []byte("hello"), 0o644)
	got, err := sha256File(p)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("hello")))
	if got != want {
		t.Errorf("hash=%q, want %q", got, want)
	}
}

// TestSha256File_MissingFile_Error — error path.
func TestSha256File_MissingFile_Error(t *testing.T) {
	if _, err := sha256File("/no/such/file"); err == nil {
		t.Error("want error for missing file")
	}
}

// TestShortHash — direct helper.
func TestShortHash(t *testing.T) {
	if shortHash("abc") != "abc" {
		t.Errorf("short input should pass through")
	}
	if shortHash("0123456789abcdef") != "01234567" {
		t.Errorf("long input should truncate to 8")
	}
}

// TestLoadLedger_FallsBackToParentDir — the ledger may live at the
// workspace's parent (project-root case).
func TestLoadLedger_FallsBackToParentDir(t *testing.T) {
	parent := t.TempDir()
	ws := filepath.Join(parent, "workspace")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeArtifacts(t, ws)
	// Put ledger in the parent, not the workspace.
	entries := []ledgerEntry{
		{Cycle: 1, Role: "scout", Phase: "scout", Timestamp: 1000, EntryHash: "h1"},
		{Cycle: 1, Role: "builder", Phase: "build", Timestamp: 1100, PrevHash: "h1", EntryHash: "h2"},
		{Cycle: 1, Role: "auditor", Phase: "audit", Timestamp: 1200, PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	os.WriteFile(filepath.Join(parent, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644)

	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	for _, a := range r.Anomalies {
		if a.Signal == "ledger_completeness" {
			t.Errorf("ledger should be found in parent dir; got %v", a)
		}
	}
}

// TestLoadLedger_Missing_NoFile — when no ledger exists anywhere, the
// completeness signal fires fatal.
func TestLoadLedger_Missing_NoFile(t *testing.T) {
	ws := t.TempDir()
	writeArtifacts(t, ws)
	r, _ := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "ledger_completeness" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ledger_completeness fatal; got %+v", r.Anomalies)
	}
}
