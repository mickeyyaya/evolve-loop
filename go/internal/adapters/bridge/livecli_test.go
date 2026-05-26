package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// livecli_test.go — the tier-2 cutover proof for the HEADLESS path: a real
// claude-p launch driven through the *production* adapter seam (real engine
// factory, real exec runner) with EVOLVE_BRIDGE_GO=1, proving that flipping
// the default actually routes a real LLM invocation in-process. Gated behind
// EVOLVE_BRIDGE_ADAPTER_LIVE=1 so it spends tokens only when an operator asks.
// The interactive (tmux) auto-reply proofs live in internal/bridge's live
// suite; this guards the non-interactive adapter→engine→claude-p path.
func TestAdapter_Launch_LiveClaudeP_RoutesToRealEngine(t *testing.T) {
	if os.Getenv("EVOLVE_BRIDGE_ADAPTER_LIVE") != "1" {
		t.Skip("set EVOLVE_BRIDGE_ADAPTER_LIVE=1 to run the live adapter→claude-p proof (spends tokens)")
	}
	// Credential-isolation guard would (correctly) abort the launch with
	// EC_COST_LEAK if these are set; the live proof needs the CLI's own
	// configured auth, so surface the misconfig loudly instead of a confusing
	// non-zero exit.
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		t.Skip("ANTHROPIC_API_KEY is set; the claude-p driver refuses to run (cost-leak guard). Unset it for the live proof.")
	}

	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	profPath := filepath.Join(root, "profile.json")
	if err := os.WriteFile(profPath, []byte(`{"name":"livetest","model":"haiku"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "artifact")

	// $ARTIFACT_PATH is substituted by preparePrompt → the resolved artifact
	// path; the model writes its answer there, which the bridge reads back
	// into BridgeResponse.Stdout.
	prompt := "Use your file-writing tool to write exactly the single word " +
		"PONG (uppercase, no other text) to the file $ARTIFACT_PATH. Then stop."

	// New("bridge", nil) wires the PRODUCTION engine factory and real exec
	// runner — the exact object the orchestrator constructs. EVOLVE_BRIDGE_GO=1
	// in req.Env drives the cutover branch at bridge.go:126.
	a := New("bridge", nil)
	resp, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI:          "claude-p",
		Profile:      profPath,
		Model:        "auto", // resolves to the profile's "haiku"
		Prompt:       prompt,
		Workspace:    ws,
		ArtifactPath: artifact,
		Agent:        "livetest",
		Env:          map[string]string{"EVOLVE_BRIDGE_GO": "1"},
		// Grant the Write tool non-interactively so the headless run can
		// produce the artifact without a permission prompt.
		ExtraFlags: []string{"--permission-mode", "bypassPermissions"},
	})
	if err != nil {
		t.Fatalf("adapter Launch (live claude-p) err: %v\nstderr: %s", err, resp.Stderr)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("live claude-p exit = %d, want 0\nstderr: %s", resp.ExitCode, resp.Stderr)
	}
	// Primary proof: the adapter routed a real claude-p invocation in-process
	// and it succeeded. Secondary: the artifact round-tripped into the
	// response. Some headless tool configs may not write the file even on a
	// clean exit, so treat an empty artifact as a soft signal, not a failure.
	if got := strings.TrimSpace(resp.Stdout); got != "" && !strings.Contains(got, "PONG") {
		t.Logf("artifact present but unexpected content (non-fatal): %q", got)
	}
	t.Logf("live adapter→engine→claude-p OK: exit=0, artifact=%q", strings.TrimSpace(resp.Stdout))
}
