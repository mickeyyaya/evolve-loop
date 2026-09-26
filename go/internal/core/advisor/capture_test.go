package advisor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestLaunch_CapturesEvenWhenTheResponseIsUnparseable(t *testing.T) {
	in := tempInput(t)
	if _, err := New(&fakeLauncher{stdout: "no json here"}, defaultIdentity(), plainWriter).Plan(in); err == nil {
		t.Fatal("an unparseable plan must error")
	}
	if got := readArtifact(t, filepath.Join(in.Workspace, "advisor-response-plan.txt")); got != "no json here" {
		t.Errorf("the raw response is captured before the parse: %q", got)
	}
	readArtifact(t, filepath.Join(in.Workspace, "advisor-prompt-plan.txt"))
	readArtifact(t, filepath.Join(in.Workspace, "advisor-span-plan.json"))
}

func TestCapture_RedactsPersistsAndWarnsPerFailedArtifact(t *testing.T) {
	const secret = "sk-livesecret0123456789ABCDEF"
	in := tempInput(t)
	in.GoalText = "rotate the leaked key " + secret + " across the fleet"
	fl := &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, defaultIdentity(), plainWriter).Plan(in); err != nil {
		t.Fatal(err)
	}
	gotPrompt := readArtifact(t, filepath.Join(in.Workspace, "advisor-prompt-plan.txt"))
	if strings.Contains(gotPrompt, secret) || !strings.Contains(gotPrompt, "[REDACTED]") || !strings.Contains(fl.gotReq.Prompt, secret) {
		t.Errorf("only the PERSISTED copy is redacted; the live prompt keeps the real text:\n%s", gotPrompt)
	}
	if got := readArtifact(t, filepath.Join(in.Workspace, "advisor-response-plan.txt")); got != planJSON() {
		t.Errorf("the response artifact is the raw stdout: %q", got)
	}
	plain := tempInput(t)
	fl = &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, defaultIdentity(), plainWriter).Plan(plain); err != nil {
		t.Fatal(err)
	}
	if got := readArtifact(t, filepath.Join(plain.Workspace, "advisor-prompt-plan.txt")); got != fl.gotReq.Prompt {
		t.Errorf("without a secret the persisted prompt IS the prompt sent")
	}

	t.Run("the prompt write fails, the rest still land", func(t *testing.T) {
		in := tempInput(t)
		writer := func(path string, data []byte) error {
			if strings.Contains(path, "advisor-prompt-") {
				return errors.New("disk full")
			}
			return plainWriter(path, data)
		}
		a, got := observed(t, &fakeLauncher{stdout: planJSON()}, defaultIdentity())
		a.writeArtifact = writer
		plan, err := a.Plan(in)
		if err != nil || plan == nil || len(plan.Entries) != 1 {
			t.Fatalf("a forensic-capture failure must NOT fail the advisor: %v %+v", err, plan)
		}
		e := assertOneEvent(t, *got, CodeCaptureWriteFailed, map[string]string{"step": "capture", "artifact": "prompt", "op": "write", "path": filepath.Join(in.Workspace, "advisor-prompt-plan.txt"), "decision": "plan"})
		if e.Reason != "disk full" {
			t.Errorf("reason: %+v", e)
		}
		readArtifact(t, filepath.Join(in.Workspace, "advisor-response-plan.txt"))
		readArtifact(t, filepath.Join(in.Workspace, "advisor-span-plan.json"))
	})
	t.Run("every write fails: three events in artifact order", func(t *testing.T) {
		a, got := observed(t, &fakeLauncher{stdout: planJSON()}, defaultIdentity())
		a.writeArtifact = func(string, []byte) error { return errors.New("disk full") }
		if _, err := a.Plan(tempInput(t)); err != nil {
			t.Fatal(err)
		}
		if len(*got) != 3 {
			t.Fatalf("three events: %+v", *got)
		}
		for i, want := range []string{"prompt", "response", "span"} {
			if e := (*got)[i]; e.Code != CodeCaptureWriteFailed || e.Fields["artifact"] != want || e.Fields["op"] != "write" {
				t.Errorf("event %d: %+v, want artifact=%s", i, e, want)
			}
		}
	})
	t.Run("a nil writer is the Null Object", func(t *testing.T) {
		in := tempInput(t)
		a, got := observed(t, &fakeLauncher{stdout: planJSON()}, defaultIdentity())
		a.writeArtifact = nil
		if _, err := a.Plan(in); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(in.Workspace)
		if len(entries) != 0 || len(*got) != 0 {
			t.Errorf("no forensics, no events: %v %+v", entries, *got)
		}
	})
}

func TestCapture_SpanOpChainRoutesMarshalAndWriteThroughOneWarn(t *testing.T) {
	in := tempInput(t)
	a, got := observed(t, &fakeLauncher{stdout: planJSON()}, defaultIdentity())
	a.writeArtifact = func(path string, data []byte) error {
		if strings.Contains(path, "advisor-span-") {
			return errors.New("span write refused")
		}
		return plainWriter(path, data)
	}
	if _, err := a.Plan(in); err != nil {
		t.Fatal(err)
	}
	assertOneEvent(t, *got, CodeCaptureWriteFailed, map[string]string{"artifact": "span", "op": "write", "path": filepath.Join(in.Workspace, "advisor-span-plan.json")})
	readArtifact(t, filepath.Join(in.Workspace, "advisor-prompt-plan.txt"))
}

func TestPlan_RecordsDecisionSpanMetadata(t *testing.T) {
	in := tempInput(t)
	want := cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 80, CacheWrite: 16}
	fl := &fakeLauncher{stdout: planJSON(), durationMS: 1234, tokens: want}
	if _, err := New(fl, defaultIdentity(), plainWriter).Plan(in); err != nil {
		t.Fatal(err)
	}
	raw := readArtifact(t, filepath.Join(in.Workspace, "advisor-span-plan.json"))
	var span map[string]any
	if err := json.Unmarshal([]byte(raw), &span); err != nil {
		t.Fatalf("span json: %v\n%s", err, raw)
	}
	for _, k := range []string{"gen_ai.request.model", "gen_ai.system", "prompt_sha", "response_sha", "duration_ms", "replan_depth", "tokens"} {
		if _, ok := span[k]; !ok {
			t.Errorf("decision span missing key %q:\n%s", k, raw)
		}
	}
	if span["gen_ai.request.model"] != "opus" || span["gen_ai.system"] != "claude" || span["duration_ms"] != float64(1234) {
		t.Errorf("model/system/duration: %s", raw)
	}
	if span["prompt_sha"] != sha256OfString(readArtifact(t, filepath.Join(in.Workspace, "advisor-prompt-plan.txt"))) ||
		span["response_sha"] != sha256OfString(readArtifact(t, filepath.Join(in.Workspace, "advisor-response-plan.txt"))) {
		t.Error("the span SHAs are the sha256 of the persisted redacted files")
	}
	var typed Span
	if err := json.Unmarshal([]byte(raw), &typed); err != nil || typed.Tokens != want {
		t.Errorf("tokens threaded into the span: %+v (%v)", typed.Tokens, err)
	}
	if buf, _ := json.Marshal(Span{Model: "m", System: "s", PromptSHA: "p", ResponseSHA: "r", DurationMS: 5, ReplanDepth: 0, Tokens: cyclestate.TokenUsage{}}); !strings.Contains(string(buf), `"replan_depth":0`) || !strings.Contains(string(buf), `"tokens":{`) {
		t.Errorf("replan_depth and tokens are always emitted: %s", buf)
	}
}

func TestCapture_KindPerDecision(t *testing.T) {
	in := tempInput(t)
	if _, err := New(&fakeLauncher{stdout: `{"next_phase":"audit","justification":"build green"}`}, defaultIdentity(), plainWriter).Propose(in); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"advisor-prompt-proposal.txt", "advisor-response-proposal.txt", "advisor-span-proposal.json"} {
		if _, err := os.Stat(filepath.Join(in.Workspace, f)); err != nil {
			t.Errorf("proposal capture must use kind=proposal: %v", err)
		}
	}
}
