package retryladder_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func tryOf(phase, rung, class string, exit *int, in, out, cacheRead, cacheWrite int) llmcalls.Record {
	return llmcalls.Record{
		Agent: phase, Phase: phase, Rung: rung, FailureClass: class, ExitCode: exit,
		Tokens: cyclestate.TokenUsage{Input: in, Output: out, CacheRead: cacheRead, CacheWrite: cacheWrite},
	}
}

func exitCode(code int) *int { return &code }

func fixtureRuns() [][]llmcalls.Record {
	runA := []llmcalls.Record{
		tryOf("build", "R0", "", exitCode(1), 10, 5, 0, 1),
		tryOf("scout", "R0", "", exitCode(0), 1, 1, 1, 0),
		tryOf("build", "R1", "capability", exitCode(0), 20, 10, 100, 0),
	}
	runB := []llmcalls.Record{
		tryOf("build", "R0", "", exitCode(1), 3, 3, 0, 0),
		tryOf("audit", "R0", "", exitCode(0), 2, 0, 0, 0),
		tryOf("build", "R1", "capability", exitCode(1), 4, 4, 0, 0),
		tryOf("build", "R2", "capability", exitCode(0), 5, 5, 5, 5),
		tryOf("tdd", "R0", "", exitCode(85), 1, 0, 0, 0),
		tryOf("tdd", "R1", "quota", exitCode(0), 7, 7, 7, 7),
	}
	return [][]llmcalls.Record{runA, runB}
}

func TestLadderReport_ExactCountsPerRungAndClass(t *testing.T) {
	got := retryladder.BuildReport(fixtureRuns())
	want := []retryladder.ReportRow{
		{Rung: retryladder.R0, Class: "", Tries: 5, Fixes: 0, Tokens: cyclestate.TokenUsage{Input: 17, Output: 9, CacheRead: 1, CacheWrite: 1}},
		{Rung: retryladder.R1, Class: "capability", Tries: 2, Fixes: 1, Tokens: cyclestate.TokenUsage{Input: 24, Output: 14, CacheRead: 100}},
		{Rung: retryladder.R1, Class: "quota", Tries: 1, Fixes: 1, Tokens: cyclestate.TokenUsage{Input: 7, Output: 7, CacheRead: 7, CacheWrite: 7}},
		{Rung: retryladder.R2, Class: "capability", Tries: 1, Fixes: 1, Tokens: cyclestate.TokenUsage{Input: 5, Output: 5, CacheRead: 5, CacheWrite: 5}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report rows\n got  %+v\n want %+v", got, want)
	}
}

func TestLadderReport_FixRequiresARetryThatPassed(t *testing.T) {
	cases := []struct {
		name      string
		run       []llmcalls.Record
		wantFixes int
	}{
		{"first try that passes is not a fix", []llmcalls.Record{tryOf("build", "R0", "", exitCode(0), 1, 1, 0, 0)}, 0},
		{"retry that fails is not a fix", []llmcalls.Record{
			tryOf("build", "R0", "", exitCode(1), 1, 1, 0, 0),
			tryOf("build", "R1", "capability", exitCode(1), 1, 1, 0, 0),
		}, 0},
		{"retry with no recorded exit code is not a fix", []llmcalls.Record{
			tryOf("build", "R0", "", exitCode(1), 1, 1, 0, 0),
			tryOf("build", "R1", "capability", nil, 1, 1, 0, 0),
		}, 0},
		{"retry that passes is a fix", []llmcalls.Record{
			tryOf("build", "R0", "", exitCode(1), 1, 1, 0, 0),
			tryOf("build", "R1", "capability", exitCode(0), 1, 1, 0, 0),
		}, 1},
		{"another phase's first try does not make this phase's first try a retry", []llmcalls.Record{
			tryOf("scout", "R0", "", exitCode(0), 1, 1, 0, 0),
			tryOf("build", "R0", "", exitCode(0), 1, 1, 0, 0),
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixes := 0
			for _, row := range retryladder.BuildReport([][]llmcalls.Record{tc.run}) {
				fixes += row.Fixes
			}
			if fixes != tc.wantFixes {
				t.Fatalf("fixes = %d, want %d", fixes, tc.wantFixes)
			}
		})
	}
}

func TestLadderReport_RunsAreIndependent(t *testing.T) {
	first := []llmcalls.Record{tryOf("build", "R0", "", exitCode(1), 1, 1, 0, 0)}
	second := []llmcalls.Record{tryOf("build", "R0", "", exitCode(0), 1, 1, 0, 0)}
	rows := retryladder.BuildReport([][]llmcalls.Record{first, second})
	if len(rows) != 1 || rows[0].Tries != 2 || rows[0].Fixes != 0 {
		t.Fatalf("a passing first try in a second run must not count as a fix of the first run: %+v", rows)
	}
}

func TestLadderReport_RecordsWithoutARungAreLeftOut(t *testing.T) {
	legacy := []llmcalls.Record{
		tryOf("build", "", "", exitCode(1), 100, 100, 100, 100),
		tryOf("build", "R0", "", exitCode(0), 2, 0, 0, 0),
	}
	rows := retryladder.BuildReport([][]llmcalls.Record{legacy})
	want := []retryladder.ReportRow{{Rung: retryladder.R0, Tries: 1, Tokens: cyclestate.TokenUsage{Input: 2}}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("pre-ladder records must not enter the report\n got  %+v\n want %+v", rows, want)
	}
}

func TestLadderReport_NoRunsGiveNoRows(t *testing.T) {
	for name, runs := range map[string][][]llmcalls.Record{"nil": nil, "empty runs": {{}, {}}} {
		if rows := retryladder.BuildReport(runs); len(rows) != 0 {
			t.Errorf("%s: rows = %+v, want none", name, rows)
		}
	}
}

func TestLadderReport_BuildDoesNotMutateItsInput(t *testing.T) {
	runs := fixtureRuns()
	before, err := json.Marshal(runs)
	if err != nil {
		t.Fatal(err)
	}
	retryladder.BuildReport(runs)
	after, err := json.Marshal(runs)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("BuildReport mutated the records\n before %s\n after  %s", before, after)
	}
}

func TestLadderReport_RowJSONFieldNames(t *testing.T) {
	row := retryladder.ReportRow{Rung: retryladder.R1, Class: "quota", Tries: 3, Fixes: 2, Tokens: cyclestate.TokenUsage{Input: 4, Output: 5, CacheRead: 6, CacheWrite: 7}}
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"rung": "R1", "class": "quota", "tries": float64(3), "fixes": float64(2),
		"tokens": map[string]any{"input": float64(4), "output": float64(5), "cache_read": float64(6), "cache_write": float64(7)},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("row json = %s", raw)
	}
}

func writeLedger(t *testing.T, runsDir, name string, lines string) {
	t.Helper()
	dir := filepath.Join(runsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, llmcalls.Filename), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLadderReport_LoadRunsReadsEachRunDirLedgerInNameOrder(t *testing.T) {
	runsDir := t.TempDir()
	writeLedger(t, runsDir, "cycle-2", `{"phase":"build","rung":"R1","failure_class":"quota"}`+"\n")
	writeLedger(t, runsDir, "cycle-1", `{"phase":"scout","rung":"R0"}`+"\nnot json\n"+`{"phase":"tdd","rung":"R0"}`+"\n")
	if err := os.MkdirAll(filepath.Join(runsDir, "cycle-3"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runsDir, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := retryladder.LoadRuns(runsDir)
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2 (a dir without a ledger and a loose file are skipped)", len(runs))
	}
	if len(runs[0]) != 2 || runs[0][0].Phase != "scout" || runs[0][1].Phase != "tdd" {
		t.Fatalf("cycle-1 records (a malformed line is skipped) = %+v", runs[0])
	}
	if len(runs[1]) != 1 || runs[1][0].FailureClass != "quota" {
		t.Fatalf("cycle-2 records = %+v", runs[1])
	}
}

func TestLadderReport_LoadRunsFailsLoudlyOnAnAbsentRunsDir(t *testing.T) {
	if _, err := retryladder.LoadRuns(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("an absent runs dir must be an error, not an empty report")
	}
}

func TestLadderReport_LoadRunsEmptyDirIsNotAnError(t *testing.T) {
	runs, err := retryladder.LoadRuns(t.TempDir())
	if err != nil || len(runs) != 0 {
		t.Fatalf("empty runs dir: runs=%v err=%v", runs, err)
	}
}

func TestLadderReport_LoadRunsFailsLoudlyOnAnUnreadableLedger(t *testing.T) {
	runsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runsDir, "cycle-1", llmcalls.Filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := retryladder.LoadRuns(runsDir); err == nil {
		t.Fatal("a ledger that cannot be read must be an error, not a silent skip")
	}
}
