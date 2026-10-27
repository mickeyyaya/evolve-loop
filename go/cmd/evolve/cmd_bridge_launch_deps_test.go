package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestBridgeLaunchDeps_TakesTheSandboxModeAndTheCredentialOptInsFromThePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"sandbox":{"mode":"on"},"bridge":{"allow_anthropic_base_url":true,"allow_openai_api_key":true}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stderr strings.Builder
	deps := bridgeLaunchDeps(os.Getwd, &stderr)
	if deps.SandboxMode != "on" || !deps.AllowAnthropicBaseURL || !deps.AllowOpenAIAPIKey || deps.TokenResolver == nil {
		t.Errorf("deps = {SandboxMode:%q AllowAnthropicBaseURL:%v AllowOpenAIAPIKey:%v}, want on/true/true from policy.json", deps.SandboxMode, deps.AllowAnthropicBaseURL, deps.AllowOpenAIAPIKey)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing for a valid policy", stderr.String())
	}
}

func TestBridgeLaunchDeps_AnUnknownRootKeepsTheOptInsOffAndSaysSo(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stderr strings.Builder
	deps := bridgeLaunchDeps(func() (string, error) { return "", errors.New("gone") }, &stderr)
	if deps.SandboxMode != "" || deps.AllowAnthropicBaseURL || deps.AllowOpenAIAPIKey || deps.TokenResolver == nil {
		t.Errorf("deps = %+v, want the opt-ins off", deps)
	}
	if !strings.Contains(stderr.String(), "the policy opt-ins stay off") {
		t.Errorf("stderr = %q, want the WARN", stderr.String())
	}
}

func TestBridgeLaunchDeps_AHumanInputIsRecordedInTheProjectLedgerAndRefusedInACell(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	deps := bridgeLaunchDeps(os.Getwd, io.Discard)
	if err := deps.RecordBypass("human-input", "demo for the operator"); err != nil {
		t.Fatalf("RecordBypass: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil || !strings.Contains(string(raw), `"action":"human-input"`) || !strings.Contains(string(raw), "demo for the operator") {
		t.Fatalf("ledger = %q (%v), want one human-input record with the reason", raw, err)
	}
	t.Setenv(ipcenv.PhaseKey, "build")
	if err := deps.RecordBypass("human-input", "x"); err == nil || !strings.Contains(err.Error(), "inside an LLM cell") {
		t.Errorf("in a cell: err = %v, want the refusal", err)
	}
}

func TestRunBridgeLaunch_AHumanInputIsAuditedThroughTheLaunchRoot(t *testing.T) {
	root := t.TempDir()
	ws := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	profile := filepath.Join(ws, "profile.json")
	prompt := filepath.Join(ws, "prompt.txt")
	if err := os.WriteFile(profile, []byte(`{"name":"audit-probe","model":"haiku"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prompt, []byte("probe"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"launch", "--cli=claude-p", "--profile=" + profile, "--model=haiku", "--prompt-file=" + prompt,
		"--workspace=" + ws, "--stdout-log=" + filepath.Join(ws, "o.log"), "--stderr-log=" + filepath.Join(ws, "e.log"),
		"--artifact=" + filepath.Join(ws, "a.md"), "--human-input=operator demo", "--effort=bogus"}
	var stderr strings.Builder
	if code := runBridge(args, nil, io.Discard, &stderr); code == 0 {
		t.Fatalf("code = 0, want the bad --effort to stop the launch after the audit; stderr = %q", stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil || !strings.Contains(string(raw), `"action":"human-input"`) || !strings.Contains(string(raw), "operator demo") {
		t.Fatalf("ledger = %q (%v), want the human-input record written by the launch root; stderr = %q", raw, err, stderr.String())
	}
}
