package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func agyPromptFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(path, []byte("implement"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgyDriver_RequiredSandboxUnavailableFailsClosed(t *testing.T) {
	runner := &fakeRunner{}
	var stderr bytes.Buffer
	rc, err := (agyDriver{}).Launch(context.Background(), &Config{
		Agent: "build", PromptFile: agyPromptFile(t), Model: "auto", RequireSandbox: true,
	}, Deps{Runner: runner.runner(), Stderr: &stderr})
	if err != nil || rc != ExitSafetyGate {
		t.Fatalf("agy required-sandbox launch rc=%d err=%v, want ExitSafetyGate/nil", rc, err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("agy ran unconfined despite required sandbox: calls=%v", runner.calls)
	}
}

func TestAgyDriver_RequiredSandboxWrapsInvocation(t *testing.T) {
	runner := &fakeRunner{}
	wrap := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "policy"}, available: true}
	var stderr bytes.Buffer
	logs := t.TempDir()
	rc, err := (agyDriver{}).Launch(context.Background(), &Config{
		Agent: "build", PromptFile: agyPromptFile(t), Model: "auto", RequireSandbox: true,
		StdoutLog: filepath.Join(logs, "stdout.log"), StderrLog: filepath.Join(logs, "stderr.log"),
	}, Deps{Runner: runner.runner(), SandboxWrap: wrap.wrap(), Stderr: &stderr})
	if err != nil || rc != 0 {
		t.Fatalf("agy sandboxed launch rc=%d err=%v", rc, err)
	}
	if len(runner.calls) != 1 || runner.calls[0].name != "sandbox-exec" {
		t.Fatalf("agy did not launch through sandbox prefix: calls=%v", runner.calls)
	}
}
