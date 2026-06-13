package subagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// worker_token_test.go — H1 self-correction: fan-out per-worker artifact
// verification must check PROVENANCE, not just presence. The fan-out parent
// dictates each worker's challenge token (parentToken+"-"+subtask), threads it
// to the worker (which writes it into its artifact), and verifies it on the
// parent side. Before this fix the per-worker Verify ran with an empty token
// (bytes.Contains(body, []byte("")) == true), so ANY non-empty file passed.

// TestDefaultVerifyWorkerArtifact_TokenChecked pins that the per-worker verifier
// rejects an artifact lacking the expected token and accepts one bearing it —
// the direct fix for the H1 vacuous-token-check.
func TestDefaultVerifyWorkerArtifact_TokenChecked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("forged content, no token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := defaultVerifyWorkerArtifact(time.Now, bad, "tok-worker-1"); got.Verdict == VerdictPASS {
		t.Errorf("tokenless artifact must NOT pass per-worker verification; got %s", got.Verdict)
	}

	good := filepath.Join(dir, "good.md")
	if err := os.WriteFile(good, []byte("<!-- challenge-token: tok-worker-1 -->\nreal output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := defaultVerifyWorkerArtifact(time.Now, good, "tok-worker-1"); got.Verdict != VerdictPASS {
		t.Errorf("token-bearing fresh artifact must pass; got %s", got.Verdict)
	}

	// Missing artifact → not PASS (presence still enforced).
	if got := defaultVerifyWorkerArtifact(time.Now, filepath.Join(dir, "nope.md"), "tok-worker-1"); got.Verdict == VerdictPASS {
		t.Errorf("missing artifact must NOT pass; got %s", got.Verdict)
	}
}

// TestBuildWorkerRecursionCommand_ThreadsWorkerToken pins that the worker
// recursion command threads EVOLVE_FANOUT_WORKER_TOKEN so the worker writes the
// parent-known token (the provenance the parent later verifies).
func TestBuildWorkerRecursionCommand_ThreadsWorkerToken(t *testing.T) {
	t.Parallel()
	cmd := buildWorkerRecursionCommand("/bin/evolve", "auditor", "sub1", 7, 1, "/ws", "/p.md", "ptok-worker-sub1")
	if !strings.Contains(cmd, "EVOLVE_FANOUT_WORKER_TOKEN=") {
		t.Errorf("worker command must thread EVOLVE_FANOUT_WORKER_TOKEN: %s", cmd)
	}
	if !strings.Contains(cmd, "ptok-worker-sub1") {
		t.Errorf("worker command must carry the per-worker token value: %s", cmd)
	}
	// The existing recursion contract is unchanged.
	if !strings.Contains(cmd, "CLAUDECODE_TYPE= ") || !strings.Contains(cmd, "subagent run auditor-worker-sub1 ") {
		t.Errorf("worker command must preserve B2 recursion contract: %s", cmd)
	}
}

// TestRun_ChallengeTokenOverride pins that a fan-out worker uses the
// parent-dictated token (so its artifact bears the token the parent verifies)
// instead of minting a fresh one.
func TestRun_ChallengeTokenOverride(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), RunRequest{
		Agent:                  "scout",
		Cycle:                  5,
		WorkspacePath:          ws,
		ProjectRoot:            tmp,
		PluginRoot:             tmp,
		PromptReader:           strings.NewReader("hi"),
		CachePrefixV2:          true,
		ChallengeTokenOverride: "parent-tok-worker-x",
	}, runHappyOpts(t))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.ChallengeToken != "parent-tok-worker-x" {
		t.Errorf("ChallengeToken = %q, want the override 'parent-tok-worker-x'", res.ChallengeToken)
	}
}
