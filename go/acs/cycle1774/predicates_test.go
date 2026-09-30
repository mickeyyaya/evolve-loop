//go:build acs

package cycle1774

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

const canarySocket = "evolve-bridge-p999999999"

var sweepReachers = []string{
	"TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd",
	"TestRunLoopBatch_SignalExitSkipsBatchEndGCHook",
	"TestC1735_002_PreBatchGCHookTargetsBatchOwnedDir",
	"TestC1735_003_NextCycleWorkspaceHasNoGCManifestsAfterPreBatchGC",
	"TestC1735_006_PreBatchGCManifestSurvivesBatchEndGC",
	"TestLoop_CycleLevelFailureContinues",
	"TestRunLoop_AutoPrunesExpiredCarryoverTodos",
	"TestRunLoop_BootRefreshRunsBeforeRecovery",
	"TestRunLoop_FailVerdictBreaks",
	"TestRunLoop_HaltsPreScoutOnWithinVersionSelfShaMismatch",
	"TestRunLoop_InvokesBootRecoveryBeforeGate",
	"TestRunLoop_OrchestratorError",
	"TestRunLoop_PreflightHalt_AbortsBeforeCycle",
	"TestRunLoop_ResumeFailVerdict",
	"TestRunLoop_ResumeFullProtocol",
	"TestRunLoop_ResumeMissingCheckpoint",
	"TestRunLoop_ResumePhaseRunnerError",
	"TestRunLoop_VerifyIterError",
}

type testEvent struct {
	Action  string
	Package string
	Test    string
	Output  string
}

func childEnv(tmuxTmp string) []string {
	var env []string
	for _, kv := range ipcenv.Scrub(os.Environ()) {
		key, _, _ := strings.Cut(kv, "=")
		if key == "TMUX_TMPDIR" || key == "TMUX" || key == "TMUX_PANE" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TMUX_TMPDIR="+tmuxTmp)
}

func runCmdEvolve(t *testing.T, tmuxTmp, runPattern string) []testEvent {
	t.Helper()
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline.Add(-10*time.Second))
		defer cancel()
	}
	args := []string{"test", "-json", "-count=1"}
	if runPattern != "" {
		args = append(args, "-run", runPattern)
	}
	cmd := exec.CommandContext(ctx, "go", append(args, "./cmd/evolve")...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.Env = childEnv(tmuxTmp)
	cmd.WaitDelay = 5 * time.Second
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("go test ./cmd/evolve could not run: %v\nstderr:\n%s", err, stderr.String())
	}
	var events []testEvent
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var ev testEvent
		if derr := dec.Decode(&ev); derr == io.EOF {
			break
		} else if derr != nil {
			t.Fatalf("decode go test -json stream: %v\nstderr:\n%s", derr, stderr.String())
		}
		events = append(events, ev)
	}
	return events
}

func suiteCompleted(events []testEvent) error {
	started, terminal := false, ""
	for _, ev := range events {
		if ev.Package != cmdEvolvePkg {
			continue
		}
		if strings.Contains(ev.Output, "panic: ") || strings.Contains(ev.Output, "test timed out") {
			return fmt.Errorf("the suite aborted mid-run: %s", strings.TrimSpace(ev.Output))
		}
		if ev.Test != "" {
			continue
		}
		switch ev.Action {
		case "start":
			started = true
		case "pass", "fail":
			terminal = ev.Action
		case "skip":
			return fmt.Errorf("the package was skipped")
		}
	}
	if !started || terminal == "" {
		return fmt.Errorf("the package never started or never finished (build failure?)")
	}
	return nil
}

func notPassed(events []testEvent, names []string) []string {
	final := map[string]string{}
	output := map[string]*strings.Builder{}
	for _, ev := range events {
		if ev.Package != cmdEvolvePkg || ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			final[ev.Test] = ev.Action
		case "output":
			if output[ev.Test] == nil {
				output[ev.Test] = &strings.Builder{}
			}
			output[ev.Test].WriteString(ev.Output)
		}
	}
	var bad []string
	for _, name := range names {
		if final[name] == "pass" {
			continue
		}
		tail := ""
		if b := output[name]; b != nil {
			tail = b.String()
			if len(tail) > 1500 {
				tail = "…" + tail[len(tail)-1500:]
			}
		}
		bad = append(bad, fmt.Sprintf("%s=%q\n%s", name, final[name], tail))
	}
	return bad
}

func TestC1774_001_HostTmuxCanarySurvivesCmdEvolveSuite(t *testing.T) {
	tmuxTmp := t.TempDir()
	sockDir := filepath.Join(tmuxTmp, fmt.Sprintf("tmux-%d", os.Getuid()))
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(sockDir, canarySocket)
	if err := os.WriteFile(canary, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	events := runCmdEvolve(t, tmuxTmp, "")

	if err := suiteCompleted(events); err != nil {
		t.Fatalf("go test ./cmd/evolve did not run to completion, so canary survival proves nothing: %v", err)
	}
	if bad := notPassed(events, sweepReachers); len(bad) > 0 {
		t.Errorf("sweep-reaching tests did not execute and pass (a skipped or failing test is not an isolated one): %s", strings.Join(bad, ", "))
	}
	if _, err := os.Stat(canary); err != nil {
		t.Errorf("RED: the dead-pid canary %s in the host tmux socket dir (TMUX_TMPDIR as inherited by `go test ./cmd/evolve`) was removed by the suite (stat err=%v): a test still runs the loop's orphan-socket sweep against the host dir instead of an isolated one", canary, err)
	}
}

func TestC1774_002_LoopOrphanSocketGCStillReapsInTheEnvNamedDir(t *testing.T) {
	const guard = "TestRunLoopBatch_OrphanSocketGCReapsOnlyDeadSocketsInTheEnvNamedDir"
	events := runCmdEvolve(t, t.TempDir(), "^"+guard+"$")

	if err := suiteCompleted(events); err != nil {
		t.Fatalf("go test -run %s ./cmd/evolve did not run to completion: %v", guard, err)
	}
	if bad := notPassed(events, []string{guard}); len(bad) > 0 {
		t.Errorf("the loop's orphan-socket sweep no longer reaps a dead-pid socket (and spares a live one) in the TMUX_TMPDIR socket dir: %s — isolate the sweep, never disable it", strings.Join(bad, ", "))
	}
}
