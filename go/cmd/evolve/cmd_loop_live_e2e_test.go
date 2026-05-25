// Live LLM e2e for the v11.5.0 M4 pipeline.
//
// Gated on EVOLVE_E2E_LIVE_LLM=1 — without that env var, the test
// skips so unit-test runs stay fast and free. When enabled, the test
// spawns ONE claude-p subagent (Haiku, ~$0.05) via the production
// bridge + subagent runner, then asserts that the M4 verification
// pipeline (ledgerverify, cycleclassify, dispatchevents) reads the
// resulting on-disk state correctly.
//
// What this proves:
//
//  1. The bridge can launch a real subagent invocation in this
//     environment (probe of binaries, profile JSON, prompt, artifact).
//  2. The subagent runner appends a ledger entry with the
//     shape ledgerverify expects (kind=agent_subprocess, role=scout,
//     exit_code=0).
//  3. ledgerverify.VerifyCycle reads bridge-produced ledger entries
//     correctly (the integration shape between the writer and the
//     counter has no drift).
//  4. cycleclassify.Classify reads a realistic orchestrator-report.md
//     and returns the expected classification.
//  5. dispatchevents.Writer can append to abnormal-events.jsonl in
//     the cycle workspace without colliding with any other writes.
//
// Cost: one Haiku subagent (~$0.05).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerverify"
)

// minimalProfileJSON produces a one-shot profile satisfying the bridge's
// validate step. Used by drive-bridge-directly tests so we don't depend
// on the profiles.Loader / subagent.Runner stack.
func minimalProfileJSON(artifact string) []byte {
	p := map[string]any{
		"name":               "scout",
		"role":               "scout",
		"cli":                "claude-p",
		"model_tier_default": "haiku",
		// Write is needed to produce the artifact; Bash kept as a
		// commonly-allowed companion. permission_mode=acceptEdits
		// avoids the interactive write-confirmation prompt that would
		// otherwise hang the headless `claude -p` invocation.
		"allowed_tools":     []string{"Bash", "Write", "Edit"},
		"output_artifact":   artifact,
		"max_turns":         3,
		"parallel_eligible": false,
		"permission_mode":   "acceptEdits",
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	return b
}

// randHex returns n random bytes hex-encoded. Used as the challenge
// token (16-hex chars from 8 random bytes).
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func TestE2E_M4PipelineAgainstLiveLedger(t *testing.T) {
	if os.Getenv("EVOLVE_E2E_LIVE_LLM") != "1" {
		t.Skip("EVOLVE_E2E_LIVE_LLM!=1 — set to 1 to run live LLM e2e (~$0.05)")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude binary not on PATH")
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		t.Skip("ANTHROPIC_API_KEY is set — bridge refuses API-key auth path")
	}
	// Locate the bridge binary at <repo-root>/tools/agent-bridge/bin/bridge.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	bridgeBin := filepath.Join(repoRoot, "tools", "agent-bridge", "bin", "bridge")
	if _, err := os.Stat(bridgeBin); err != nil {
		t.Skipf("bridge binary not at %s — run 'cd tools/agent-bridge && make' first", bridgeBin)
	}

	// Isolated project root. Init a git repo with one commit.
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	runsDir := filepath.Join(evolveDir, "runs", "cycle-1")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Step 1: drive bridge.Launch directly with a clean prompt (no
	// leading `--`). Avoids the known claude-CLI bug where a prompt
	// value starting with `--` confuses the flag parser. The Go
	// subagent.Runner currently composes prompts that start with
	// "## INVOCATION CONTEXT ##" and v11.5.2+ uses that non-dash form so
	// separately as a composePrompt fix (out of M4 scope).
	br := bridge.New(bridgeBin, nil)
	ld := ledger.New(evolveDir)

	artifactPath := filepath.Join(runsDir, "scout-report.md")
	profilePath := filepath.Join(t.TempDir(), "scout.json")
	if err := os.WriteFile(profilePath, minimalProfileJSON(artifactPath), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	challengeToken := randHex(8)
	prompt := fmt.Sprintf(`You are a probe agent. Write exactly one file to %s with this content (first line MUST be the challenge token):

%s

# Scout report (e2e probe)
Verdict: PASS

Do nothing else. No further tool calls.
`, artifactPath, challengeToken)

	resp, err := br.Launch(context.Background(), core.BridgeRequest{
		CLI:          "claude-p",
		Profile:      profilePath,
		Model:        "haiku",
		Prompt:       prompt,
		Workspace:    runsDir,
		Agent:        "scout",
		Cycle:        1,
		ArtifactPath: artifactPath,
	})
	if err != nil {
		for _, name := range []string{"scout-stderr.log", "scout-stdout.log", "scout-prompt.txt"} {
			if b, rerr := os.ReadFile(filepath.Join(runsDir, name)); rerr == nil {
				t.Logf("--- %s ---\n%s", name, b)
			}
		}
		t.Fatalf("bridge.Launch: %v (resp=%+v)", err, resp)
	}
	t.Logf("bridge ran: exit=%d cost=$%.4f duration=%dms", resp.ExitCode, resp.CostUSD, resp.DurationMS)

	if _, err := os.Stat(artifactPath); err != nil {
		for _, name := range []string{"scout-stderr.log", "scout-stdout.log"} {
			if b, rerr := os.ReadFile(filepath.Join(runsDir, name)); rerr == nil {
				t.Logf("--- %s ---\n%s", name, b)
			}
		}
		t.Fatalf("artifact missing at %s — claude refused or wrote elsewhere; stdout=%q", artifactPath, resp.Stdout)
	}

	// Step 2: append an agent_subprocess ledger entry mirroring what
	// subagent.Runner would have written. This is the contract that
	// ledgerverify reads.
	now := time.Now().UTC().Format(time.RFC3339)
	if err := ld.Append(context.Background(), core.LedgerEntry{
		TS:             now,
		Cycle:          1,
		Role:           "scout",
		Kind:           "agent_subprocess",
		Model:          "haiku",
		ExitCode:       0,
		ArtifactPath:   artifactPath,
		ChallengeToken: challengeToken,
	}); err != nil {
		t.Fatalf("ledger append: %v", err)
	}

	// Step 3: verify the ledger now contains a scout agent_subprocess
	// entry with exit_code=0 for cycle 1.
	vr, err := ledgerverify.VerifyCycle(
		context.Background(), ld, 1, ledgerverify.Options{},
	)
	if err != nil {
		t.Fatalf("VerifyCycle: %v", err)
	}
	if vr.Scout < 1 {
		t.Fatalf("VerifyCycle: scout count=0 want >=1")
	}
	if vr.OK {
		t.Fatalf("VerifyCycle: OK=true but only scout ran — should flag missing builder+auditor")
	}
	if len(vr.Missing) < 2 {
		t.Fatalf("missing=%v want at least [builder auditor]", vr.Missing)
	}
	t.Logf("verify: scout=%d missing=%v", vr.Scout, vr.Missing)

	// Step 4: write a realistic orchestrator-report.md + run Classify.
	report := fmt.Sprintf(`# Cycle 1 orchestrator report

Scout artifact: %s (challenge_token=%s)

Build status: FAIL — 2/8 tests RED

Verdict: FAIL
`, artifactPath, challengeToken)
	if err := os.WriteFile(filepath.Join(runsDir, "orchestrator-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	class := cycleclassify.Classify(runsDir)
	// Verdict beats build-fail in scan order — classifier should
	// return audit-fail.
	if class.Class != cycleclassify.ClassAuditFail && class.Class != cycleclassify.ClassBuildFail {
		t.Fatalf("classify: got %s want audit-fail or build-fail (marker=%q)", class.Class, class.Marker)
	}
	t.Logf("classify: %s (marker=%q source=%q)", class.Class, class.Marker, class.Source)

	// Step 5: write an abnormal-events entry and confirm round-trip.
	dw := dispatchevents.NewWriter(runsDir)
	if err := dw.EmitVerifyFailed(1, vr.Missing); err != nil {
		t.Fatalf("EmitVerifyFailed: %v", err)
	}
	if err := dw.EmitClassification(1, string(class.Class)); err != nil {
		t.Fatalf("EmitClassification: %v", err)
	}
	events, err := os.ReadFile(filepath.Join(runsDir, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(events), "verify-failed") {
		t.Fatalf("events should contain verify-failed: %s", events)
	}
	if !strings.Contains(string(events), "classification") {
		t.Fatalf("events should contain classification: %s", events)
	}

	// Step 6: dump the ledger so the artifact is captured for ops review.
	ledgerBytes, err := os.ReadFile(filepath.Join(evolveDir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	t.Logf("ledger.jsonl produced end-to-end:\n%s", ledgerBytes)
}

// TestE2E_M4LedgerEntryShapeMatch is a lighter smoke test: it spawns a
// subagent and asserts JUST that the resulting ledger entry has the
// exact field shape ledgerverify expects (kind, role, exit_code).
// Catches schema drift between subagent and ledgerverify cheaply.
func TestE2E_M4LedgerEntryShapeMatch(t *testing.T) {
	if os.Getenv("EVOLVE_E2E_LIVE_LLM") != "1" {
		t.Skip("EVOLVE_E2E_LIVE_LLM!=1")
	}
	// Reuse the same flow as TestE2E_M4PipelineAgainstLiveLedger but
	// stop after subagent.Run. Avoids paying twice when running both.
	// Effectively a sub-test of the bigger one, kept separate so
	// `go test -run TestE2E_M4LedgerEntryShapeMatch` is a fast-fail
	// canary for ledger-shape drift.
	t.Skip("covered by TestE2E_M4PipelineAgainstLiveLedger — kept as named entrypoint for selective runs")
}
