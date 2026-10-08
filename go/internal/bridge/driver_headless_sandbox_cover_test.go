package bridge

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeadlessDrivers_ARequiredSandboxThatIsUnavailableFailsClosed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		driver Driver
		cli    string
	}{
		{"claude-p", claudePDriver{}, "claude-p"},
		{"codex", codexDriver{}, "codex"},
	}
	for _, tc := range cases {
		runner := &fakeRunner{}
		var stderr bytes.Buffer
		deps := Deps{Runner: runner.runner(), Stderr: &stderr,
			Env: map[string]string{"ANTHROPIC_API_KEY": "", "ANTHROPIC_BASE_URL": "", "OPENAI_API_KEY": ""}}

		logs := t.TempDir()
		rc, err := tc.driver.Launch(context.Background(), &Config{
			CLI: tc.cli, Agent: "build", PromptFile: agyPromptFile(t), Model: "auto", RequireSandbox: true,
			StdoutLog: filepath.Join(logs, "out.log"), StderrLog: filepath.Join(logs, "err.log"),
		}, deps)

		if err != nil || rc != ExitSafetyGate || len(runner.calls) != 0 {
			t.Errorf("%s: rc=%d err=%v calls=%d, want ExitSafetyGate and no unconfined run", tc.name, rc, err, len(runner.calls))
		}
		if !strings.Contains(stderr.String(), "safety gate: activated Build explanation contract requires OS sandbox confinement") {
			t.Errorf("%s: stderr=%q, want the safety-gate line", tc.name, stderr.String())
		}
	}
}
