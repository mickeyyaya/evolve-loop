package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type failingStageWriter struct{}

func (failingStageWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestStageVerb_CoveragePaths(t *testing.T) {
	t.Parallel()

	root := stageProjectRoot(t, stageVerbPolicy)
	version := stageVersion(t, root)

	if _, _, code := runStageVerb(t, "verify", "--unknown-flag"); code != cliExitUsage {
		t.Errorf("verify unknown flag exited %d, want %d", code, cliExitUsage)
	}
	if _, _, code := runStageVerb(t, "report", "--unknown-flag"); code != cliExitUsage {
		t.Errorf("report unknown flag exited %d, want %d", code, cliExitUsage)
	}
	if _, _, code := runStageVerb(t, "verify", stageVerbKey, "extra"); code != cliExitUsage {
		t.Errorf("verify extra arg exited %d, want %d", code, cliExitUsage)
	}
	if _, _, code := runStageVerb(t, "report", "extra"); code != cliExitUsage {
		t.Errorf("report extra arg exited %d, want %d", code, cliExitUsage)
	}

	emptyRoot := t.TempDir()
	if _, _, code := runStageVerb(t, "verify", stageVerbKey, "--project-root", emptyRoot); code != cliExitIO {
		t.Errorf("verify missing policy exited %d, want %d", code, cliExitIO)
	}
	if _, _, code := runStageVerb(t, "report", "--project-root", emptyRoot); code != cliExitIO {
		t.Errorf("report missing policy exited %d, want %d", code, cliExitIO)
	}

	clean := stageChainFile(t, version, map[string]string{"would_stop": "false"})
	var errOut bytes.Buffer
	if code := runStage([]string{"verify", "--project-root", root, "--chain", clean, "--json", stageVerbKey}, nil, failingStageWriter{}, &errOut); code != cliExitIO {
		t.Errorf("verify json write failure exited %d, want %d; stderr: %s", code, cliExitIO, errOut.String())
	}

	textOut, _, code := runStageVerb(t, "verify", "--project-root", root, "--chain", clean, stageVerbKey)
	if code != cliExitOK || !strings.Contains(textOut, stageVerbKey+": ready\n") {
		t.Errorf("ready text verify = (code %d, %q), want ready", code, textOut)
	}

	stale := stageChainFile(t, "old-ver-1", map[string]string{"would_stop": "false"})
	staleText, _, staleCode := runStageVerb(t, "verify", stageVerbKey, "--project-root", root, "--chain", stale)
	if staleCode != cliExitFailed || !strings.Contains(staleText, "other version: old-ver-1 (1 records)") {
		t.Errorf("stale text verify = (code %d, %q), want other version line", staleCode, staleText)
	}

	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	policyBytes, err := os.ReadFile(filepath.Join(repo, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	entry := gateregistry.StageEntries()[0]
	realVer, err := stageevidence.DecisionVersion(filepath.Join(repo, "go"), entry.Shadow.Closure, policyBytes, entry.Shadow.PolicySubtree)
	if err != nil {
		t.Fatal(err)
	}
	rec := stageevidence.Link("", stageevidence.Record{
		Key:     entry.Shadow.PolicyKey,
		Version: realVer,
		Kind:    stageevidence.KindDecision,
		ID:      1,
		Seq:     1,
		Fields:  map[string]string{"would_stop": "false"},
	})
	rawChain, err := json.Marshal(stageevidence.Chain{
		Records:  []stageevidence.Record{rec},
		Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	readyChainPath := filepath.Join(t.TempDir(), "ready-report-chain.json")
	if err := os.WriteFile(readyChainPath, rawChain, 0o644); err != nil {
		t.Fatal(err)
	}
	reportText, reportErr, reportCode := runStageVerb(t, "report", "--project-root", repo, "--chain", readyChainPath)
	if reportCode != cliExitOK || !strings.Contains(reportText, entry.Shadow.PolicyKey+"\tready\t") {
		t.Errorf("report ready text = (code %d, %q, stderr %q), want ready row for %s", reportCode, reportText, reportErr, entry.Shadow.PolicyKey)
	}
}

func TestStageVerb_LoadPolicyGetwdError(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd failed") }

	if _, errOut, code := runStageVerb(t, "report"); code != cliExitIO || !strings.Contains(errOut, "getwd failed") {
		t.Errorf("report getwd error = (code %d, %q), want cliExitIO with getwd failed", code, errOut)
	}
}
