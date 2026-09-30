//go:build acs

package cycle943

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/skilloverlay"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeSkillFile(t *testing.T, root, name, file, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

func contains(got []string, want string) bool {
	for _, s := range got {
		if s == want {
			return true
		}
	}
	return false
}

type capturingBridge struct {
	mu    sync.Mutex
	calls []core.BridgeRequest
	resp  core.BridgeResponse
	err   error
}

func (b *capturingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.mu.Lock()
	b.calls = append(b.calls, req)
	b.mu.Unlock()
	return b.resp, b.err
}

func (b *capturingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func (b *capturingBridge) firstCall(t *testing.T) core.BridgeRequest {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.calls) == 0 {
		t.Fatalf("bridge.Launch was never called — dispatcher never reached the launch seam")
	}
	return b.calls[0]
}

type nopLedger struct{}

func (nopLedger) Append(context.Context, core.LedgerEntry) error { return nil }
func (nopLedger) Verify(context.Context) error                   { return nil }
func (nopLedger) Iter(context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("nopLedger: Iter unused")
}

func fixedRand(b []byte) (int, error) {
	for i := range b {
		b[i] = 0xAB
	}
	return len(b), nil
}

func TestC943_001_MaterializePrefersCompactWhenPresent(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "fable", "COMPACT.md", "COMPACT_BODY_marker rules ≤15")
	writeSkillFile(t, dir, "fable", "SKILL.md", "SKILL_BODY_marker full discipline")

	prefix, missing := skilloverlay.Materialize(dir, []string{"fable"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none (both files readable)", missing)
	}
	if !strings.Contains(prefix, "COMPACT_BODY_marker") {
		t.Errorf("prefix did not include the COMPACT.md body; got:\n%s", prefix)
	}
	if strings.Contains(prefix, "SKILL_BODY_marker") {
		t.Errorf("prefix included the SKILL.md body — COMPACT.md must WIN when present; got:\n%s", prefix)
	}
}

func TestC943_002_MaterializeFallsBackToSkillWhenNoCompact(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "fable", "SKILL.md", "SKILL_ONLY_marker discipline")

	prefix, missing := skilloverlay.Materialize(dir, []string{"fable"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none (SKILL.md readable)", missing)
	}
	if !strings.Contains(prefix, "SKILL_ONLY_marker") {
		t.Errorf("prefix did not fall back to SKILL.md when COMPACT.md absent; got:\n%s", prefix)
	}
}

func TestC943_003_MaterializeFailsOpenOnMissingBody(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ghost"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prefix, missing := skilloverlay.Materialize(dir, []string{"ghost"})
	if !contains(missing, "ghost") {
		t.Errorf("missing = %v, want it to contain %q (no body on disk)", missing, "ghost")
	}
	if strings.Contains(prefix, "ghost") {
		t.Errorf("prefix must exclude a skill with no readable body; got:\n%s", prefix)
	}
}

func TestC943_004_FormatSkillOverlayLogEmitsResolvedSet(t *testing.T) {
	line := runner.FormatSkillOverlayLog("audit", []string{"fable"}, "deep")
	for _, want := range []string{"phase=audit", "skill-overlays=[fable]", "tier=deep"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q missing token %q", line, want)
		}
	}
}

func TestC943_005_FormatSkillOverlayLogEmptySet(t *testing.T) {
	line := runner.FormatSkillOverlayLog("scout", nil, "balanced")
	if !strings.Contains(line, "skill-overlays=[]") {
		t.Errorf("empty-overlay log line %q must render skill-overlays=[]", line)
	}
	if !strings.Contains(line, "tier=balanced") {
		t.Errorf("empty-overlay log line %q must still carry tier=balanced", line)
	}
}

func subagentDeepProfileFS() *profiles.Loader {
	return profiles.NewFromFS(fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name":"builder","role":"builder","cli":"claude-p",
			"model_tier_default":"deep",
			"output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
	})
}

func runSubagent(t *testing.T, model string) *capturingBridge {
	t.Helper()
	br := &capturingBridge{}
	r, err := subagent.New(subagent.Config{
		Profiles: subagentDeepProfileFS(),
		Bridge:   br,
		Ledger:   nopLedger{},
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
		Rand:     fixedRand,
		GitState: func(context.Context, string) (string, string, error) {
			return "", "", errors.New("not a git repo")
		},
	})
	if err != nil {
		t.Fatalf("subagent.New: %v", err)
	}
	tmp := t.TempDir()
	_, _ = r.Run(context.Background(), subagent.Request{
		Agent:       "builder",
		Cycle:       943,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-943"),
		Prompt:      "do the work",
		Model:       model,
	})
	return br
}

func TestC943_007_SubagentDeepTierAttachesFableOverlay(t *testing.T) {
	br := runSubagent(t, "deep")
	got := br.firstCall(t).Skills
	if !contains(got, "fable") {
		t.Errorf("subagent deep-tier BridgeRequest.Skills = %v, want it to contain %q", got, "fable")
	}
}

func TestC943_009_SubagentBalancedTierAttachesNoOverlay(t *testing.T) {
	br := runSubagent(t, "sonnet")
	got := br.firstCall(t).Skills
	if contains(got, "fable") {
		t.Errorf("subagent balanced-tier BridgeRequest.Skills = %v, want no fable overlay", got)
	}
}

func TestC943_008_RetroDeepTierAttachesFableOverlay(t *testing.T) {
	br := &capturingBridge{}
	phase := retro.New(retro.Config{
		Bridge:  br,
		Prompts: retroPromptsFS(),
		Model:   "deep",
		NowFn:   func() time.Time { return time.Unix(0, 0).UTC() },
	})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       943,
		ProjectRoot: t.TempDir(),
		Workspace:   t.TempDir(),
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	got := br.firstCall(t).Skills
	if !contains(got, "fable") {
		t.Errorf("retro deep-tier BridgeRequest.Skills = %v, want it to contain %q", got, "fable")
	}
}

func retroPromptsFS() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-retrospective.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-retrospective\n---\nretro body"),
		},
	})
}

// acs-predicate: config-check — an "is this feature still deferred?" doc claim
func TestC943_010_OverlaysDocCommentNoLongerClaimsDeferred(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go/internal/policy/overlays.go")
	acsassert.FileNotContains(t, path, "deferred to an out-of-cycle manual ship")
}
