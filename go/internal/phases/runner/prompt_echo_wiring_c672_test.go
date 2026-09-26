package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
)

// logWritingBridge writes the artifact and the raw phase logs, as a tmux driver does, so the producer classifies real input.
type logWritingBridge struct {
	artifact   string
	stderrLine string
}

func (b *logWritingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755); err != nil {
		return core.BridgeResponse{}, err
	}
	if err := os.WriteFile(req.ArtifactPath, []byte(b.artifact), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, req.Agent+"-stdout.log"), []byte("agent transcript line\n"), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, req.Agent+"-stderr.log"), []byte(b.stderrLine+"\n"), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	return core.BridgeResponse{Stdout: b.artifact}, nil
}

func (b *logWritingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func eventsContainInfraFailure(t *testing.T, ws, phase string) bool {
	t.Helper()
	f, err := os.Open(filepath.Join(ws, phase+"-events.ndjson"))
	if err != nil {
		t.Fatalf("open events file: %v", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<10), 1<<22)
	for sc.Scan() {
		var e struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal envelope %q: %v", sc.Text(), err)
		}
		if e.Kind == string(phasestream.KindInfraFailure) {
			return true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan events file: %v", err)
	}
	return false
}

func TestC672_002_RunnerThreadsComposedPromptToEventsProducer(t *testing.T) {
	const composedPrompt = "Adversarial Reviewer checklist: unbounded allocation or recursion; " +
		"TOCTOU / race windows; missing rate limits. Report exploits only."

	cases := []struct {
		name       string
		stderrLine string
		wantInfra  bool
	}{
		{"echoed composed-prompt line is suppressed", "missing rate limits.", false},
		{"genuine 429 frame still emits", "Error: 429 Too Many Requests (rate limit hit)", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hooks := &fakeHooks{
				phase:     "build",
				agent:     "evolve-builder",
				model:     "sonnet",
				prompt:    composedPrompt,
				verdict:   core.VerdictPASS,
				nextPhase: "audit",
			}
			ws := t.TempDir()
			r := New(Options{
				Hooks:   hooks,
				Bridge:  &logWritingBridge{artifact: "# build artifact\n## Files Modified\n- a.go\n", stderrLine: tc.stderrLine},
				Prompts: fakePromptsFS("evolve-builder", "agent body"),
				// EventsProducer is left unset on purpose: the test pins the real default producer.
			})

			if _, err := r.Run(context.Background(), core.PhaseRequest{
				Cycle: 672, ProjectRoot: t.TempDir(), Workspace: ws,
				RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			}); err != nil {
				t.Fatalf("Run: %v", err)
			}

			if got := eventsContainInfraFailure(t, ws, "build"); got != tc.wantInfra {
				t.Errorf("infra_failure in build-events.ndjson = %v, want %v (stderr line %q; composed prompt must reach the events producer)", got, tc.wantInfra, tc.stderrLine)
			}
		})
	}
}
