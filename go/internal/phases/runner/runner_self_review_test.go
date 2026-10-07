package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRunner_ASourceWriterThatLoadedTheSelfReviewSkillOwesASelfReviewSection(t *testing.T) {
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	document := map[string]string{config.SignalDeliverableKind: config.DeliverableKindDocument}
	const without = "# Build Report\n## Changes\n- a.go\n"
	const scores = "- Scores: composite 0.91, correctness 0.9, security 1.0, performance 0.9, maintainability 0.85\n"
	const audit = "# Audit Report\n\n## Verdict\n**PASS**\n"
	const reviewerOverlay = `{"overlays":{"rules":[{"phases":["audit"],"skills":["code-review-simplify"]}]}}`
	const craftOnlyOverlay = `{"overlays":{"rules":[{"writes_source":true,"skills":["engineering-craft"]}]}}`
	for _, tc := range []struct {
		name     string
		phase    string
		agent    string
		readOnly bool
		writable []string
		signals  map[string]string
		report   string
		verdict  string
		overlays string
		want     int
	}{
		{"a code build report without the section", "build", "evolve-builder", false, nil, code, without, core.VerdictPASS, "", 1},
		{"a code tdd report without the section", "tdd", "evolve-tdd-engineer", false, nil, code, without, core.VerdictWARN, "", 1},
		{"a code build report with the section and its scores", "build", "evolve-builder", false, nil, code, without + "## Self-Review\n" + scores, core.VerdictPASS, "", 0},
		{"the lower-case heading counts", "build", "evolve-builder", false, nil, code, without + "## Self-review\n" + scores, core.VerdictPASS, "", 0},
		{"the template's empty heading is no record", "build", "evolve-builder", false, nil, code, without + "## Self-Review\n<!-- Step 5.6: the Self-review block -->\n", core.VerdictPASS, "", 1},
		{"a prose mention is no record", "build", "evolve-builder", false, nil, code, without + "I skipped the `## Self-Review` section.\n" + scores, core.VerdictPASS, "", 1},
		{"a failed build owes nothing more", "build", "evolve-builder", false, nil, code, without, core.VerdictFAIL, "", 0},
		{"a read-only reviewer that loads the skill writes no source", "audit", "evolve-auditor", true, nil, code, audit, core.VerdictPASS, reviewerOverlay, 0},
		{"a document build loads no self-review skill", "build", "evolve-builder", false, nil, document, without, core.VerdictPASS, "", 0},
		{"a debugger resolving its conflicts writes source through its writable paths", "debugger", "evolve-debugger", true, []string{"go/internal/x.go"}, code, without, core.VerdictPASS, "", 1},
		{"a writer whose policy drops the self-review skill owes nothing", "build", "evolve-builder", false, nil, code, without, core.VerdictPASS, craftOnlyOverlay, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFallbackProfile(t, tc.agent, "claude-tmux", nil)
			if tc.overlays != "" {
				if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(tc.overlays), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			hooks := &fakeHooks{phase: tc.phase, agent: tc.agent, model: "auto", prompt: "x", verdict: tc.verdict}
			center := signalcenter.New()
			var got []signalcenter.Event
			center.Subscribe(func(e signalcenter.Event) {
				if e.Code == CodeSelfReviewMissing {
					got = append(got, e)
				}
			})
			fb := &fakeBridge{writeArtifact: tc.report}
			r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS(tc.agent, "x"),
				Signals: func() *signalcenter.Center { return center }})
			req := core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir(), Cycle: 42, Signals: tc.signals, WorktreeReadOnly: tc.readOnly, WorktreeWritablePaths: tc.writable}
			resp, err := r.Run(context.Background(), req)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			center.Flush()
			if tc.overlays != "" && len(fb.gotReq.Skills) != 1 {
				t.Fatalf("precondition: the policy rule must load exactly its one skill, got %v", fb.gotReq.Skills)
			}
			if resp.Verdict != tc.verdict {
				t.Errorf("verdict = %s, want %s: the self-review check is advisory and never changes a verdict", resp.Verdict, tc.verdict)
			}
			if len(got) != tc.want {
				t.Fatalf("%s signals = %+v, want %d", CodeSelfReviewMissing, got, tc.want)
			}
			if tc.want == 0 {
				return
			}
			e := got[0]
			if e.Severity != signalcenter.SeverityWarn || e.Module != signalcenter.ModuleRunner || e.Phase != tc.phase || e.Cycle != 42 ||
				e.Fields["skill"] != "code-review-simplify" || e.Fields["section"] != "## Self-Review" {
				t.Errorf("signal = %+v, want one runner WARN for %s in cycle 42 naming the skill and the section", e, tc.phase)
			}
		})
	}
}

func TestRunner_TheSelfReviewObligationFollowsTheFinalAttemptsSkills(t *testing.T) {
	for _, tc := range []struct {
		name     string
		skillCLI string
		want     int
	}{
		{"only the walled primary loaded the skill", "codex-tmux", 0},
		{"only the fallback that ran loaded the skill", "claude-tmux", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFallbackProfile(t, "evolve-builder", "codex-tmux", []string{"claude-tmux"})
			pol := `{"overlays":{"rules":[{"clis":["` + tc.skillCLI + `"],"writes_source":true,"skills":["code-review-simplify"]}]}}`
			if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(pol), 0o644); err != nil {
				t.Fatal(err)
			}
			sb := &scriptedBridge{responses: map[string]scriptedResp{
				"codex-tmux":  {resp: core.BridgeResponse{ExitCode: 80, Stderr: "REPL boot timeout"}, err: errors.New("bridge: launch exit=80")},
				"claude-tmux": {},
			}}
			center := signalcenter.New()
			got := 0
			center.Subscribe(func(e signalcenter.Event) {
				if e.Code == CodeSelfReviewMissing {
					got++
				}
			})
			hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
			r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-builder", "x"), Signals: func() *signalcenter.Center { return center }})
			req := core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir(), Signals: map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}}
			if _, err := r.Run(context.Background(), req); err != nil {
				t.Fatalf("Run: %v", err)
			}
			center.Flush()
			if len(sb.calls) != 2 {
				t.Fatalf("precondition: want the walled primary then the fallback, got %v", sb.calls)
			}
			if got != tc.want {
				t.Errorf("%s emitted %d, want %d: the obligation follows the skills of the attempt whose report is judged", CodeSelfReviewMissing, got, tc.want)
			}
		})
	}
}
