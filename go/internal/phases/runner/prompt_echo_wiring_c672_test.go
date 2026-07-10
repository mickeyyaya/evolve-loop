package runner

// RED wiring test for cycle-672 top_n task `echo-veto-wiring-completion`,
// caller side of AC1: BaseRunner composes the phase prompt (runner.go ~307)
// and invokes the events producer per dispatch attempt (~551), but the default
// producer builds phasestream.ProduceConfig WITHOUT the prompt — so the live
// classifier has no echo context and an agent quoting its own prompt text is
// classified infra_failure (cycle-656 retro D3, third recurrence of the
// cycle-641 lesson).
//
// This test drives the REAL default events producer (no EventsProducer
// override) through Run() with a bridge fake that writes the phase logs, then
// asserts on the emitted <phase>-events.ndjson. RED today behaviorally: the
// echoed-line case emits infra_failure because the composed prompt never
// reaches phasestream.Produce. DO NOT MODIFY — Builder threads the composed
// prompt from Run into the producer (and ProduceConfig.InjectedPrompt) to make
// it GREEN; the genuine-frame case must STAY green (anti-over-suppression).

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

// logWritingBridge mimics a tmux driver: it writes the artifact AND the raw
// phase logs (<agent>-stdout.log / <agent>-stderr.log) into the workspace, so
// the runner's post-attempt events producer has something real to classify.
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

// eventsContainInfraFailure reports whether <ws>/<phase>-events.ndjson holds
// at least one infra_failure envelope.
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

// TestC672_002_RunnerThreadsComposedPromptToEventsProducer — AC1 (caller
// wiring): a stderr line that verbatim-echoes the COMPOSED phase prompt must
// not surface as infra_failure in the runner-produced events stream, while a
// genuine provider error frame (absent from the prompt) still must.
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
				// EventsProducer deliberately NOT overridden: this test pins the
				// real default producer chain runner → phasestream.Produce.
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
