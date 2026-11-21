package logread

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func fixture(t *testing.T, events []signalcenter.Event) Reader {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	for i, e := range events {
		e.SchemaVersion, e.TS, e.Seq = signalcenter.SchemaVersion, fmt.Sprintf("2026-11-06T07:00:%02d.000000Z", i%60), uint64(i+1)
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	c := signalcenter.New()
	c.Subscribe(c.NDJSONSink(func(int) string {
		return filepath.Join(dir, signalcenter.StreamFileName)
	}))
	return Reader{RunDir: dir, Cycle: 4, Signals: c}
}

func outcome(phase, verdict string) signalcenter.Event {
	return signalcenter.Event{Phase: phase, Module: signalcenter.ModuleOrchestrator, Origin: "t.outcome", Kind: signalcenter.KindPhaseOutcome,
		Severity: signalcenter.SeverityInfo, Reason: phase, Fields: map[string]string{"verdict": verdict}}
}

func TestDigest_WithoutAStreamIsTheEmptyHeader(t *testing.T) {
	t.Parallel()
	got, err := Reader{RunDir: t.TempDir(), Cycle: 4}.Digest()
	if err != nil || len(got) != 1 || got[0] != "cycle 4: 0 phase outcomes, 0 failed" {
		t.Errorf("Digest of a cycle without signals = %v, %v", got, err)
	}
}

func TestDigest_UnknownVerdictPointsDown(t *testing.T) {
	t.Parallel()
	got, _ := fixture(t, []signalcenter.Event{outcome("scout", ""), outcome("build", "SKIPPED")}).Digest()
	if len(got) != 3 || !strings.Contains(got[1], "scout unknown -> evolve logs events --cycle 4 --phase scout") || strings.Contains(got[2], "->") {
		t.Errorf("Digest = %v, want an unknown verdict to point down and SKIPPED not to", got)
	}
}

func TestEvents_FromMovesTheWindowAndTheTruncationLineCarriesTheNextFrom(t *testing.T) {
	t.Parallel()
	var evs []signalcenter.Event
	for i := 0; i < 450; i++ {
		e := outcome("build", "PASS")
		e.Reason = fmt.Sprintf("rec-%03d", i+1)
		evs = append(evs, e)
	}
	r := fixture(t, evs)
	first, _ := r.Events(Query{Phase: "build"})
	if len(first) != BudgetD1 || !strings.HasSuffix(first[len(first)-1], "--phase build --from 200") {
		t.Fatalf("first window = %d lines, last %q", len(first), first[len(first)-1])
	}
	second, _ := r.Events(Query{Phase: "build", From: 200})
	if !strings.Contains(second[0], "rec-200") || !strings.HasSuffix(second[len(second)-1], "--from 399") {
		t.Errorf("second window starts %q ends %q", second[0], second[len(second)-1])
	}
	beyond, _ := r.Events(Query{From: 9999})
	if len(beyond) != 0 {
		t.Errorf("a window past the end = %v, want no lines", beyond)
	}
}

func TestQuery_LevelAndCodeFilter(t *testing.T) {
	t.Parallel()
	warn := outcome("build", "FAIL")
	warn.Severity, warn.Code = signalcenter.SeverityWarn, "BUILD_X_Y"
	r := fixture(t, []signalcenter.Event{outcome("scout", "PASS"), warn})
	if got, _ := r.Events(Query{Level: signalcenter.SeverityWarn}); len(got) != 1 {
		t.Errorf("level warn = %v", got)
	}
	if got, _ := r.Events(Query{Code: "BUILD_X_Y"}); len(got) != 1 || !strings.Contains(got[0], "BUILD_X_Y") {
		t.Errorf("code filter = %v", got)
	}
	if got, _ := r.Failures(Query{Phase: "scout"}); len(got) != 0 {
		t.Errorf("failures of an INFO-only phase = %v", got)
	}
}

func TestIndex_ListsFilesSortedAndBounded(t *testing.T) {
	t.Parallel()
	r := fixture(t, nil)
	for i := 0; i < 250; i++ {
		if err := os.WriteFile(filepath.Join(r.RunDir, fmt.Sprintf("f-%03d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.Index(0)
	if err != nil || len(got) != BudgetD1 || got[0] != "f-000.txt 1 bytes" || !strings.HasSuffix(got[len(got)-1], "evolve logs index --cycle 4 --from 200") {
		t.Errorf("Index = %d lines %v, first %q", len(got), err, got[0])
	}
	if _, err := (Reader{RunDir: filepath.Join(r.RunDir, "absent"), Cycle: 4}).Index(0); !errors.Is(err, ErrNoLog) {
		t.Errorf("Index of a missing dir = %v, want ErrNoLog", err)
	}
}

func TestShow_D2ReadsTheReportAndAMissingFileIsErrNoLog(t *testing.T) {
	t.Parallel()
	r := fixture(t, nil)
	if err := os.WriteFile(filepath.Join(r.RunDir, "build-report.md"), []byte("alpha\nbeta"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := r.Show("build", D2, 2)
	if err != nil || len(got) != 1 || got[0] != "beta" {
		t.Errorf("Show d2 from 2 = %v, %v (a last line without a newline still counts)", got, err)
	}
	if past, _ := r.Show("build", D2, 9); len(past) != 1 || !strings.HasPrefix(past[0], "no line at or after 9") {
		t.Errorf("Show past the end = %v", past)
	}
	if _, err := r.Show("audit", D3, 1); !errors.Is(err, ErrNoLog) {
		t.Errorf("Show of a missing phase file = %v, want ErrNoLog", err)
	}
}

func TestShow_D3PrefersTheCleanFileAndCutsALongLine(t *testing.T) {
	t.Parallel()
	r := fixture(t, nil)
	_ = os.WriteFile(filepath.Join(r.RunDir, "build-stdout.log"), []byte("raw\n"), 0o644)
	_ = os.WriteFile(filepath.Join(r.RunDir, "build-stdout.clean.txt"), []byte(strings.Repeat("z", 3*MaxLineBytes)+"\n"), 0o644)
	got, _ := r.Show("build", D3, 1)
	if len(got) != 1 || len(got[0]) != MaxLineBytes+len("…") || !strings.HasSuffix(got[0], "…") {
		t.Errorf("Show d3 = %d lines, first %d bytes", len(got), len(got[0]))
	}
}

func TestRecordShadow_WritesOneWarnForAReadOutsideTheAccess(t *testing.T) {
	t.Parallel()
	r := fixture(t, nil)
	access := &phasespec.LogAccess{Layers: []string{"D0"}}
	if !r.RecordShadow("scout", access, D3, "show") || r.RecordShadow("scout", access, D0, "digest") || r.RecordShadow("scout", nil, D3, "show") {
		t.Fatal("RecordShadow must report true only for an undeclared layer of a declared access")
	}
	chunk, err := signalcenter.ReadStream(filepath.Join(r.RunDir, signalcenter.StreamFileName), 0)
	if err != nil || len(chunk.Events) != 1 {
		t.Fatalf("stream = %v, %v", chunk.Events, err)
	}
	e := chunk.Events[0]
	if e.Code != signalcenter.CodeLogAccessShadow || e.Phase != "scout" || e.Severity != signalcenter.SeverityWarn || e.Fields["layer"] != "D3" || e.Fields["declared"] != "D0" || e.Fields["verb"] != "show" {
		t.Errorf("shadow event = %+v", e)
	}
}

func TestDispatchBlock_EndsWithOneInstruction(t *testing.T) {
	t.Parallel()
	got := fixture(t, []signalcenter.Event{outcome("scout", "PASS")}).DispatchBlock()
	if len(got) != 3 || !strings.HasPrefix(got[2], "Use evolve logs") {
		t.Errorf("DispatchBlock = %v", got)
	}
}

func TestParseLayer(t *testing.T) {
	t.Parallel()
	if l, ok := ParseLayer("d3"); !ok || l != D3 {
		t.Errorf("ParseLayer(d3) = %q, %v", l, ok)
	}
	if _, ok := ParseLayer("d9"); ok {
		t.Error("ParseLayer accepted d9")
	}
}

func TestDigest_SkipsNonOutcomeIncludesDurationAndErrorsOnDir(t *testing.T) {
	t.Parallel()
	ev := outcome("build", "PASS")
	ev.Fields["duration_ms"] = "42"
	other := signalcenter.Event{
		Phase: "build", Module: signalcenter.ModuleRunner, Origin: "t",
		Kind: signalcenter.KindRunnerWarning, Severity: signalcenter.SeverityWarn, Reason: "warn",
	}
	got, err := fixture(t, []signalcenter.Event{other, ev}).Digest()
	if err != nil || len(got) != 2 || !strings.Contains(got[1], "build PASS 42ms") {
		t.Fatalf("Digest = %v, %v", got, err)
	}
	badDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(badDir, signalcenter.StreamFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	badReader := Reader{RunDir: badDir, Cycle: 4}
	if _, err := badReader.Digest(); err == nil {
		t.Error("Digest on directory stream = nil, want error")
	}
	block := badReader.DispatchBlock()
	if len(block) != 1 || !strings.HasPrefix(block[0], "Use evolve logs") {
		t.Errorf("DispatchBlock on error = %v", block)
	}
}

func TestIndex_SkipsSubdirectories(t *testing.T) {
	t.Parallel()
	r := fixture(t, nil)
	if err := os.MkdirAll(filepath.Join(r.RunDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := r.Index(0)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	for _, row := range got {
		if strings.HasPrefix(row, "nested ") {
			t.Errorf("Index included directory row %q", row)
		}
	}
}

func TestShow_TruncatesOverBudgetAndErrorsOnDirectory(t *testing.T) {
	t.Parallel()
	r := fixture(t, nil)
	var b strings.Builder
	for i := 1; i <= 210; i++ {
		fmt.Fprintf(&b, "line-%03d\n", i)
	}
	if err := os.WriteFile(filepath.Join(r.RunDir, "build-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := r.Show("build", D2, 1)
	if err != nil || len(got) != BudgetD2 || !strings.HasSuffix(got[len(got)-1], "--from 200") {
		t.Fatalf("Show over budget = %d lines, %v", len(got), err)
	}
	if err := os.MkdirAll(filepath.Join(r.RunDir, "audit-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Show("audit", D2, 1); err == nil {
		t.Error("Show on directory = nil, want error")
	}
}

func TestReadWindow_MissingFileAndExact4096ByteLineWithoutNewline(t *testing.T) {
	t.Parallel()
	if _, _, err := readWindow(filepath.Join(t.TempDir(), "missing"), 1, 5); !errors.Is(err, ErrNoLog) {
		t.Errorf("readWindow missing = %v, want ErrNoLog", err)
	}
	r := fixture(t, nil)
	if err := os.WriteFile(filepath.Join(r.RunDir, "build-stdout.clean.txt"), []byte(strings.Repeat("a", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := r.Show("build", D3, 1)
	if err != nil || len(got) != 1 || len(got[0]) != MaxLineBytes+len("…") {
		t.Fatalf("Show 4096-byte EOF line = %d lines, %v", len(got), err)
	}
	if err := os.WriteFile(filepath.Join(r.RunDir, "scout-report.md"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	short, err := r.Show("scout", D2, 1)
	if err != nil || len(short) != 2 || short[1] != "two" {
		t.Errorf("Show trailing newline = %v, %v", short, err)
	}
}

func TestEvents_CapsLongLines(t *testing.T) {
	t.Parallel()
	ev := outcome("build", "PASS")
	ev.Reason = strings.Repeat("x", 2*MaxLineBytes)
	got, err := fixture(t, []signalcenter.Event{ev}).Events(Query{})
	if err != nil || len(got) != 1 || len(got[0]) != MaxLineBytes+len("…") || !strings.HasSuffix(got[0], "…") {
		t.Errorf("Events long line = %d lines, len=%d, err=%v", len(got), len(got[0]), err)
	}
}
