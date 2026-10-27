package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func readTryRecords(t *testing.T, ws string) []llmcalls.Record {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, llmcalls.Filename))
	if err != nil {
		t.Fatalf("read %s: %v", llmcalls.Filename, err)
	}
	var recs []llmcalls.Record
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		var rec llmcalls.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("record %q: %v", line, err)
		}
		recs = append(recs, rec)
	}
	return recs
}

func launchClaudeTmuxTry(t *testing.T, resolver func(tokenusage.Window) (tokenusage.Result, error)) llmcalls.Record {
	t.Helper()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "try-record", "")
	artifact := filepath.Join(ws, "artifact.md")
	tmux := &artifactOnPasteTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: slices.Repeat([]string{"working ❯"}, 20)}, artifact: artifact}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), TokenResolver: resolver})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: prof, Model: "deep", Prompt: "work",
		Workspace: ws, Worktree: ws, ProjectRoot: ws, ArtifactPath: artifact, Agent: "builder", Attempt: 3,
	}); err != nil {
		t.Fatal(err)
	}
	return readTryRecords(t, ws)[0]
}

func TestTryRecord_ATmuxDispatchRecordsTheEffortItSentAndTheR0Rung(t *testing.T) {
	t.Parallel()
	rec := launchClaudeTmuxTry(t, nil)

	want := LaunchEffort("claude-tmux", "deep", policy.EffortTable{}, "try-record")
	if want == "" || rec.Effort != want {
		t.Fatalf("effort = %q, want the launch effort %q", rec.Effort, want)
	}
	if rec.Rung != string(retryladder.R0) || rec.Context != string(retryladder.ContextNew) || rec.LadderStage != string(retryladder.StageShadow) {
		t.Fatalf("ladder fields = rung %q context %q stage %q, want R0 new shadow", rec.Rung, rec.Context, rec.LadderStage)
	}
	if rec.FailureClass != "" || rec.Fingerprint != "" || rec.LadderWould != "" {
		t.Fatalf("an R0 try names no earlier failure: %+v", rec)
	}
	if rec.Attempt != 3 {
		t.Fatalf("attempt = %d, want the request's 3", rec.Attempt)
	}
}

func TestTryRecord_ServedModelIsTheModelTheTranscriptReportsNotTheTier(t *testing.T) {
	t.Parallel()
	rec := launchClaudeTmuxTry(t, func(tokenusage.Window) (tokenusage.Result, error) {
		return tokenusage.Result{Source: tokenusage.SourceTranscript, ServedModel: "claude-opus-5-5"}, nil
	})

	if rec.ServedModel != "claude-opus-5-5" {
		t.Fatalf("served_model = %q, want the transcript's model id", rec.ServedModel)
	}
}

func TestTryRecord_ServedModelStaysEmptyWhenTheCLIReportsNone(t *testing.T) {
	t.Parallel()
	rec := launchClaudeTmuxTry(t, nil)

	if rec.ServedModel != "" {
		t.Fatalf("served_model = %q, want empty: no CLI report, and the tier alias %q is not a model", rec.ServedModel, rec.Model)
	}
}

func TestTryRecord_ServedModelIsTheVerifiedFooterLabel(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "try-record-agy", "")
	artifact := filepath.Join(ws, "artifact.md")
	tmux := &artifactOnPasteTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: slices.Repeat([]string{agyFooterClaudeOpus}, 60)}, artifact: artifact}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"})})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "agy-claude-tmux", Profile: prof, Model: "deep", Prompt: "work",
		Workspace: ws, Worktree: ws, ProjectRoot: ws, ArtifactPath: artifact, Agent: "builder",
	}); err != nil {
		t.Fatal(err)
	}
	if rec := readTryRecords(t, ws)[0]; rec.ServedModel != "Claude Opus 5.5 (High)" || rec.Model != "deep" {
		t.Fatalf("record model %q served %q, want the tier deep requested and the verified footer label served as the agy model id", rec.Model, rec.ServedModel)
	}
}

func TestTryRecord_ATranscriptModelOutranksTheFooterLabel(t *testing.T) {
	t.Parallel()
	rec := llmcalls.Record{ServedModel: "claude-opus-5-5"}
	markTry(launchRun{effort: "high", servedModel: "Claude Opus 5.5 · high"}, nil)(&rec)

	if rec.ServedModel != "claude-opus-5-5" || rec.Effort != "high" {
		t.Fatalf("record = %+v, want the transcript model kept and the effort high", rec)
	}
	rec = llmcalls.Record{}
	markTry(launchRun{servedModel: "Claude Opus 5.5 · high"}, nil)(&rec)
	if rec.ServedModel != "Claude Opus 5.5 · high" {
		t.Fatalf("served_model = %q, want the footer label when no transcript reports one", rec.ServedModel)
	}
}

func TestServedModelOfFooterLabel_SpellsTheLabelAsTheAgyModelID(t *testing.T) {
	t.Parallel()
	for label, want := range map[string]string{
		"Claude Opus 5.5 · high":  "Claude Opus 5.5 (High)",
		"Gemini 3.8 Flash · low":  "Gemini 3.8 Flash (Low)",
		"Gemini 3.8 Flash (High)": "Gemini 3.8 Flash (High)",
		"Claude Opus 5.5 · ":      "Claude Opus 5.5 · ",
	} {
		if got := servedModelOfFooterLabel(label); got != want {
			t.Errorf("servedModelOfFooterLabel(%q) = %q, want %q", label, got, want)
		}
	}
}
