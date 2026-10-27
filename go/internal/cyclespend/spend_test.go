package cyclespend

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func ms(v int64) *int64 { return &v }

func TestSum_AddsTheDurationAndSplitsFreshFromCacheReadTokens(t *testing.T) {
	t.Parallel()
	records := []llmcalls.Record{
		{Phase: "build", DurationMS: ms(60_000), UsageStatus: llmcalls.UsageMeasured, Tokens: cyclestate.TokenUsage{Input: 10, Output: 20, CacheWrite: 30, CacheRead: 1000}},
		{Phase: "audit", DurationMS: ms(30_000), UsageStatus: llmcalls.UsagePartial, Tokens: cyclestate.TokenUsage{Input: 1, CacheRead: 5}},
		{Phase: "scout", UsageStatus: llmcalls.UsageUnavailable},
	}

	got := Sum(records)

	want := Spend{LLMMillis: 90_000, FreshTokens: 61, CacheReadTokens: 1005, UnmeasuredCalls: 2}
	if got != want {
		t.Errorf("Sum = %+v, want %+v", got, want)
	}
	if got.LLMMinutes() != 1.5 {
		t.Errorf("LLMMinutes = %v, want 1.5", got.LLMMinutes())
	}
}

func TestSpend_AddJoinsEachField(t *testing.T) {
	t.Parallel()
	a := Spend{LLMMillis: 1, FreshTokens: 2, CacheReadTokens: 3, UnmeasuredCalls: 4, AuditRounds: 5}
	if got := a.Add(a); got != (Spend{LLMMillis: 2, FreshTokens: 4, CacheReadTokens: 6, UnmeasuredCalls: 8, AuditRounds: 10}) {
		t.Errorf("Add = %+v", got)
	}
}

func TestRead_SumsTheLedgerOfAWorkspace(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	line := `{"phase":"build","duration_ms":120000,"usage_status":"measured","tokens":{"input":5,"output":5,"cache_read":7,"cache_write":0}}` + "\n" + "not json\n"
	if err := os.WriteFile(filepath.Join(ws, llmcalls.Filename), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(ws)

	if err != nil || got != (Spend{LLMMillis: 120_000, FreshTokens: 10, CacheReadTokens: 7, UnmeasuredCalls: 1}) {
		t.Errorf("Read = %+v, %v; want the malformed line counted as one unmeasured call", got, err)
	}
}

func TestRead_AWorkspaceWithNoLedgerSpentNothing(t *testing.T) {
	t.Parallel()
	got, err := Read(t.TempDir())
	if err != nil || got != (Spend{}) {
		t.Errorf("Read = %+v, %v; want zero and no error", got, err)
	}
}

func TestRead_AnUnreadableLedgerIsAnError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, llmcalls.Filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(ws); !errors.Is(err, ErrLedger) || errors.Is(err, ErrCycleState) || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read of a directory = %v, want a ledger read error", err)
	}
}

func TestRead_Cycle2035SpentTheNumbersOfItsIncidentRecord(t *testing.T) {
	t.Parallel()
	got, err := Read(filepath.Join("testdata", "cycle-2035"))

	if err != nil || got.FreshTokens != 4_338_241 || got.CacheReadTokens != 87_431_829 || got.UnmeasuredCalls != 0 {
		t.Fatalf("Read(cycle 2035) = %+v, %v", got, err)
	}
	if minutes := math.Round(got.LLMMinutes()*10) / 10; minutes != 160.1 {
		t.Errorf("LLM minutes = %v, want 160.1 (incident record F1.1)", minutes)
	}
}

func TestRead_TheAuditRoundsComeFromTheCycleState(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, StateFile), []byte(`{"cycle_id":7,"audit_dispatches":4}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(ws)

	if err != nil || got != (Spend{AuditRounds: 4}) {
		t.Errorf("Read = %+v, %v; want 4 audit rounds", got, err)
	}
}

func TestRead_AMalformedCycleStateIsAnErrorAndKeepsTheLedgerSpend(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	line := `{"phase":"build","duration_ms":60000,"usage_status":"measured","tokens":{}}` + "\n"
	if err := os.WriteFile(filepath.Join(ws, llmcalls.Filename), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, StateFile), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(ws)

	if !errors.Is(err, ErrCycleState) || errors.Is(err, ErrLedger) || got.LLMMillis != 60_000 {
		t.Errorf("Read = %+v, %v; want the ledger spend and a cycle-state parse error", got, err)
	}
}
