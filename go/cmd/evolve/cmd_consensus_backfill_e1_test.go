package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func consensusDispatchEnv(t *testing.T, policyBody string) (root, ws string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "auditor.json")
	prompt := filepath.Join(root, "prompt.md")
	if err := os.WriteFile(profile, []byte("{not a profile"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prompt, []byte("audit"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws = filepath.Join(root, "ws", "cycle-9")
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("CYCLE", "9")
	t.Setenv("WORKSPACE_PATH", ws)
	t.Setenv("PROFILE_PATH", profile)
	t.Setenv("PROMPT_FILE", prompt)
	return root, ws
}

func TestRunConsensusDispatch_PassesEachEnvInputToTheDispatcher(t *testing.T) {
	cases := []struct {
		name  string
		unset string
		code  int
		want  string
	}{
		{"no cycle", "CYCLE", 2, "[consensus-dispatch] FAIL: CYCLE required"},
		{"no workspace", "WORKSPACE_PATH", 2, "[consensus-dispatch] FAIL: WORKSPACE_PATH required"},
		{"no profile", "PROFILE_PATH", 10, "[consensus-dispatch] PROFILE-ERROR: PROFILE_PATH required"},
		{"no prompt", "PROMPT_FILE", 2, "[consensus-dispatch] FAIL: PROMPT_FILE required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consensusDispatchEnv(t, `{}`)
			t.Setenv(tc.unset, "")
			var out, errb bytes.Buffer
			if code := runConsensusDispatch(nil, nil, &out, &errb); code != tc.code || !strings.Contains(errb.String(), tc.want) {
				t.Errorf("code=%d stderr=%q, want %d and %q", code, errb.String(), tc.code, tc.want)
			}
		})
	}
}

func TestRunConsensusDispatch_ThePolicyOptOutRefusesAfterTheWorkspaceIsMade(t *testing.T) {
	_, ws := consensusDispatchEnv(t, `{"workflow":{"consensus_audit_enabled":false}}`)
	var out, errb bytes.Buffer

	code := runConsensusDispatch(nil, nil, &out, &errb)

	if code != 2 || !strings.Contains(errb.String(), "refusing to run consensus dispatch (operator opt-out)") {
		t.Errorf("code=%d stderr=%q, want the opt-out refusal", code, errb.String())
	}
	if info, err := os.Stat(ws); err != nil || !info.IsDir() {
		t.Errorf("workspace %s not created from WORKSPACE_PATH: %v", ws, err)
	}
}

func TestRunConsensusDispatch_AnEnabledPolicyReachesTheProfileParse(t *testing.T) {
	consensusDispatchEnv(t, `{"workflow":{"consensus_audit_enabled":true}}`)
	var out, errb bytes.Buffer

	code := runConsensusDispatch(nil, nil, &out, &errb)

	if code != 10 || strings.Contains(errb.String(), "opt-out") {
		t.Errorf("code=%d stderr=%q, want the profile parse error and no opt-out", code, errb.String())
	}
}

func TestRaiseText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   convergence.TierEffort
		want string
	}{
		{convergence.TierEffort{}, ""},
		{convergence.TierEffort{Family: "claude", Tier: "top", Model: "opus", Effort: "xhigh"}, "claude top (opus xhigh)"},
		{convergence.TierEffort{Family: "agy", Tier: "deep", Model: "gemini"}, "agy deep (gemini)"},
		{convergence.TierEffort{Family: "agy", Tier: "deep", Effort: "high"}, "agy deep (high)"},
		{convergence.TierEffort{Tier: "deep"}, " deep ()"},
	}
	for _, tc := range cases {
		if got := raiseText(tc.in); got != tc.want {
			t.Errorf("raiseText(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
