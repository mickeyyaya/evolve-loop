//go:build integration && darwin

package bridge

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

func TestCI_TheRunnerCanApplyTheAgentSandbox(t *testing.T) {
	probe := requireNativeSandbox(t)
	var stderr bytes.Buffer
	deps := Deps{Env: map[string]string{}, LookupEnv: mapLookup(nil), Stderr: &stderr}
	wrap := defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	prefix, ok := wrap(SandboxWrapRequest{Phase: "builder", RepoRoot: t.TempDir(), Worktree: t.TempDir(), Workspace: t.TempDir(), AllowNetwork: true})
	if !ok {
		t.Fatalf("the bridge refused to build the agent profile on this runner: binary=%s stderr=%q", probe.BinaryPath, stderr.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, prefix[0], append(prefix[1:], "/usr/bin/true")...).CombinedOutput()
	if err != nil {
		sbpl, readErr := os.ReadFile(prefix[2])
		t.Fatalf("the runner applies a sandbox but not the generated agent profile: binary=%s apply error=%v output=%q read=%v\nSBPL:\n%s",
			probe.BinaryPath, err, out, readErr, sbpl)
	}
}
