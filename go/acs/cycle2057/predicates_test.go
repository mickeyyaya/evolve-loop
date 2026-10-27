//go:build acs

package cycle2057

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	evolog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var selectionKeys = []string{"ts", "run", "cycle", "phase", "dispatch", "cat", "component", "level", "code"}

var fullSelection = evolog.Selection{
	Run: "run-7", Cycle: 2057, Phase: "tdd", Dispatch: "run-7/2057/tdd/1",
	Cat: "loop", Component: "signalcenter", Code: "SC_SAMPLE",
}

var textKey = regexp.MustCompile(`(?:^| )([a-z_]+)=`)

func newPair(level string) (*slog.Logger, *bytes.Buffer, *bytes.Buffer) {
	var jsonOut, textOut bytes.Buffer
	logger := evolog.NewSelectionLogger(evolog.Sinks{JSON: &jsonOut, Text: &textOut, Level: level})
	return logger, &jsonOut, &textOut
}

func decodeOne(t *testing.T, raw string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("want exactly one JSON line, got %d: %q", len(lines), raw)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("JSON line does not parse: %v: %q", err, lines[0])
	}
	return rec
}

func textKeys(line string) map[string]bool {
	keys := map[string]bool{}
	for _, m := range textKey.FindAllStringSubmatch(line, -1) {
		keys[m[1]] = true
	}
	return keys
}

func TestC2057_001_JSONRecordCarriesAllNineSelectionKeysWithTheirValues(t *testing.T) {
	logger, jsonOut, _ := newPair("info")
	ctx := evolog.WithSelection(context.Background(), fullSelection)
	logger.InfoContext(ctx, "phase dispatched")
	rec := decodeOne(t, jsonOut.String())
	want := map[string]any{
		"run": "run-7", "cycle": float64(2057), "phase": "tdd", "dispatch": "run-7/2057/tdd/1",
		"cat": "loop", "component": "signalcenter", "level": "INFO", "code": "SC_SAMPLE",
	}
	for _, key := range selectionKeys {
		got, present := rec[key]
		if !present {
			t.Errorf("JSON record lacks key %q: %v", key, rec)
			continue
		}
		if wantValue, fixed := want[key]; fixed && got != wantValue {
			t.Errorf("key %q = %v, want %v", key, got, wantValue)
		}
	}
	ts, _ := rec["ts"].(string)
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("ts %q is not an RFC 3339 time: %v", ts, err)
	}
	if _, stale := rec["time"]; stale {
		t.Errorf("record keeps the slog default key \"time\" beside ts: %v", rec)
	}
	if rec["msg"] != "phase dispatched" {
		t.Errorf("msg = %v, want the logged message", rec["msg"])
	}
}

func TestC2057_002_RecordWithoutASelectionGetsNoFabricatedSelectionValues(t *testing.T) {
	logger, jsonOut, _ := newPair("info")
	logger.InfoContext(context.Background(), "bare record")
	rec := decodeOne(t, jsonOut.String())
	for _, key := range []string{"ts", "level", "msg"} {
		if _, present := rec[key]; !present {
			t.Errorf("bare record lacks %q: %v", key, rec)
		}
	}
	for _, key := range []string{"run", "cycle", "phase", "dispatch", "cat", "component", "code"} {
		if value, present := rec[key]; present {
			t.Errorf("bare record carries invented %s=%v", key, value)
		}
	}
}

func TestC2057_003_RecordAttributeNamedLikeASelectionKeyWinsAndAppearsOnce(t *testing.T) {
	logger, jsonOut, textOut := newPair("info")
	ctx := evolog.WithSelection(context.Background(), fullSelection)
	logger.InfoContext(ctx, "failure", slog.String("code", "SC_OVERRIDE"))
	raw := strings.TrimRight(jsonOut.String(), "\n")
	if n := strings.Count(raw, `"code":`); n != 1 {
		t.Errorf("JSON line has %d code keys, want 1: %s", n, raw)
	}
	if rec := decodeOne(t, jsonOut.String()); rec["code"] != "SC_OVERRIDE" {
		t.Errorf("code = %v, want the record attribute SC_OVERRIDE", rec["code"])
	}
	if n := strings.Count(textOut.String(), "code="); n != 1 {
		t.Errorf("text line has %d code keys, want 1: %s", n, textOut.String())
	}
}

func TestC2057_004_OneCallGivesTheSameRecordToTheTextAndJSONHandlers(t *testing.T) {
	logger, jsonOut, textOut := newPair("info")
	ctx := evolog.WithSelection(context.Background(), fullSelection)
	logger.InfoContext(ctx, "one call", slog.String("verdict", "PASS"))
	rec := decodeOne(t, jsonOut.String())
	textLines := strings.Split(strings.TrimRight(textOut.String(), "\n"), "\n")
	if len(textLines) != 1 || textLines[0] == "" {
		t.Fatalf("want exactly one text line, got %q", textOut.String())
	}
	keys := textKeys(textLines[0])
	for key := range rec {
		if !keys[key] {
			t.Errorf("JSON key %q is missing from the text line: %s", key, textLines[0])
		}
	}
	for key := range keys {
		if _, present := rec[key]; !present {
			t.Errorf("text key %q is missing from the JSON record: %v", key, rec)
		}
	}
	for _, fragment := range []string{"run=run-7", "cycle=2057", "phase=tdd", "dispatch=run-7/2057/tdd/1", "cat=loop", "component=signalcenter", "level=INFO", "code=SC_SAMPLE", "verdict=PASS"} {
		if !strings.Contains(textLines[0], fragment) {
			t.Errorf("text line lacks %q: %s", fragment, textLines[0])
		}
	}
}

func TestC2057_005_LevelFilterAppliesToBothHandlers(t *testing.T) {
	logger, jsonOut, textOut := newPair("warn")
	ctx := evolog.WithSelection(context.Background(), fullSelection)
	logger.InfoContext(ctx, "below the level")
	if jsonOut.Len() != 0 || textOut.Len() != 0 {
		t.Fatalf("an info record passed a warn logger: json=%q text=%q", jsonOut.String(), textOut.String())
	}
	logger.WarnContext(ctx, "at the level")
	if decodeOne(t, jsonOut.String())["level"] != "WARN" {
		t.Errorf("warn record missing from the JSON handler: %q", jsonOut.String())
	}
	if !strings.Contains(textOut.String(), "level=WARN") {
		t.Errorf("warn record missing from the text handler: %q", textOut.String())
	}
}

func TestC2057_006_ANilWriterSkipsThatHandlerAndKeepsTheOther(t *testing.T) {
	var textOut bytes.Buffer
	textOnly := evolog.NewSelectionLogger(evolog.Sinks{Text: &textOut, Level: "info"})
	textOnly.InfoContext(evolog.WithSelection(context.Background(), fullSelection), "text only")
	if !strings.Contains(textOut.String(), "msg=\"text only\"") {
		t.Errorf("text handler lost the record when the JSON writer is nil: %q", textOut.String())
	}
	var jsonOut bytes.Buffer
	jsonOnly := evolog.NewSelectionLogger(evolog.Sinks{JSON: &jsonOut, Level: "info"})
	jsonOnly.InfoContext(evolog.WithSelection(context.Background(), fullSelection), "json only")
	if decodeOne(t, jsonOut.String())["msg"] != "json only" {
		t.Errorf("JSON handler lost the record when the text writer is nil: %q", jsonOut.String())
	}
}

const goldenWarnLine = `[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=1636 phase=triage attempt=1 seq=1 origin=Orchestrator.recordPhaseOutcome — triage verdict=FAIL: top_n card names protected surface archetype=plan verdict=FAIL`

func goldenEvent() signalcenter.Event {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, "ORCHESTRATOR_PHASE_VERDICT_FAIL", "acs fixture: a phase recorded verdict FAIL")
	return signalcenter.Event{
		Seq: 1, Cycle: 1636, RunID: "01J", Phase: "triage", Attempt: 1, Module: signalcenter.ModuleOrchestrator,
		Origin: "Orchestrator.recordPhaseOutcome", Kind: signalcenter.KindPhaseOutcome, Code: "ORCHESTRATOR_PHASE_VERDICT_FAIL",
		Severity: signalcenter.SeverityWarn, Reason: "triage verdict=FAIL: top_n card names protected surface",
		Fields: map[string]string{"verdict": "FAIL", "archetype": "plan"},
	}
}

func TestC2057_007_TheConsoleSinkKeepsTheTextOfEachLoopLogLine(t *testing.T) {
	var out bytes.Buffer
	sink := signalcenter.ConsoleSink(&out)
	sink(goldenEvent())
	if got := out.String(); got != goldenWarnLine+"\n" {
		t.Errorf("loop-log line drifted:\n got %q\nwant %q", got, goldenWarnLine+"\n")
	}
	info := goldenEvent()
	info.Severity = signalcenter.SeverityInfo
	out.Reset()
	sink(info)
	if out.Len() != 0 {
		t.Errorf("the console sink wrote an INFO line: %q", out.String())
	}
}

func TestC2057_008_TheStderrSinkKeepsTheTextOfInfoAndMinimalLines(t *testing.T) {
	var out bytes.Buffer
	sink := signalcenter.StderrSink(&out)
	info := signalcenter.Event{Seq: 2, Module: signalcenter.ModuleLoop, Origin: "Batch.run", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "wave 1 open"}
	sink(info)
	sink(goldenEvent())
	want := "[loop] loop.wave INFO seq=2 origin=Batch.run — wave 1 open\n" + goldenWarnLine + "\n"
	if out.String() != want {
		t.Errorf("stderr sink text drifted:\n got %q\nwant %q", out.String(), want)
	}
}

func TestC2057_009_TheStderrSinkWritesThroughTheFactory(t *testing.T) {
	root := acsassert.RepoRoot(t)
	sinks := filepath.Join(root, "go", "internal", "signalcenter", "sinks.go")
	calls, err := acsassert.CountInGoFunc(sinks, "StderrSink", "NewSelectionLogger")
	if err != nil {
		t.Fatalf("read StderrSink: %v", err)
	}
	if calls < 1 {
		t.Errorf("StderrSink does not build its writer with log.NewSelectionLogger")
	}
	var out bytes.Buffer
	signalcenter.StderrSink(&out)(goldenEvent())
	if out.String() != goldenWarnLine+"\n" {
		t.Errorf("line text changed when the writer moved to the factory: %q", out.String())
	}
}

func logDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "log")
}

func TestC2057_010_ApicoverNamesAndCoversEveryExportOfTheFactory(t *testing.T) {
	dir := logDir(t)
	symbols, err := apicover.Enumerate(context.Background(), dir)
	if err != nil {
		t.Fatalf("enumerate %s: %v", dir, err)
	}
	exported := map[string]bool{}
	for _, s := range symbols {
		exported[s.Name] = true
	}
	for _, name := range []string{"NewSelectionLogger", "WithSelection", "Selection", "Sinks"} {
		if !exported[name] {
			t.Errorf("internal/log exports no %s", name)
		}
	}
	scratch := t.TempDir()
	profile := filepath.Join(scratch, "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var coverFunc string
	for _, args := range [][]string{
		{"test", "-count=1", "-coverprofile=" + profile, "./internal/log"},
		{"tool", "cover", "-func=" + profile},
	} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		cmd.Env = ipcenv.GoTestEnv()
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("go %s: %v\n%s%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
		}
		coverFunc = stdout.String()
	}
	coverPath := filepath.Join(scratch, "cover.func")
	if err := os.WriteFile(coverPath, []byte(coverFunc), 0o644); err != nil {
		t.Fatal(err)
	}
	var report bytes.Buffer
	code, err := apicover.Run(context.Background(), apicover.Config{Dirs: []string{dir}, CoverPath: coverPath, Enforce: true}, &report)
	if err != nil || code != 0 {
		t.Errorf("apicover on internal/log exited %d (err=%v):\n%s", code, err, report.String())
	}
}

func sectionOf(t *testing.T, path, headingFragment string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	start := -1
	level := 0
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, "#")
		depth := len(line) - len(trimmed)
		if depth > 0 && strings.Contains(strings.ToLower(trimmed), headingFragment) {
			start, level = i, depth
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimLeft(lines[i], "#")
		if depth := len(lines[i]) - len(trimmed); depth > 0 && depth <= level {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// acs-predicate: config-check
func TestC2057_011_EventChannelsDocNamesTheSelectionKeysAndTheFactory(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "event-channels.md")
	section := sectionOf(t, doc, "selection keys")
	if section == "" {
		t.Fatalf("event-channels.md has no section headed \"Selection keys\"")
	}
	for _, key := range selectionKeys {
		if !strings.Contains(section, "`"+key+"`") {
			t.Errorf("the Selection keys section does not name `%s`", key)
		}
	}
	for _, symbol := range []string{"NewSelectionLogger", "WithSelection"} {
		if !strings.Contains(section, symbol) {
			t.Errorf("the Selection keys section does not name %s", symbol)
		}
	}
}

// acs-predicate: config-check
func TestC2057_012_PackageNoteOfInternalLogNamesTheFactoryAndTheKeys(t *testing.T) {
	note := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "internal-log.md")
	for _, token := range []string{"NewSelectionLogger", "WithSelection", "`ts`", "`dispatch`", "`component`"} {
		acsassert.FileContains(t, note, token)
	}
}
