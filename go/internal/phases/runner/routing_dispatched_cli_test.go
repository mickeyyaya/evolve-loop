package runner

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var agentLineCLI = regexp.MustCompile(`\[runner\] phase=builder agent=builder cli=(\S+)`)

func deepCeilingBuilder(t *testing.T, bridge core.Bridge) (*BaseRunner, string) {
	t.Helper()
	root := writeFallbackProfile(t, "evolve-builder", "agy-tmux", []string{"claude-tmux"})
	block := policy.CLIRouting{
		CLIs:    []string{"agy", "claude"},
		Default: []string{"agy", "claude"},
		Tiers:   map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}},
	}
	b := New(Options{Hooks: &fakeHooks{phase: "builder", agent: "evolve-builder", prompt: "x", verdict: core.VerdictPASS, nextPhase: "audit"},
		Bridge: bridge, Prompts: fakePromptsFS("evolve-builder", "x"), Router: declaredRouter(t, root, block)})
	return b, root
}

func builderPreparation(root string) phasePreparation {
	dir := filepath.Join(root, ".evolve", "profiles")
	prof, _ := profiles.NewFromDir(dir).Get("builder")
	return phasePreparation{phase: "builder", profileDir: dir, profileName: "builder", profilePath: filepath.Join(dir, "builder.json"), profile: &prof}
}

func loggedCLI(t *testing.T, stderr string) string {
	t.Helper()
	m := agentLineCLI.FindAllStringSubmatch(stderr, -1)
	if len(m) != 1 {
		t.Fatalf("want exactly one agent dispatch line, got %d:\n%s", len(m), stderr)
	}
	return m[0][1]
}

func TestLogDecision_ADeepSeatWhoseChainStartsWithAForbiddenCLILogsTheDispatchedCLI(t *testing.T) {
	cases := []struct {
		name        string
		tier        string
		wantCLI     string
		wantSkipped bool
	}{
		{"the deep ceiling forbids the chain's first entry", "deep", "claude-tmux", true},
		{"no tier ceiling applies to the seat", "", "agy-tmux", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, root := deepCeilingBuilder(t, &fakeBridge{})
			req := core.PhaseRequest{ProjectRoot: root, ModelRoutingTier: tc.tier}
			stderr := captureStderrF4(t, func() {
				if _, resp, err := b.resolveDispatchPlan(req, builderPreparation(root)); err != nil || resp != nil {
					t.Fatalf("resolveDispatchPlan: %v %+v", err, resp)
				}
			})
			if got := loggedCLI(t, stderr); got != tc.wantCLI {
				t.Errorf("logged cli=%s, want the dispatched cli=%s:\n%s", got, tc.wantCLI, stderr)
			}
			skipLine := skippedLine(stderr, "agy-tmux")
			if tc.wantSkipped && (skipLine == "" || !strings.Contains(skipLine, "deep")) {
				t.Errorf("the walk's skipped candidate agy-tmux is logged with the deep tier ceiling as its reason:\n%s", stderr)
			}
			if !tc.wantSkipped && skipLine != "" {
				t.Errorf("a seat with no ceiling skips nothing, yet logged %q", skipLine)
			}
		})
	}
}

func skippedLine(stderr, cli string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "[runner] phase=builder") && strings.Contains(line, "skipped") && strings.Contains(line, cli) {
			return line
		}
	}
	return ""
}

func TestRun_TheBridgeLaunchesTheCLITheRunnerLogged(t *testing.T) {
	fb := &fakeBridge{writeArtifact: "# build artifact\n## Files Modified\n- a.go\n"}
	b, root := deepCeilingBuilder(t, fb)
	stderr := captureStderrF4(t, func() {
		if _, err := b.Run(context.Background(), core.PhaseRequest{Cycle: 9, ProjectRoot: root, Workspace: t.TempDir(), ModelRoutingTier: "deep"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
	if fb.gotReq.CLI != "claude-tmux" {
		t.Fatalf("the deep ceiling launches claude-tmux, got %q", fb.gotReq.CLI)
	}
	if got := loggedCLI(t, stderr); got != fb.gotReq.CLI {
		t.Errorf("runner logged cli=%s but the bridge launched cli=%s:\n%s", got, fb.gotReq.CLI, stderr)
	}
}
