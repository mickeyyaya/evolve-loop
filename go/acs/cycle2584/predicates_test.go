//go:build acs

package cycle2584

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleN          = 7
	digestBudget    = 40
	eventsBudget    = 200
	failuresBudget  = 200
	showD3Budget    = 150
	indexBudget     = 200
	shadowCode      = "LOG_ACCESS_SHADOW"
	readerPhase     = "log-reader-demo"
	composedPrompt  = "COMPOSED PROMPT"
	promptAddedMax  = 48
	oneLineMaxBytes = 256 * 1024
)

var narrowFlag = regexp.MustCompile(`--(phase|kind|from|since|level|code|layer)\b`)

var instructionLine = regexp.MustCompile(`(?i)\buse\b[^\n]*evolve logs`)

var (
	binDir    string
	buildOnce sync.Once
	builtPath string
	buildFail error
)

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2584-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	binDir = dir
	return m.Run()
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		builtPath = filepath.Join(binDir, "evolve-under-test")
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", builtPath, "./cmd/evolve")
		if err != nil || code != 0 {
			buildFail = fmt.Errorf("go build ./cmd/evolve: code=%d err=%v\n%s", code, err, acsassert.OutputTail(stdout+stderr, 30))
		}
	})
	if buildFail != nil {
		t.Fatalf("%v", buildFail)
	}
	return builtPath
}

func evolveLogs(t *testing.T, root string, args ...string) (string, string, int) {
	t.Helper()
	full := append([]string{"logs"}, args...)
	full = append(full, "--project-root", root)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), full...)
	return stdout, stderr, code
}

func newRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, "docs", "architecture", "phase-registry.json")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "architecture", "phase-registry.json"), []byte(`{"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func runDir(root string) string {
	return filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycleN))
}

type signal map[string]any

func record(seq int, phase, kind, severity, reason string) signal {
	s := signal{
		"schema_version": "signal/1.0", "seq": seq, "pid": 1, "cycle": cycleN,
		"ts":     fmt.Sprintf("2026-11-06T07:%02d:%02d.000000Z", (seq/60)%60, seq%60),
		"run_id": "RUN7", "module": "orchestrator", "origin": "fixture",
		"kind": kind, "severity": severity, "reason": reason,
		"fields": map[string]string{"verdict": "PASS"},
	}
	if phase != "" {
		s["phase"] = phase
		s["attempt"] = 1
	}
	return s
}

func outcome(seq int, phase, verdict string) signal {
	severity := "INFO"
	if verdict != "PASS" {
		severity = "WARN"
	}
	s := record(seq, phase, "phase.outcome", severity, fmt.Sprintf("%s verdict=%s", phase, verdict))
	s["fields"] = map[string]string{"archetype": "plan", "duration_ms": "1200", "verdict": verdict}
	return s
}

func writeSignals(t *testing.T, root string, recs []signal) {
	t.Helper()
	var b bytes.Buffer
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), "signals.ndjson")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir(root), "signals.ndjson"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sixPhaseCycle() []signal {
	recs := []signal{}
	seq := 1
	for _, o := range []struct{ phase, verdict string }{
		{"scout", "PASS"}, {"triage", "PASS"}, {"tdd", "PASS"}, {"build", "FAIL"}, {"audit", "PASS"}, {"ship", "PASS"},
	} {
		recs = append(recs, outcome(seq, o.phase, o.verdict))
		seq++
	}
	for i := 0; i < 300; i++ {
		recs = append(recs, record(seq, "", "ledger.appended", "INFO", fmt.Sprintf("ledger-noise-%d", i)))
		seq++
	}
	for i := 0; i < 200; i++ {
		recs = append(recs, record(seq, "build", "isolation.violation", "WARN", fmt.Sprintf("violation-noise-%d", i)))
		seq++
	}
	return recs
}

func lines(out string) []string {
	trimmed := strings.TrimRight(out, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func requireOK(t *testing.T, label string, code int, stderr string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("%s exited %d, want 0; stderr:\n%s", label, code, acsassert.OutputTail(stderr, 20))
	}
}

func assertWithinBudget(t *testing.T, label, out string, budget int) []string {
	t.Helper()
	got := lines(out)
	if len(got) == 0 {
		t.Fatalf("%s printed nothing", label)
	}
	if len(got) > budget {
		t.Fatalf("%s printed %d lines, want %d or fewer", label, len(got), budget)
	}
	return got
}

func assertTruncationLine(t *testing.T, label string, got []string) {
	t.Helper()
	last := got[len(got)-1]
	if !strings.Contains(strings.ToLower(last), "truncated") {
		t.Errorf("%s: the last line is not a truncation line: %q", label, last)
		return
	}
	if !strings.Contains(last, "evolve logs") || !narrowFlag.MatchString(last) {
		t.Errorf("%s: the truncation line does not name a narrower query (evolve logs ... --phase|--kind|--from|--since|--level|--code|--layer): %q", label, last)
	}
}

func TestC2584_001_DigestIsOneBoundedLinePerPhaseOutcome(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())

	out, stderr, code := evolveLogs(t, root, "digest", "--cycle", "7")
	requireOK(t, "evolve logs digest", code, stderr)
	got := assertWithinBudget(t, "evolve logs digest", out, digestBudget)

	for _, o := range []struct{ phase, verdict string }{
		{"scout", "PASS"}, {"triage", "PASS"}, {"tdd", "PASS"}, {"build", "FAIL"}, {"audit", "PASS"}, {"ship", "PASS"},
	} {
		n := 0
		for _, line := range got {
			if strings.Contains(line, o.phase) && strings.Contains(line, o.verdict) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("digest has %d lines for phase %s with verdict %s, want exactly 1:\n%s", n, o.phase, o.verdict, out)
		}
	}
	if strings.Contains(out, "ledger-noise-") || strings.Contains(out, "violation-noise-") {
		t.Errorf("digest carries raw event noise instead of one line per phase outcome:\n%s", out)
	}
}

func TestC2584_002_DigestOfAFailedPhasePointsToTheNextLayer(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())

	out, stderr, code := evolveLogs(t, root, "digest", "--cycle", "7")
	requireOK(t, "evolve logs digest", code, stderr)

	var failedLine string
	for _, line := range lines(out) {
		if strings.Contains(line, "build") && strings.Contains(line, "FAIL") {
			failedLine = line
		}
	}
	if failedLine == "" {
		t.Fatalf("no digest line for the failed build phase:\n%s", out)
	}
	pointsDown := strings.Contains(failedLine, "evolve logs events") || strings.Contains(failedLine, "evolve logs show")
	if !pointsDown || !strings.Contains(failedLine, "--phase build") || !strings.Contains(failedLine, "--cycle 7") {
		t.Errorf("the failed-phase line has no pointer to D1 or D2 (evolve logs events|show --cycle 7 --phase build): %q", failedLine)
	}
}

func TestC2584_003_DigestStaysWithinBudgetWhenThereAreMoreOutcomesThanLines(t *testing.T) {
	root := newRoot(t)
	var recs []signal
	for i := 1; i <= 90; i++ {
		recs = append(recs, outcome(i, fmt.Sprintf("phase-%02d", i), "FAIL"))
	}
	writeSignals(t, root, recs)

	out, stderr, code := evolveLogs(t, root, "digest", "--cycle", "7")
	requireOK(t, "evolve logs digest", code, stderr)
	got := assertWithinBudget(t, "evolve logs digest over 90 outcomes", out, digestBudget)
	assertTruncationLine(t, "evolve logs digest over 90 outcomes", got)
}

func TestC2584_004_EventsStopAtTheLineBudgetAndNameANarrowerQuery(t *testing.T) {
	root := newRoot(t)
	var recs []signal
	seq := 1
	for i := 0; i < 1000; i++ {
		recs = append(recs, record(seq, "build", "bridge.warning", "INFO", fmt.Sprintf("build-event-%d", i)))
		seq++
	}
	for i := 0; i < 50; i++ {
		recs = append(recs, record(seq, "audit", "bridge.warning", "INFO", fmt.Sprintf("audit-only-sentinel-%d", i)))
		seq++
	}
	writeSignals(t, root, recs)

	out, stderr, code := evolveLogs(t, root, "events", "--cycle", "7", "--phase", "build")
	requireOK(t, "evolve logs events", code, stderr)
	got := assertWithinBudget(t, "evolve logs events over 1000 records", out, eventsBudget)
	assertTruncationLine(t, "evolve logs events over 1000 records", got)
	if strings.Contains(out, "audit-only-sentinel-") {
		t.Errorf("events --phase build returned a record of the audit phase")
	}
}

func TestC2584_005_EventsFilterByKeysAndAShortResultHasNoTruncationLine(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())

	out, stderr, code := evolveLogs(t, root, "events", "--cycle", "7", "--kind", "phase.outcome")
	requireOK(t, "evolve logs events --kind phase.outcome", code, stderr)
	got := assertWithinBudget(t, "evolve logs events --kind phase.outcome", out, eventsBudget)
	if len(got) != 6 {
		t.Errorf("events --kind phase.outcome printed %d lines, want exactly the 6 phase.outcome records and no truncation line:\n%s", len(got), out)
	}
	if strings.Contains(out, "ledger-noise-") || strings.Contains(strings.ToLower(out), "truncated") {
		t.Errorf("events --kind phase.outcome returned other kinds or a truncation line for a short result:\n%s", out)
	}
}

func TestC2584_006_FailuresStopAtTheLineBudgetAndSkipInfoRecords(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())

	out, stderr, code := evolveLogs(t, root, "failures", "--cycle", "7")
	requireOK(t, "evolve logs failures", code, stderr)
	got := assertWithinBudget(t, "evolve logs failures over 200 warnings", out, failuresBudget)
	assertTruncationLine(t, "evolve logs failures over 200 warnings", got)
	if strings.Contains(out, "ledger-noise-") {
		t.Errorf("failures returned an INFO ledger record")
	}

	small := newRoot(t)
	writeSignals(t, small, []signal{
		record(1, "scout", "ledger.appended", "INFO", "info-only-sentinel"),
		record(2, "build", "isolation.violation", "WARN", "warn-sentinel-a"),
		record(3, "build", "phase.aborted", "WARN", "warn-sentinel-b"),
	})
	short, stderr, code := evolveLogs(t, small, "failures", "--cycle", "7")
	requireOK(t, "evolve logs failures (short)", code, stderr)
	for _, want := range []string{"warn-sentinel-a", "warn-sentinel-b"} {
		if !strings.Contains(short, want) {
			t.Errorf("failures dropped %s:\n%s", want, short)
		}
	}
	if strings.Contains(short, "info-only-sentinel") || strings.Contains(strings.ToLower(short), "truncated") {
		t.Errorf("failures returned an INFO record or a truncation line for a short result:\n%s", short)
	}
}

func numberedFile(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s-%04d some raw tool output\n", prefix, i)
	}
	return b.String()
}

func TestC2584_007_ShowStopsAtTheD3BudgetAndFromMovesTheWindow(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), "build-stdout.log")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir(root), "build-stdout.log"), []byte(numberedFile("rawline", 5000)), 0o644); err != nil {
		t.Fatal(err)
	}

	out, stderr, code := evolveLogs(t, root, "show", "--cycle", "7", "--phase", "build", "--layer", "d3")
	requireOK(t, "evolve logs show", code, stderr)
	got := assertWithinBudget(t, "evolve logs show over a 5000-line file", out, showD3Budget)
	assertTruncationLine(t, "evolve logs show over a 5000-line file", got)
	if strings.Contains(out, "rawline-5000") {
		t.Errorf("show returned the end of the file as well as the start")
	}

	window, stderr, code := evolveLogs(t, root, "show", "--cycle", "7", "--phase", "build", "--layer", "d3", "--from", "4000")
	requireOK(t, "evolve logs show --from 4000", code, stderr)
	assertWithinBudget(t, "evolve logs show --from 4000", window, showD3Budget)
	if !strings.Contains(window, "rawline-4000") || strings.Contains(window, "rawline-0001") {
		t.Errorf("show --from 4000 did not start at line 4000:\n%s", acsassert.OutputTail(window, 5))
	}
}

func TestC2584_008_NoLogsVerbReturnsAWholeFileLargerThanItsBudget(t *testing.T) {
	root := newRoot(t)
	var recs []signal
	for i := 1; i <= 5000; i++ {
		recs = append(recs, record(i, "build", "isolation.violation", "WARN", fmt.Sprintf("big-%d", i)))
	}
	recs = append(recs, outcome(5001, "build", "FAIL"))
	writeSignals(t, root, recs)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), "build-stdout.log")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir(root), "build-stdout.log"), []byte(numberedFile("rawline", 5000)), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), fmt.Sprintf("extra-%03d.txt", i))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runDir(root), fmt.Sprintf("extra-%03d.txt", i)), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, v := range []struct {
		label  string
		args   []string
		budget int
	}{
		{"index", []string{"index", "--cycle", "7"}, indexBudget},
		{"digest", []string{"digest", "--cycle", "7"}, digestBudget},
		{"events", []string{"events", "--cycle", "7"}, eventsBudget},
		{"failures", []string{"failures", "--cycle", "7"}, failuresBudget},
		{"show", []string{"show", "--cycle", "7", "--phase", "build", "--layer", "d3"}, showD3Budget},
	} {
		out, stderr, code := evolveLogs(t, root, v.args...)
		requireOK(t, "evolve logs "+v.label, code, stderr)
		assertWithinBudget(t, "evolve logs "+v.label, out, v.budget)
	}

	_, stderr, code := evolveLogs(t, root, "cat", "--cycle", "7", "--phase", "build")
	if code == 0 {
		t.Errorf("evolve logs cat exited 0: a verb that dumps a file must not exist; stderr:\n%s", stderr)
	}
}

func TestC2584_009_AFileOfOneHugeLineIsStillBounded(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), "build-stdout.log")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir(root), "build-stdout.log"), []byte(strings.Repeat("A", 2<<20)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, stderr, code := evolveLogs(t, root, "show", "--cycle", "7", "--phase", "build", "--layer", "d3")
	requireOK(t, "evolve logs show over one 2 MiB line", code, stderr)
	if len(out) > oneLineMaxBytes {
		t.Errorf("show printed %d bytes for a file of one 2 MiB line, want %d or fewer: a line budget alone returns the whole file", len(out), oneLineMaxBytes)
	}
}

func readerSpecJSON(layers string) string {
	return `{"name":"` + readerPhase + `","archetype":"plan","log_access":{"layers":` + layers + `,"categories":["signals"],"scope":"cycle","line_budget":60}}`
}

func TestC2584_010_PhaseJSONAcceptsLogAccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, readerPhase, "phase.json")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, readerPhase, "phase.json"), []byte(readerSpecJSON(`["D0","D1"]`)), 0o644); err != nil {
		t.Fatal(err)
	}

	specs, warns := phasespec.DiscoverUserSpecs(dir)
	if len(specs) != 1 || len(warns) != 0 {
		t.Fatalf("DiscoverUserSpecs = %d specs, warnings %v; want 1 spec and none", len(specs), warns)
	}
	raw, err := json.Marshal(specs[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		LogAccess struct {
			Layers     []string `json:"layers"`
			Categories []string `json:"categories"`
			Scope      string   `json:"scope"`
			LineBudget int      `json:"line_budget"`
		} `json:"log_access"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	la := doc.LogAccess
	if strings.Join(la.Layers, ",") != "D0,D1" || strings.Join(la.Categories, ",") != "signals" || la.Scope != "cycle" || la.LineBudget != 60 {
		t.Errorf("the spec dropped or changed log_access: %s", raw)
	}
	if v := phasespec.ValidateUserSpec(specs[0]); containsLogAccess(v) {
		t.Errorf("ValidateUserSpec rejected a valid log_access: %v", v)
	}

	bad := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(bad, readerPhase, "phase.json")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, readerPhase, "phase.json"), []byte(readerSpecJSON(`["D0","D9"]`)), 0o644); err != nil {
		t.Fatal(err)
	}
	badSpecs, _ := phasespec.DiscoverUserSpecs(bad)
	if len(badSpecs) != 1 {
		t.Fatalf("DiscoverUserSpecs of the bad spec = %d specs, want 1", len(badSpecs))
	}
	if v := phasespec.ValidateUserSpec(badSpecs[0]); !containsLogAccess(v) {
		t.Errorf("ValidateUserSpec accepted the layer D9, which does not exist; violations: %v", v)
	}
}

func containsLogAccess(violations []string) bool {
	for _, v := range violations {
		if strings.Contains(v, "log_access") {
			return true
		}
	}
	return false
}

func readerRoot(t *testing.T, layers string) string {
	t.Helper()
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), "build-stdout.log")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir(root), "build-stdout.log"), []byte(numberedFile("rawline", 300)), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := phasespec.Roots(root)
	if len(roots) == 0 {
		t.Fatal("phasespec.Roots returned no root")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(roots[0], readerPhase, "phase.json")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roots[0], readerPhase, "phase.json"), []byte(readerSpecJSON(layers)), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func signalsText(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(runDir(root), "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestC2584_011_AReadOutsideTheDeclaredAccessRecordsASignalAndDoesNotBlock(t *testing.T) {
	root := readerRoot(t, `["D0","D1"]`)

	out, stderr, code := evolveLogs(t, root, "show", "--cycle", "7", "--phase", "build", "--layer", "d3", "--as", readerPhase)
	if code != 0 {
		t.Fatalf("a read outside the declared access exited %d in shadow mode, want 0 (no block); stderr:\n%s", code, acsassert.OutputTail(stderr, 20))
	}
	if !strings.Contains(out, "rawline-0001") {
		t.Errorf("shadow mode withheld the D3 lines it must still return:\n%s", acsassert.OutputTail(out, 5))
	}
	text := signalsText(t, root)
	if !strings.Contains(text, shadowCode) {
		t.Fatalf("no %s record in signals.ndjson after a D3 read by a phase that declares D0 and D1", shadowCode)
	}
	if !strings.Contains(text, readerPhase) {
		t.Errorf("the %s record does not name the reading phase %s", shadowCode, readerPhase)
	}
}

func TestC2584_012_AReadInsideTheDeclaredAccessRecordsNoSignal(t *testing.T) {
	root := readerRoot(t, `["D0","D1"]`)

	_, stderr, code := evolveLogs(t, root, "events", "--cycle", "7", "--kind", "phase.outcome", "--as", readerPhase)
	requireOK(t, "evolve logs events --as (inside the declared D1)", code, stderr)
	if strings.Contains(signalsText(t, root), shadowCode) {
		t.Errorf("a D1 read by a phase that declares D1 recorded a %s signal", shadowCode)
	}

	_, stderr, code = evolveLogs(t, root, "show", "--cycle", "7", "--phase", "build", "--layer", "d3", "--as", "phase-without-log-access")
	requireOK(t, "evolve logs show --as (a phase that declares nothing)", code, stderr)
	if strings.Contains(signalsText(t, root), shadowCode) {
		t.Errorf("a phase with no declared log_access recorded a %s signal", shadowCode)
	}
}

func TestC2584_013_TheShadowSignalCodeIsRegistered(t *testing.T) {
	if _, ok := signalcenter.IsRegistered(signalcenter.Code(shadowCode)); !ok {
		t.Errorf("signal code %s is not registered in the signal center", shadowCode)
	}
}

type promptHooks struct{ phase string }

func (h promptHooks) PhaseName() string       { return h.phase }
func (h promptHooks) AgentPromptName() string { return "evolve-" + h.phase }
func (h promptHooks) ArtifactFilename(core.PhaseRequest) string {
	return h.phase + "-report.md"
}
func (h promptHooks) DefaultModel() string { return "sonnet" }
func (h promptHooks) ComposePrompt(string, core.PhaseRequest) string {
	return composedPrompt
}
func (h promptHooks) Classify(string, core.PhaseRequest, core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, ""
}

type promptBridge struct{ prompts []string }

func (b *promptBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.prompts = append(b.prompts, req.Prompt)
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("# scout artifact\n"), 0o644)
	}
	return core.BridgeResponse{Stdout: "# scout artifact\n"}, nil
}

func (b *promptBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func dispatchedPrompt(t *testing.T, root string, spec *phasespec.PhaseSpec) string {
	t.Helper()
	bridge := &promptBridge{}
	var diag bytes.Buffer
	r := runner.New(runner.Options{
		Hooks:  promptHooks{phase: "scout"},
		Bridge: bridge,
		Prompts: prompts.NewFromFS(fstest.MapFS{
			"agents/evolve-scout.md": &fstest.MapFile{Data: []byte("---\nname: evolve-scout\n---\nSCOUT BODY\n")},
		}),
		Diag: log.Console{Out: &diag, Err: &diag},
	})
	_, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: cycleN, ProjectRoot: root, Workspace: runDir(root), Spec: spec,
	})
	if err != nil {
		t.Fatalf("runner.Run: %v\n%s", err, diag.String())
	}
	if len(bridge.prompts) != 1 {
		t.Fatalf("the bridge got %d launches, want 1", len(bridge.prompts))
	}
	return bridge.prompts[0]
}

func declaredSpec(t *testing.T) *phasespec.PhaseSpec {
	t.Helper()
	var spec phasespec.PhaseSpec
	if err := json.Unmarshal([]byte(`{"name":"scout","log_access":{"layers":["D0","D1"],"categories":["signals"],"scope":"cycle","line_budget":60}}`), &spec); err != nil {
		t.Fatal(err)
	}
	return &spec
}

func TestC2584_014_PromptInlinesOnlyTheD0DigestAndOneInstruction(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())
	if err := os.MkdirAll(filepath.Dir(filepath.Join(runDir(root), "build-stdout.log")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir(root), "build-stdout.log"), []byte("D3-RAW-SENTINEL\n"+numberedFile("rawline", 300)), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := dispatchedPrompt(t, root, declaredSpec(t))

	if !strings.Contains(prompt, composedPrompt) {
		t.Fatalf("the dispatched prompt lost the composed prompt:\n%s", prompt)
	}
	var digestLine string
	for _, line := range lines(prompt) {
		if strings.Contains(line, "build") && strings.Contains(line, "FAIL") {
			digestLine = line
		}
	}
	if digestLine == "" {
		t.Errorf("the prompt has no D0 digest line for the failed build phase:\n%s", prompt)
	}
	if n := len(instructionLine.FindAllString(prompt, -1)); n != 1 {
		t.Errorf("the prompt has %d instruction lines (Use ... evolve logs), want exactly 1", n)
	}
	for _, leak := range []string{"ledger-noise-", "violation-noise-", "D3-RAW-SENTINEL", "rawline-"} {
		if strings.Contains(prompt, leak) {
			t.Errorf("the prompt inlines %q: only the D0 digest and one instruction may be inlined", leak)
		}
	}
	if added := len(lines(prompt)) - 1; added > promptAddedMax {
		t.Errorf("the prompt grew by %d lines, want %d or fewer (D0 digest 40 lines plus the instruction)", added, promptAddedMax)
	}
}

func TestC2584_015_PromptIsUnchangedWhenThePhaseDeclaresNoLogAccess(t *testing.T) {
	root := newRoot(t)
	writeSignals(t, root, sixPhaseCycle())

	withoutSpec := dispatchedPrompt(t, root, nil)
	withEmptySpec := dispatchedPrompt(t, root, &phasespec.PhaseSpec{Name: "scout"})

	for label, got := range map[string]string{"no spec": withoutSpec, "a spec with no log_access": withEmptySpec} {
		if got != composedPrompt {
			t.Errorf("with %s the dispatched prompt is %q, want exactly the composed prompt", label, got)
		}
	}
}

func TestC2584_016_TheLogreadPackageGraduatesIntoTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce := filepath.Join(root, "go", ".apicover-enforce")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(enforce, "./internal/logread") {
		t.Errorf("go/.apicover-enforce does not list ./internal/logread")
	}
	if !acsassert.FileExists(t, filepath.Join(root, "go", "internal", "logread", "apicover_named_test.go")) {
		t.Errorf("go/internal/logread/apicover_named_test.go is missing: every exported symbol must be named in a real assertion")
	}
}
