package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/logread"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func runLogsWith(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := runLogs(args, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

func setupLogsFixture(t *testing.T, cycle int) string {
	t.Helper()
	root := t.TempDir()
	regPath := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, []byte(`{"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	phasePath := filepath.Join(root, ".evolve", "phases", "reader", "phase.json")
	if err := os.MkdirAll(filepath.Dir(phasePath), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `{"name":"reader","archetype":"plan","log_access":{"layers":["D0","D1"],"categories":["signals"],"scope":"cycle","line_budget":60}}`
	if err := os.WriteFile(phasePath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evs := []signalcenter.Event{
		{
			SchemaVersion: signalcenter.SchemaVersion,
			Seq:           1,
			TS:            "2026-11-06T07:00:00.000000Z",
			Cycle:         cycle,
			Phase:         "build",
			Module:        signalcenter.ModuleOrchestrator,
			Origin:        "fixture",
			Kind:          signalcenter.KindPhaseOutcome,
			Severity:      signalcenter.SeverityInfo,
			Reason:        "build passed",
			Fields:        map[string]string{"verdict": "PASS"},
		},
		{
			SchemaVersion: signalcenter.SchemaVersion,
			Seq:           2,
			TS:            "2026-11-06T07:00:01.000000Z",
			Cycle:         cycle,
			Phase:         "audit",
			Module:        signalcenter.ModuleOrchestrator,
			Origin:        "fixture",
			Kind:          signalcenter.KindPhaseOutcome,
			Severity:      signalcenter.SeverityWarn,
			Code:          "AUDIT_X_Y",
			Reason:        "audit warned",
			Fields:        map[string]string{"verdict": "FAIL"},
		},
	}
	var b strings.Builder
	for _, e := range evs {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(runDir, signalcenter.StreamFileName), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "build-report.md"), []byte("line-1\nline-2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "build-stdout.clean.txt"), []byte("raw-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunLogs_UsageAndValidation(t *testing.T) {
	t.Parallel()
	if rc, _, errOut := runLogsWith(); rc != 10 || !strings.Contains(errOut, "Usage: evolve logs") {
		t.Errorf("empty args: rc=%d err=%q", rc, errOut)
	}
	for _, helpArg := range []string{"help", "-h", "--help"} {
		if rc, out, _ := runLogsWith(helpArg); rc != 0 || !strings.Contains(out, "Usage: evolve logs") {
			t.Errorf("help %q: rc=%d out=%q", helpArg, rc, out)
		}
	}
	if rc, _, errOut := runLogsWith("cat"); rc != 10 || !strings.Contains(errOut, `unknown verb "cat"`) {
		t.Errorf("unknown verb: rc=%d err=%q", rc, errOut)
	}
	if rc, _, _ := runLogsWith("digest", "--bogus"); rc != 10 {
		t.Errorf("bad flag: rc=%d, want 10", rc)
	}
	if rc, _, errOut := runLogsWith("digest"); rc != 10 || !strings.Contains(errOut, "--cycle N is required") {
		t.Errorf("missing cycle: rc=%d err=%q", rc, errOut)
	}
	if rc, _, errOut := runLogsWith("show", "--cycle", "1"); rc != 10 || !strings.Contains(errOut, "--phase P is required") {
		t.Errorf("show without phase: rc=%d err=%q", rc, errOut)
	}
	if rc, _, errOut := runLogsWith("events", "--cycle", "1", "--level", "bogus"); rc != 10 || !strings.Contains(errOut, "--level") {
		t.Errorf("bad level: rc=%d err=%q", rc, errOut)
	}
	if rc, _, errOut := runLogsWith("show", "--cycle", "1", "--phase", "build", "--layer", "d1"); rc != 10 || !strings.Contains(errOut, "--layer") {
		t.Errorf("bad show layer: rc=%d err=%q", rc, errOut)
	}
}

func TestRunLogs_VerbsAndShadowAccess(t *testing.T) {
	t.Parallel()
	root := setupLogsFixture(t, 7)

	if rc, out, errOut := runLogsWith("digest", "--cycle", "7", "--as", "reader", "--project-root", root); rc != 0 || !strings.Contains(out, "cycle 7:") {
		t.Fatalf("digest: rc=%d out=%q err=%q", rc, out, errOut)
	}
	if rc, out, errOut := runLogsWith("events", "--cycle", "7", "--phase", "build", "--kind", string(signalcenter.KindPhaseOutcome), "--level", "info", "--from", "1", "--project-root", root); rc != 0 || !strings.Contains(out, "build passed") {
		t.Fatalf("events: rc=%d out=%q err=%q", rc, out, errOut)
	}
	if rc, out, errOut := runLogsWith("failures", "--cycle", "7", "--project-root", root); rc != 0 || !strings.Contains(out, "audit warned") {
		t.Fatalf("failures: rc=%d out=%q err=%q", rc, out, errOut)
	}
	if rc, out, errOut := runLogsWith("index", "--cycle", "7", "--project-root", root); rc != 0 || !strings.Contains(out, "build-report.md") {
		t.Fatalf("index: rc=%d out=%q err=%q", rc, out, errOut)
	}
	if rc, out, errOut := runLogsWith("show", "--cycle", "7", "--phase", "build", "--layer", "d2", "--as", "reader", "--project-root", root); rc != 0 || !strings.Contains(out, "line-1") {
		t.Fatalf("show d2: rc=%d out=%q err=%q", rc, out, errOut)
	}
	if rc, out, errOut := runLogsWith("show", "--cycle", "7", "--phase", "build", "--layer", "d3", "--project-root", root); rc != 0 || !strings.Contains(out, "raw-1") {
		t.Fatalf("show d3: rc=%d out=%q err=%q", rc, out, errOut)
	}
	if rc, _, errOut := runLogsWith("show", "--cycle", "7", "--phase", "missing", "--project-root", root); rc != 1 || !strings.Contains(errOut, "evolve logs show:") {
		t.Fatalf("show missing: rc=%d err=%q", rc, errOut)
	}

	chunk, err := signalcenter.ReadStream(filepath.Join(root, ".evolve", "runs", "cycle-7", signalcenter.StreamFileName), 0)
	if err != nil {
		t.Fatal(err)
	}
	var shadowCount int
	for _, ev := range chunk.Events {
		if ev.Code == signalcenter.CodeLogAccessShadow {
			shadowCount++
		}
	}
	if shadowCount != 1 {
		t.Errorf("shadow events = %d, want 1 (show d2 outside D0,D1)", shadowCount)
	}
}

func TestLogsHelpers_VerbLayerAndDeclaredLogAccess(t *testing.T) {
	t.Parallel()
	if verbLayer("digest", logread.D2) != logread.D0 || verbLayer("show", logread.D3) != logread.D3 || verbLayer("events", logread.D2) != logread.D1 {
		t.Error("verbLayer returned unexpected layer")
	}
	if got := declaredLogAccess(t.TempDir(), "reader"); got != nil {
		t.Errorf("declaredLogAccess without registry = %+v, want nil", got)
	}
	root := setupLogsFixture(t, 7)
	if got := declaredLogAccess(root, "absent-phase"); got != nil {
		t.Errorf("declaredLogAccess unknown phase = %+v, want nil", got)
	}
	if got := declaredLogAccess(root, "reader"); got == nil || len(got.Layers) != 2 {
		t.Errorf("declaredLogAccess reader = %+v, want D0,D1", got)
	}
	if f, ok := parseLogsFlags("digest", []string{"--cycle", "7"}, &bytes.Buffer{}); !ok || f.cycle != 7 {
		t.Errorf("parseLogsFlags valid = %+v, %v", f, ok)
	}
	if _, ok := parseLogsFlags("digest", []string{"--unknown-flag"}, &bytes.Buffer{}); ok {
		t.Error("parseLogsFlags accepted unknown flag")
	}
}
