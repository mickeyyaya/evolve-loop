//go:build integration

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func e3IsolatedCLIHost(t *testing.T, ollamaBody string) (root string) {
	t.Helper()
	bin := t.TempDir()
	if ollamaBody != "" {
		fakeclitest.Install(t, filepath.Join(bin, "ollama"), "#!/bin/sh\n"+ollamaBody+"\n")
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("EVOLVE_PLUGIN_ROOT", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	t.Setenv("OLLAMA_API_KEY", "")
	return t.TempDir()
}

const e3FakeOllama = `case "$1" in
--version) echo "ollama version 0.0.1" ;;
list) echo "server down" >&2; exit 3 ;;
esac`

func TestRunModelsRefresh_DetectSourceCommitsOnlyTheReadyCLIsManifestMap(t *testing.T) {
	root := e3IsolatedCLIHost(t, e3FakeOllama)
	evolveDir := filepath.Join(root, ".evolve")

	code, out, errb := runModelsCLI("refresh", "--source", "detect", "--project-root", root, "--evolve-dir", evolveDir)

	if code != 0 {
		t.Fatalf("code = %d, stderr %q", code, errb)
	}
	if !strings.HasPrefix(out, "Refreshed model catalog (source: setup detect) → 1 CLI(s):\n  ollama ") {
		t.Errorf("stdout = %q, want the detect report naming only ollama", out)
	}
	cat, err := modelcatalog.Read(evolveDir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(cat.CLIs) != 1 || cat.CLIs["ollama"].Source != modelcatalog.SourceDetect || len(cat.CLIs["ollama"].TierModels) == 0 {
		t.Errorf("committed catalog = %+v, want exactly ollama from the detect map", cat.CLIs)
	}
}

func TestRunModelsRefresh_LiveWithNoReadyCLIFailsAndWritesNothing(t *testing.T) {
	root := e3IsolatedCLIHost(t, "")
	evolveDir := filepath.Join(root, ".evolve")

	code, out, errb := runModelsCLI("refresh", "--project-root", root, "--evolve-dir", evolveDir)

	if code != 1 {
		t.Fatalf("code = %d, want 1 when no CLI is ready", code)
	}
	if !strings.Contains(errb, "evolve models refresh: no ready CLIs to query (run: evolve setup detect)") {
		t.Errorf("stderr = %q", errb)
	}
	if out != "" {
		t.Errorf("stdout = %q, want no report", out)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "model-catalog.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("catalog written although no CLI was ready: %v", err)
	}
}

func TestRunModelsRefresh_LiveListFailureFallsBackAndKeepsThePriorCatalog(t *testing.T) {
	root := e3IsolatedCLIHost(t, e3FakeOllama)
	evolveDir, priorFetched := seedPriorCatalog(t)

	code, out, errb := runModelsCLI("refresh", "--source", "live", "--project-root", root, "--evolve-dir", evolveDir)

	if code != 1 {
		t.Fatalf("code = %d, want 1 when no CLI classified live; stderr %q", code, errb)
	}
	if !strings.HasPrefix(out, "Model catalog NOT refreshed (source: live /model) → 0 of 1 CLI(s) classified live:\n  ollama ") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(out, "detect fallback, reason=") || !strings.Contains(out, "ollama list") {
		t.Errorf("the report does not name the ollama list failure: %q", out)
	}
	onDisk, err := modelcatalog.Read(evolveDir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !onDisk.FetchedAt.Equal(priorFetched) || onDisk.CLIs["agy"].TierModels["balanced"] != "Gemini 3.7 Flash (High)" {
		t.Errorf("the prior catalog changed: %+v", onDisk)
	}
}
