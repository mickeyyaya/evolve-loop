package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func writeModelsFixture(t *testing.T, root, cycle string, recs ...llmcalls.Record) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		if err := llmcalls.AppendWorkspace(ws, rec); err != nil {
			t.Fatal(err)
		}
	}
}

func modelsRecord(phase string, attempt int, tier, selector, served string) llmcalls.Record {
	return llmcalls.Record{
		TS: "2026-10-20T00:00:00Z", Agent: phase, Phase: phase, CLI: "agy-tmux",
		Model: tier, RequestedModel: tier, DispatchedModel: selector, DispatchSource: llmcalls.DispatchArgv,
		Attempt: attempt, ServedModel: served,
	}
}

func rowOf(t *testing.T, out, phase string, attempt string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == phase && fields[1] == attempt {
			return line
		}
	}
	t.Fatalf("no row for phase %q attempt %s in:\n%s", phase, attempt, out)
	return ""
}

func TestRunCycleModels_PrintsTierSelectorAndServedModelPerPhase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "7",
		modelsRecord("scout", 1, "balanced", "gemini-3.8-flash-high", "Gemini 3.8 Flash (High)"),
		modelsRecord("build", 1, "deep", "gemini-4-argon-medium", "Gemini 4 Argon (Medium)"),
	)

	var out, errb bytes.Buffer
	if code := runCycleModels([]string{"--project-root", root, "--cycle", "7"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	scout := rowOf(t, out.String(), "scout", "1")
	for _, want := range []string{"balanced", "gemini-3.8-flash-high", "Gemini 3.8 Flash (High)"} {
		if !strings.Contains(scout, want) {
			t.Errorf("scout row missing %q: %q", want, scout)
		}
	}
	build := rowOf(t, out.String(), "build", "1")
	for _, want := range []string{"deep", "gemini-4-argon-medium", "Gemini 4 Argon (Medium)"} {
		if !strings.Contains(build, want) {
			t.Errorf("build row missing %q: %q", want, build)
		}
	}
	if strings.Contains(scout, "deep") || strings.Contains(scout, "Argon") {
		t.Errorf("scout row leaks the build row's values: %q", scout)
	}
}

func TestRunCycleModels_ARecordWithoutAServedModelPrintsUnreported(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "8",
		modelsRecord("scout", 1, "balanced", "gemini-3.8-flash-high", ""),
		modelsRecord("build", 1, "deep", "claude-opus-5-5", "claude-opus-5-5"),
	)

	var out, errb bytes.Buffer
	if code := runCycleModels([]string{"--project-root", root, "--cycle", "8"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if scout := rowOf(t, out.String(), "scout", "1"); !strings.Contains(scout, "unreported") {
		t.Errorf("a record with no served model must print unreported: %q", scout)
	}
	if build := rowOf(t, out.String(), "build", "1"); strings.Contains(build, "unreported") {
		t.Errorf("a record that reports its served model must not print unreported: %q", build)
	}
}

func TestRunCycleModels_EachAttemptOfAPhaseGetsItsOwnRow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "9",
		modelsRecord("build", 1, "deep", "gemini-4-argon-medium", ""),
		modelsRecord("build", 2, "deep", "claude-opus-5-5", "claude-opus-5-5"),
	)

	var out, errb bytes.Buffer
	if code := runCycleModels([]string{"--project-root", root, "--cycle", "9"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	first := rowOf(t, out.String(), "build", "1")
	second := rowOf(t, out.String(), "build", "2")
	if !strings.Contains(first, "gemini-4-argon-medium") || !strings.Contains(first, "unreported") {
		t.Errorf("attempt 1 row = %q, want the argon selector and unreported", first)
	}
	if !strings.Contains(second, "claude-opus-5-5") || strings.Contains(second, "unreported") {
		t.Errorf("attempt 2 row = %q, want the claude selector and a reported model", second)
	}
}

func TestRunCycleModels_NoRecordsExitsOne(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(root string)
	}{
		{"workspace without a ledger", func(root string) {
			if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-3"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"ledger holding only unparseable lines", func(root string) {
			ws := filepath.Join(root, ".evolve", "runs", "cycle-3")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, llmcalls.Filename), []byte("not json\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(root)
			var out, errb bytes.Buffer
			if code := runCycleModels([]string{"--project-root", root, "--cycle", "3"}, &out, &errb); code != 1 {
				t.Fatalf("exit=%d, want 1; stdout=%q stderr=%q", code, out.String(), errb.String())
			}
			if errb.Len() == 0 {
				t.Errorf("an empty result must say why on stderr")
			}
		})
	}
}

func TestRunCycleModels_JSONCarriesTheServedModelAndUnreported(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "10",
		modelsRecord("scout", 1, "balanced", "gemini-3.8-flash-high", ""),
		modelsRecord("build", 1, "deep", "claude-opus-5-5", "claude-opus-5-5"),
	)

	var out, errb bytes.Buffer
	if code := runCycleModels([]string{"--project-root", root, "--cycle", "10", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("--json output is not valid JSON:\n%s", out.String())
	}
	for _, want := range []string{"claude-opus-5-5", "gemini-3.8-flash-high", "unreported"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--json output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunCycle_ModelsIsADispatchedSubcommand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "11", modelsRecord("audit", 1, "deep", "claude-opus-5-5", "claude-opus-5-5"))

	var out, errb bytes.Buffer
	if code := runCycle([]string{"models", "--project-root", root, "--cycle", "11"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if strings.Contains(errb.String(), "unknown subcommand") {
		t.Fatalf("models must be a known subcommand: %s", errb.String())
	}
	rowOf(t, out.String(), "audit", "1")

	errb.Reset()
	runCycle(nil, nil, &out, &errb)
	if !strings.Contains(errb.String(), "models") {
		t.Errorf("the missing-subcommand usage must list models: %q", errb.String())
	}
}

type closedModelsWriter struct{}

func (closedModelsWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestRunCycleModels_FlagAndOutputFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "12", modelsRecord("scout", 1, "balanced", "gemini-3.8-flash-high", ""))

	var out, errb bytes.Buffer
	if code := runCycleModels([]string{"--no-such-flag"}, &out, &errb); code != 10 {
		t.Errorf("bad flag exit=%d, want 10", code)
	}
	errb.Reset()
	if code := runCycleModels([]string{"--project-root", root, "--cycle", "12", "--json"}, closedModelsWriter{}, &errb); code != 1 {
		t.Errorf("failing JSON writer exit=%d, want 1", code)
	}
	if !strings.Contains(errb.String(), "encode") {
		t.Errorf("encode failure must be named on stderr: %q", errb.String())
	}
}

func TestRunCycleModels_DefaultsToTheLatestCycleWithCallRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeModelsFixture(t, root, "5", modelsRecord("scout", 1, "balanced", "old-selector", ""))
	writeModelsFixture(t, root, "21", modelsRecord("scout", 1, "balanced", "new-selector", ""))
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-99"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := runCycleModels([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cycle-21") || !strings.Contains(out.String(), "new-selector") || strings.Contains(out.String(), "old-selector") {
		t.Errorf("default must read the highest cycle that has records:\n%s", out.String())
	}

	empty := t.TempDir()
	out.Reset()
	errb.Reset()
	if code := runCycleModels([]string{"--project-root", empty}, &out, &errb); code != 1 || errb.Len() == 0 {
		t.Errorf("no ledger anywhere: exit=%d stderr=%q, want 1 with a reason", code, errb.String())
	}
}

func TestLoadCycleModelRows_UnreadableLedgerIsAnError(t *testing.T) {
	t.Parallel()
	runs := filepath.Join(t.TempDir(), "runs")
	if err := os.MkdirAll(filepath.Join(runs, "cycle-4", llmcalls.Filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadCycleModelRows(runs, "4"); err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("a ledger that cannot be read must error, got %v", err)
	}
}
