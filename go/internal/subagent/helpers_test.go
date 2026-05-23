package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedClock(t *testing.T, iso string) func() time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatalf("parse fixed clock: %v", err)
	}
	return func() time.Time { return parsed }
}

// --- AppendAbnormalEvent ---

func TestAppendAbnormalEvent_WritesJSONLine(t *testing.T) {
	ws := t.TempDir()
	clock := fixedClock(t, "2026-05-23T16:30:00Z")
	err := AppendAbnormalEvent(ws, AbnormalEvent{
		EventType:       "quota_likely",
		Severity:        "WARN",
		Details:         "cost=$18 >= threshold=$16",
		RemediationHint: "checkpoint via EVOLVE_CHECKPOINT_TRIGGERED=1",
	}, clock)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	line := strings.TrimSpace(string(body))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, line)
	}
	for _, want := range []string{
		`"event_type":"quota_likely"`,
		`"timestamp":"2026-05-23T16:30:00Z"`,
		`"source_phase":"subagent-run"`,
		`"severity":"WARN"`,
		`"details":"cost=$18 >= threshold=$16"`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in: %s", want, line)
		}
	}
}

func TestAppendAbnormalEvent_AppendsMultipleLines(t *testing.T) {
	ws := t.TempDir()
	clock := fixedClock(t, "2026-05-23T16:30:00Z")
	for i := 0; i < 3; i++ {
		if err := AppendAbnormalEvent(ws, AbnormalEvent{EventType: "x", Severity: "INFO"}, clock); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	body, _ := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if n := strings.Count(string(body), "\n"); n != 3 {
		t.Errorf("expected 3 lines, got %d", n)
	}
}

func TestAppendAbnormalEvent_MissingWorkspaceIsNoop(t *testing.T) {
	err := AppendAbnormalEvent("/nonexistent/path/here", AbnormalEvent{EventType: "x"}, nil)
	if err != nil {
		t.Errorf("missing workspace should be silent, got %v", err)
	}
}

func TestAppendAbnormalEvent_FileNotDirIsNoop(t *testing.T) {
	tmp := t.TempDir()
	notDir := filepath.Join(tmp, "regular-file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := AppendAbnormalEvent(notDir, AbnormalEvent{EventType: "x"}, nil)
	if err != nil {
		t.Errorf("file (not dir) should be no-op, got %v", err)
	}
}

func TestAppendAbnormalEvent_DefaultSourcePhase(t *testing.T) {
	ws := t.TempDir()
	_ = AppendAbnormalEvent(ws, AbnormalEvent{EventType: "x"}, nil)
	body, _ := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if !strings.Contains(string(body), `"source_phase":"subagent-run"`) {
		t.Errorf("missing default source_phase: %s", body)
	}
}

func TestAppendAbnormalEvent_CustomSourcePhase(t *testing.T) {
	ws := t.TempDir()
	_ = AppendAbnormalEvent(ws, AbnormalEvent{EventType: "x", SourcePhase: "aggregator"}, nil)
	body, _ := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if !strings.Contains(string(body), `"source_phase":"aggregator"`) {
		t.Errorf("missing custom source_phase: %s", body)
	}
}

func TestAppendAbnormalEvent_EscapesQuotesInDetails(t *testing.T) {
	ws := t.TempDir()
	_ = AppendAbnormalEvent(ws, AbnormalEvent{
		EventType: "x",
		Details:   `value with "embedded" quotes`,
	}, nil)
	body, _ := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if !strings.Contains(string(body), `"details":"value with \"embedded\" quotes"`) {
		t.Errorf("quotes not escaped: %s", body)
	}
	// Result must still be valid JSON.
	line := strings.TrimSpace(string(body))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Errorf("not parseable: %v\n%s", err, line)
	}
}

// --- QuotaLikely ---

func TestQuotaLikely_NonEmptyStderrReturnsFalse(t *testing.T) {
	for _, tail := range []string{"some error", "  partial  ", "x"} {
		got := QuotaLikely(
			QuotaLikelyRequest{StderrTail: tail, DangerPct: 0, BatchBudgetCapUSD: 20},
			QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 100, true }},
		)
		if got {
			t.Errorf("tail %q: expected false, got true", tail)
		}
	}
}

func TestQuotaLikely_EmptyTailVariantsAcceptable(t *testing.T) {
	for _, tail := range []string{"", "   ", "\t\n", "<empty>"} {
		got := QuotaLikely(
			QuotaLikelyRequest{StderrTail: tail, DangerPct: 50, BatchBudgetCapUSD: 20},
			QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 15, true }},
		)
		if !got {
			t.Errorf("tail %q: expected quota-likely, got false", tail)
		}
	}
}

func TestQuotaLikely_DangerPct100Disables(t *testing.T) {
	got := QuotaLikely(
		QuotaLikelyRequest{StderrTail: "", DangerPct: 100, BatchBudgetCapUSD: 20},
		QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 19, true }},
	)
	if got {
		t.Errorf("DangerPct=100 should disable, got true")
	}
}

func TestQuotaLikely_DangerPct0AlwaysTrue(t *testing.T) {
	// 0% threshold ⇒ any non-negative cost triggers.
	got := QuotaLikely(
		QuotaLikelyRequest{StderrTail: "", DangerPct: 0, BatchBudgetCapUSD: 20},
		QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 0, true }},
	)
	if !got {
		t.Errorf("DangerPct=0 should always classify, got false")
	}
}

func TestQuotaLikely_NoCostLookupReturnsFalse(t *testing.T) {
	got := QuotaLikely(
		QuotaLikelyRequest{StderrTail: "", DangerPct: 50, BatchBudgetCapUSD: 20},
		QuotaLikelyOptions{},
	)
	if got {
		t.Errorf("nil CostLookup should return false (conservative)")
	}
}

func TestQuotaLikely_CostLookupFailureReturnsFalse(t *testing.T) {
	got := QuotaLikely(
		QuotaLikelyRequest{StderrTail: "", DangerPct: 50, BatchBudgetCapUSD: 20},
		QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 0, false }},
	)
	if got {
		t.Errorf("CostLookup false should return false")
	}
}

func TestQuotaLikely_ThresholdExactlyMet(t *testing.T) {
	// 80% of 20 = 16; cost exactly 16 should still trigger (>=).
	got := QuotaLikely(
		QuotaLikelyRequest{StderrTail: "", DangerPct: 80, BatchBudgetCapUSD: 20},
		QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 16, true }},
	)
	if !got {
		t.Errorf("cost==threshold should trigger, got false")
	}
}

func TestQuotaLikely_BelowThresholdFalse(t *testing.T) {
	got := QuotaLikely(
		QuotaLikelyRequest{StderrTail: "", DangerPct: 80, BatchBudgetCapUSD: 20},
		QuotaLikelyOptions{CostLookup: func(int) (float64, bool) { return 15.99, true }},
	)
	if got {
		t.Errorf("cost<threshold should not trigger")
	}
}

// --- WriteFanoutLedgerEntry ---

func TestWriteFanoutLedgerEntry_BasicLine(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	clock := fixedClock(t, "2026-05-23T16:30:00Z")

	agg := filepath.Join(tmp, "aggregate.md")
	if err := os.WriteFile(agg, []byte("body bytes\n"), 0o644); err != nil {
		t.Fatalf("seed agg: %v", err)
	}

	err := WriteFanoutLedgerEntry(ledger, FanoutLedgerEntry{
		Cycle:          42,
		Agent:          "scout",
		ChallengeToken: "abc123def4567890",
		GitHEAD:        "aaaa",
		TreeStateSHA:   "bbbb",
		WorkerNames:    []string{"codebase", "docs", "research"},
		WorkerCount:    3,
		ExitCode:       0,
		AggregatePath:  agg,
		QualityTier:    "full",
	}, clock)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	body, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	line := strings.TrimSpace(string(body))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, line)
	}
	if parsed["kind"] != "agent_fanout" {
		t.Errorf("kind=%v, want agent_fanout", parsed["kind"])
	}
	if parsed["worker_count"].(float64) != 3 {
		t.Errorf("worker_count=%v, want 3", parsed["worker_count"])
	}
	if parsed["entry_seq"].(float64) != 0 {
		t.Errorf("first entry should have seq=0, got %v", parsed["entry_seq"])
	}
	if parsed["prev_hash"] != ledgerZeroSeed {
		t.Errorf("first entry prev_hash should be zero seed, got %v", parsed["prev_hash"])
	}
	if parsed["artifact_sha256"] == "" {
		t.Errorf("expected artifact_sha256 to be computed")
	}
	workers := parsed["workers"].([]interface{})
	if len(workers) != 3 || workers[0] != "codebase" {
		t.Errorf("workers array malformed: %v", workers)
	}

	// Tip file written.
	tip, err := os.ReadFile(filepath.Join(tmp, "ledger.tip"))
	if err != nil {
		t.Fatalf("read tip: %v", err)
	}
	if !strings.HasPrefix(string(tip), "0:") {
		t.Errorf("tip should start with seq, got %s", tip)
	}
}

func TestWriteFanoutLedgerEntry_ChainAdvances(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	clock := fixedClock(t, "2026-05-23T16:30:00Z")

	for i := 0; i < 3; i++ {
		err := WriteFanoutLedgerEntry(ledger, FanoutLedgerEntry{
			Cycle: i, Agent: "scout",
			ChallengeToken: "tok",
			WorkerNames:    []string{"a"},
			WorkerCount:    1,
		}, clock)
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
	}
	body, _ := os.ReadFile(ledger)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// Each entry's prev_hash must be the sha256 of the previous line.
	for i := 1; i < 3; i++ {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			t.Fatalf("parse line %d: %v", i, err)
		}
		gotPrev := entry["prev_hash"].(string)
		wantPrev := sha256Hex(lines[i-1])
		if gotPrev != wantPrev {
			t.Errorf("line %d prev_hash=%s, want %s", i, gotPrev, wantPrev)
		}
		if int(entry["entry_seq"].(float64)) != i {
			t.Errorf("line %d entry_seq=%v, want %d", i, entry["entry_seq"], i)
		}
	}
}

func TestWriteFanoutLedgerEntry_NoAggregateEmptyArtifactSHA(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	err := WriteFanoutLedgerEntry(ledger, FanoutLedgerEntry{
		Cycle: 1, Agent: "scout", ChallengeToken: "x",
		WorkerNames: []string{}, WorkerCount: 0,
		AggregatePath: "", // no aggregate
	}, nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, _ := os.ReadFile(ledger)
	var parsed map[string]interface{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(string(body))), &parsed)
	if parsed["artifact_sha256"] != "" {
		t.Errorf("empty aggregate should produce empty sha, got %v", parsed["artifact_sha256"])
	}
}

func TestWriteFanoutLedgerEntry_QualityTierDefaultUnknown(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	_ = WriteFanoutLedgerEntry(ledger, FanoutLedgerEntry{
		Cycle: 1, Agent: "scout", ChallengeToken: "x",
		WorkerNames: []string{"w"}, WorkerCount: 1,
	}, nil)
	body, _ := os.ReadFile(ledger)
	if !strings.Contains(string(body), `"quality_tier":"unknown"`) {
		t.Errorf("expected unknown default: %s", body)
	}
}

func TestWriteFanoutLedgerEntry_AggregateUnreadableEmptyHash(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	_ = WriteFanoutLedgerEntry(ledger, FanoutLedgerEntry{
		Cycle: 1, Agent: "scout", ChallengeToken: "x",
		WorkerNames: []string{"w"}, WorkerCount: 1,
		AggregatePath: "/no/such/file.md",
	}, nil)
	body, _ := os.ReadFile(ledger)
	if !strings.Contains(string(body), `"artifact_sha256":""`) {
		t.Errorf("unreadable aggregate should leave sha empty: %s", body)
	}
}

func TestWriteFanoutLedgerEntry_MkdirError(t *testing.T) {
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "file-not-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	bad := filepath.Join(blocker, "sub", "ledger.jsonl")
	err := WriteFanoutLedgerEntry(bad, FanoutLedgerEntry{
		Cycle: 1, Agent: "x", ChallengeToken: "t",
	}, nil)
	if err == nil {
		t.Fatalf("expected mkdir error")
	}
}

// --- shared helpers ---

func TestReadChainLink_EmptyLedger(t *testing.T) {
	tmp := t.TempDir()
	prev, seq, err := readChainLink(filepath.Join(tmp, "missing.jsonl"))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if prev != ledgerZeroSeed || seq != 0 {
		t.Errorf("expected zero seed + seq 0, got %s %d", prev, seq)
	}
}

func TestReadChainLink_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "ledger.jsonl")
	_ = os.WriteFile(p, []byte{}, 0o644)
	prev, seq, _ := readChainLink(p)
	if prev != ledgerZeroSeed || seq != 0 {
		t.Errorf("got %s %d", prev, seq)
	}
}

func TestReadChainLink_SingleLine(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "ledger.jsonl")
	_ = os.WriteFile(p, []byte("first entry\n"), 0o644)
	prev, seq, _ := readChainLink(p)
	if prev != sha256Hex("first entry") {
		t.Errorf("prev hash mismatch")
	}
	if seq != 1 {
		t.Errorf("seq=%d, want 1", seq)
	}
}

func TestJSONStringEscape(t *testing.T) {
	tests := []struct{ in, want string }{
		{`simple`, `simple`},
		{`with "quotes"`, `with \"quotes\"`},
		{`with \\back`, `with \\\\back`},
		{"with\nnewline", `with\nnewline`},
		{"with\ttab", `with\ttab`},
		{"with\rcr", `with\rcr`},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := jsonStringEscape(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSHA256Hex(t *testing.T) {
	// Known vector: sha256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	if sha256Hex("") != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("sha256Hex empty mismatch")
	}
}

func TestParseQuotaDangerPct(t *testing.T) {
	tests := []struct {
		raw  string
		want int
	}{
		{"", 80},
		{"50", 50},
		{"0", 0},
		{"100", 100},
		{"not-a-number", 80},
		{"-5", 80},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			if got := ParseQuotaDangerPct(tc.raw); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseBatchBudgetCap(t *testing.T) {
	tests := []struct {
		raw  string
		want float64
	}{
		{"", 20.00},
		{"15.50", 15.50},
		{"100", 100},
		{"oops", 20.00},
		{"0", 20.00},
		{"-1", 20.00},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			if got := ParseBatchBudgetCap(tc.raw); got != tc.want {
				t.Errorf("got %f, want %f", got, tc.want)
			}
		})
	}
}
