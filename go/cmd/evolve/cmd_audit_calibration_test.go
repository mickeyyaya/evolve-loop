package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCalibration_InvalidRootFailsLoudly(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	out := filepath.Join(t.TempDir(), "report.md")
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"audit", "calibration", "--dossiers-dir", missing, "--runs-dir", missing, "--output", out}, nil, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), missing) {
		t.Fatalf("dispatch invalid root = %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("invalid input wrote report: %v", err)
	}
}

func TestAuditCalibration_DeterministicMarkdown(t *testing.T) {
	root := t.TempDir()
	dossiers := filepath.Join(root, "knowledge-base", "cycles")
	runs := filepath.Join(root, ".evolve", "runs")
	commandWrite(t, filepath.Join(dossiers, "cycle-3.json"), `{"cycle":3,"goal":"test","final_verdict":"PASS","phases":[{"name":"audit","verdict":"PASS"}]}`)
	commandWrite(t, filepath.Join(runs, "cycle-3", "audit-chain-shadow.json"), `{"cycle":3,"phase":"audit","narrative_verdict":"PASS","chain_verdict":"PASS","shipped_verdict":"PASS"}`)

	run := func(name string) []byte {
		t.Helper()
		out := filepath.Join(root, name)
		var stdout, stderr bytes.Buffer
		code := dispatch([]string{"audit", "calibration", "--project-root", root, "--output", out}, nil, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("dispatch = %d: %s", code, stderr.String())
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first, second := run("first.md"), run("second.md")
	if !bytes.Equal(first, second) || !strings.Contains(string(first), "| 3 | PASS | PASS | PASS | PASS |") {
		t.Fatalf("reports differ or omit pair:\nfirst=%s\nsecond=%s", first, second)
	}
}

func commandWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(fmt.Sprint(content)), 0o644); err != nil {
		t.Fatal(err)
	}
}
