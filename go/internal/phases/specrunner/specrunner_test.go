package specrunner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type fakeBridge struct {
	resp          core.BridgeResponse
	writeArtifact string
	gotReq        core.BridgeRequest
}

func (f *fakeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	return f.resp, nil
}

func (f *fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func fakePrompts(agent, body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/" + agent + ".md": &fstest.MapFile{
			Data: []byte("---\nname: " + agent + "\n---\n" + body),
		},
	})
}

// --- classify evaluator (pure function) ---

func TestEvaluateClassify(t *testing.T) {
	cases := []struct {
		name        string
		rules       *phasespec.ClassifyRules
		artifact    string
		wantVerdict string
		wantDiag    string // substring expected in a diagnostic; "" = no diags
	}{
		{"nil rules empty → FAIL", nil, "   ", core.VerdictFAIL, "empty artifact"},
		{"nil rules content → PASS", nil, "anything", core.VerdictPASS, ""},
		{"fail_if_empty empty → FAIL", &phasespec.ClassifyRules{FailIfEmpty: true}, "", core.VerdictFAIL, "empty artifact"},
		{
			"require section present → PASS",
			&phasespec.ClassifyRules{RequireSections: []string{"## Findings"}},
			"# Report\n## Findings\n- none\n", core.VerdictPASS, "",
		},
		{
			"require section missing → FAIL",
			&phasespec.ClassifyRules{RequireSections: []string{"## Findings"}},
			"# Report\nno section here\n", core.VerdictFAIL, "missing required section",
		},
		{
			"verdict_on_pass override",
			&phasespec.ClassifyRules{VerdictOnPass: core.VerdictWARN},
			"content", core.VerdictWARN, "",
		},
		{
			// cycle-241 declared-semantics-rejection: a fail_if_signal gate
			// without the Stage-3 signal bus can never fire — silently passing
			// it lets an authoring mistake reach runtime undetected (retro
			// 215-231 Practice 4). Loud authoring-time FAIL, not WARN.
			"fail_if_signal without signal bus → FAIL (authoring-time rejection)",
			&phasespec.ClassifyRules{FailIfSignal: map[string]string{"security.severity_max": ">=HIGH"}},
			"content", core.VerdictFAIL, "fail_if_signal",
		},
		{
			"invalid verdict_on_pass → FAIL (fail loud on typo)",
			&phasespec.ClassifyRules{VerdictOnPass: "pass"}, // lowercase typo
			"content", core.VerdictFAIL, "invalid verdict_on_pass",
		},
		{
			"require section mid-line does NOT match (line-anchored)",
			&phasespec.ClassifyRules{RequireSections: []string{"## Findings"}},
			"prose mentioning ## Findings inline\n", core.VerdictFAIL, "missing required section",
		},
		{
			"rules present, FailIfEmpty unset, empty artifact → PASS (explicit opt-out)",
			&phasespec.ClassifyRules{}, "", core.VerdictPASS, "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, diags := evaluateClassify(tc.artifact, tc.rules)
			if verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q", verdict, tc.wantVerdict)
			}
			if tc.wantDiag == "" {
				if len(diags) != 0 {
					t.Errorf("expected no diags, got %+v", diags)
				}
				return
			}
			if !hasDiag(diags, tc.wantDiag) {
				t.Errorf("diags %+v missing substring %q", diags, tc.wantDiag)
			}
		})
	}
}

// TestEvaluateClassify_FailIfSignal_RejectsWithErrorSeverity pins the
// severity of the cycle-241 declared-semantics rejection: the diagnostic
// naming fail_if_signal must be Severity "error" (not "warning") AND the
// verdict must be FAIL. The table above checks verdict+message; this test
// is the severity pin the table's shape cannot express.
func TestEvaluateClassify_FailIfSignal_RejectsWithErrorSeverity(t *testing.T) {
	rules := &phasespec.ClassifyRules{FailIfSignal: map[string]string{"security.severity_max": ">=HIGH"}}

	verdict, diags := evaluateClassify("non-empty artifact", rules)
	if verdict != core.VerdictFAIL {
		t.Errorf("verdict = %q, want %q (fail_if_signal without a signal bus is an authoring error)", verdict, core.VerdictFAIL)
	}

	found := false
	for _, d := range diags {
		if !strings.Contains(d.Message, "fail_if_signal") {
			continue
		}
		found = true
		if d.Severity != "error" {
			t.Errorf("fail_if_signal diagnostic severity = %q, want \"error\" (silent WARN is the retro-215-231 defect class)", d.Severity)
		}
	}
	if !found {
		t.Errorf("no diagnostic names fail_if_signal — rejection must be a named violation; diags=%+v", diags)
	}
}

func hasDiag(diags []core.Diagnostic, sub string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, sub) {
			return true
		}
	}
	return false
}

// --- Hooks accessors ---

func TestHooks_ArtifactFilename(t *testing.T) {
	cases := []struct {
		name string
		spec phasespec.PhaseSpec
		want string
	}{
		{"basename of templated path", phasespec.PhaseSpec{Name: "scout", Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/scout-report.md"}}}, "scout-report.md"},
		{"default when no outputs", phasespec.PhaseSpec{Name: "echo"}, "echo-report.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := hooks{spec: tc.spec}
			if got := h.ArtifactFilename(core.PhaseRequest{}); got != tc.want {
				t.Errorf("ArtifactFilename = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHooks_NameAndAgentDefaults(t *testing.T) {
	h := hooks{spec: phasespec.PhaseSpec{Name: "security-scan"}}
	if h.PhaseName() != "security-scan" {
		t.Errorf("PhaseName = %q", h.PhaseName())
	}
	if h.AgentPromptName() != "evolve-security-scan" {
		t.Errorf("AgentPromptName = %q", h.AgentPromptName())
	}
	if h.DefaultModel() != "auto" {
		t.Errorf("DefaultModel = %q", h.DefaultModel())
	}
}

// --- Run-level: a pure-data spec produces the same dispatch contract as a
// hand-written phase would (artifact path, agent, profile, prompt context). ---

func TestRun_SpecDrivenPhase_PASS(t *testing.T) {
	ws := t.TempDir()
	spec := phasespec.PhaseSpec{
		Name:          "security-scan",
		Outputs:       phasespec.IO{Files: []string{"security-scan-report.md"}},
		PromptContext: []string{"goal"},
		Classify:      &phasespec.ClassifyRules{RequireSections: []string{"## Findings"}, VerdictOnPass: core.VerdictPASS},
		OnPass:        "audit",
	}
	fb := &fakeBridge{writeArtifact: "# Scan\n## Findings\n- no CVEs\n"}
	phase := New(spec, Config{Bridge: fb, Prompts: fakePrompts("evolve-security-scan", "scan body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       12,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		Context:     map[string]string{"goal": "find vulnerabilities"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict = %q, want PASS", resp.Verdict)
	}
	if resp.Phase != "security-scan" {
		t.Errorf("Phase = %q", resp.Phase)
	}
	if resp.NextPhase != "audit" {
		t.Errorf("NextPhase = %q, want audit (from spec.OnPass)", resp.NextPhase)
	}
	if want := filepath.Join(ws, "security-scan-report.md"); fb.gotReq.ArtifactPath != want {
		t.Errorf("ArtifactPath = %q, want %q", fb.gotReq.ArtifactPath, want)
	}
	if want := filepath.Join("/tmp/proj", ".evolve", "profiles", "security-scan.json"); fb.gotReq.Profile != want {
		t.Errorf("Profile = %q, want %q", fb.gotReq.Profile, want)
	}
	if !strings.Contains(fb.gotReq.Prompt, "goal: find vulnerabilities") {
		t.Errorf("Prompt missing prompt_context key; got:\n%s", fb.gotReq.Prompt)
	}
}

func TestRun_SpecDrivenPhase_MissingSection_FAIL(t *testing.T) {
	spec := phasespec.PhaseSpec{
		Name:     "security-scan",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"## Findings"}},
	}
	fb := &fakeBridge{writeArtifact: "# Scan\nno section\n"}
	phase := New(spec, Config{Bridge: fb, Prompts: fakePrompts("evolve-security-scan", "body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict = %q, want FAIL", resp.Verdict)
	}
}

func TestName(t *testing.T) {
	p := New(phasespec.PhaseSpec{Name: "echo-noop"}, Config{})
	if p.Name() != "echo-noop" {
		t.Errorf("Name = %q, want echo-noop", p.Name())
	}
}

// TestRun_NoOnPass_DefersNextPhase confirms an empty OnPass yields an empty
// NextPhase, leaving successor selection to the orchestrator (Stage 1 contract).
func TestRun_NoOnPass_DefersNextPhase(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "echo", Classify: &phasespec.ClassifyRules{}}
	fb := &fakeBridge{writeArtifact: "ok\n"}
	phase := New(spec, Config{Bridge: fb, Prompts: fakePrompts("evolve-echo", "body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.NextPhase != "" {
		t.Errorf("NextPhase = %q, want empty (defer to orchestrator)", resp.NextPhase)
	}
}
