package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// driver_credentials_test.go — parity tests for the per-driver
// credential-isolation cost-leak guards (drivers/claude-p.sh +
// drivers/codex.sh) and the codex model-tier mapping (mock-cli-drivers.bats
// T-mock.5/6/7). Uses runLookup to inject a controlled env via the
// Deps.LookupEnv seam, so guard behavior is deterministic regardless of
// the host's ambient ANTHROPIC_API_KEY / OPENAI_API_KEY.

// mapLookup adapts a map to the Deps.LookupEnv signature.
func mapLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// runLookup drives LaunchArgs with the fake runner and a controlled env
// lookup (nil = empty, so guards stay inert).
func runLookup(t *testing.T, fr *fakeRunner, args []string, lookup map[string]string) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(lookup)})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), args, nil, &stdout, &stderr)
	return code, stderr.String()
}

// codexArgs builds a codex launch argv with an explicit model (the
// shared fixture hardcodes --model=auto; the mapping tests need a tier).
func codexArgs(fx launchFixture, model string) []string {
	return []string{
		"--cli=codex",
		"--profile=" + fx.profile,
		"--model=" + model,
		"--prompt-file=" + fx.promptFile,
		"--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog,
		"--stderr-log=" + fx.stderrLog,
		"--artifact=" + fx.artifact,
	}
}

// --- claude-p credential-isolation guards (drivers/claude-p.sh) -----------

func TestLaunchArgs_ClaudeP_AnthropicAPIKey_CostLeak(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{}
	code, stderr := runLookup(t, fr, fx.args("claude-p"), map[string]string{"ANTHROPIC_API_KEY": "sk-test"})
	if code != ExitCostLeak {
		t.Fatalf("exit = %d, want %d (ExitCostLeak)", code, ExitCostLeak)
	}
	if !strings.Contains(stderr, "ANTHROPIC_API_KEY") {
		t.Fatalf("stderr should name ANTHROPIC_API_KEY; got %q", stderr)
	}
	if len(fr.calls) != 0 {
		t.Fatalf("driver must NOT invoke the inner CLI when a credential leak is detected")
	}
}

func TestLaunchArgs_ClaudeP_AnthropicBaseURL_CostLeak(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{}
	code, stderr := runLookup(t, fr, fx.args("claude-p"), map[string]string{"ANTHROPIC_BASE_URL": "http://127.0.0.1:4000"})
	if code != ExitCostLeak {
		t.Fatalf("exit = %d, want ExitCostLeak", code)
	}
	if !strings.Contains(stderr, "ANTHROPIC_BASE_URL") {
		t.Fatalf("stderr should name ANTHROPIC_BASE_URL; got %q", stderr)
	}
}

func TestLaunchArgs_ClaudeP_AnthropicBaseURL_AllowedProceeds(t *testing.T) {
	// With BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1 the guard is waived.
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := runLookup(t, fr, fx.args("claude-p"), map[string]string{
		"ANTHROPIC_BASE_URL":              "http://127.0.0.1:4000",
		"BRIDGE_ALLOW_ANTHROPIC_BASE_URL": "1",
	})
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (guard waived)", code)
	}
	if len(fr.calls) == 0 {
		t.Fatalf("driver should have invoked the inner CLI")
	}
}

// --- codex credential-isolation guard (drivers/codex.sh) ------------------

func TestLaunchArgs_Codex_OpenAIKey_CostLeak(t *testing.T) {
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{}
	code, stderr := runLookup(t, fr, fx.args("codex"), map[string]string{"OPENAI_API_KEY": "sk-x"})
	if code != ExitCostLeak {
		t.Fatalf("exit = %d, want ExitCostLeak", code)
	}
	if !strings.Contains(stderr, "OPENAI_API_KEY") {
		t.Fatalf("stderr should name OPENAI_API_KEY; got %q", stderr)
	}
}

func TestLaunchArgs_Codex_OpenAIKey_AllowedProceeds(t *testing.T) {
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := runLookup(t, fr, fx.args("codex"), map[string]string{
		"OPENAI_API_KEY":              "sk-x",
		"BRIDGE_ALLOW_OPENAI_API_KEY": "1",
	})
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (guard waived)", code)
	}
}

// --- codex model-tier mapping (mock-cli-drivers.bats T-mock.5/6/7) --------

func TestLaunchArgs_Codex_ModelMap(t *testing.T) {
	cases := []struct {
		tier, codexModel string
	}{
		{"haiku", "gpt-5.4-mini"},
		{"sonnet", "gpt-5.4"},
		{"opus", "gpt-5.5"},
	}
	for _, tc := range cases {
		t.Run(tc.tier, func(t *testing.T) {
			fx := newFixture(t, "codex", "")
			fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
			code, _ := runLookup(t, fr, codexArgs(fx, tc.tier), nil)
			if code != ExitOK {
				t.Fatalf("exit = %d, want ExitOK", code)
			}
			if !fr.argvContainsPair("-m", tc.codexModel) {
				t.Fatalf("codex argv should map %s → -m %s; calls=%+v", tc.tier, tc.codexModel, fr.calls)
			}
		})
	}
}
