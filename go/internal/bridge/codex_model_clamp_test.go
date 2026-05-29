package bridge

import (
	"strings"
	"testing"
)

// codex_model_clamp_test.go — cycle-142 incident: the auditor ran codex-tmux
// with model gpt-5.4 (resolved from tier "sonnet"), which a ChatGPT/
// subscription codex account rejects (400 invalid_request_error → model-switch
// modal → 10-min hang → ExitArtifactTimeout). The clamp substitutes a
// ChatGPT-safe model on subscription auth; API-key auth keeps the big model.

func TestCodexAuthMode(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"no api key → subscription", map[string]string{}, "chatgpt"},
		{"api key set + allowed → api-key", map[string]string{"OPENAI_API_KEY": "sk-x", "BRIDGE_ALLOW_OPENAI_API_KEY": "1"}, "api-key"},
		{"api key set but not allowed → subscription (guard would reject anyway)", map[string]string{"OPENAI_API_KEY": "sk-x"}, "chatgpt"},
		{"empty api key → subscription", map[string]string{"OPENAI_API_KEY": ""}, "chatgpt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := Deps{LookupEnv: func(k string) (string, bool) { v, ok := tc.env[k]; return v, ok }}
			if got := codexAuthMode(deps); got != tc.want {
				t.Errorf("codexAuthMode = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClampCodexModelForAuth(t *testing.T) {
	m := Manifest{
		ChatGPTSafeModels:   []string{"gpt-5.4-mini", "gpt-5.2"},
		ChatGPTDefaultModel: "gpt-5.4-mini",
	}
	cases := []struct {
		name      string
		flags     []string
		auth      string
		wantFlags []string
		wantFrom  string
		wantTo    string
	}{
		{
			name:      "chatgpt + unsafe model → clamped",
			flags:     []string{"-m", "gpt-5.4"},
			auth:      "chatgpt",
			wantFlags: []string{"-m", "gpt-5.4-mini"},
			wantFrom:  "gpt-5.4", wantTo: "gpt-5.4-mini",
		},
		{
			name:      "chatgpt + unsafe deep model → clamped",
			flags:     []string{"--yolo", "-m", "gpt-5.5"},
			auth:      "chatgpt",
			wantFlags: []string{"--yolo", "-m", "gpt-5.4-mini"},
			wantFrom:  "gpt-5.5", wantTo: "gpt-5.4-mini",
		},
		{
			name:      "chatgpt + already-safe model → untouched",
			flags:     []string{"-m", "gpt-5.4-mini"},
			auth:      "chatgpt",
			wantFlags: []string{"-m", "gpt-5.4-mini"},
			wantFrom:  "", wantTo: "",
		},
		{
			name:      "chatgpt + other safe model → untouched",
			flags:     []string{"-m", "gpt-5.2"},
			auth:      "chatgpt",
			wantFlags: []string{"-m", "gpt-5.2"},
			wantFrom:  "", wantTo: "",
		},
		{
			name:      "api-key mode → big model preserved",
			flags:     []string{"-m", "gpt-5.4"},
			auth:      "api-key",
			wantFlags: []string{"-m", "gpt-5.4"},
			wantFrom:  "", wantTo: "",
		},
		{
			name:      "no -m flag → untouched",
			flags:     []string{"--yolo"},
			auth:      "chatgpt",
			wantFlags: []string{"--yolo"},
			wantFrom:  "", wantTo: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Snapshot the ENTIRE input to detect any in-place mutation,
			// regardless of where the -m value sits in the slice (NUL-joined so
			// element boundaries can't alias). \x00 never appears in a flag.
			before := strings.Join(tc.flags, "\x00")
			got, from, to := clampCodexModelForAuth(tc.flags, m, tc.auth)
			if strings.Join(got, " ") != strings.Join(tc.wantFlags, " ") {
				t.Errorf("flags = %v, want %v", got, tc.wantFlags)
			}
			if from != tc.wantFrom || to != tc.wantTo {
				t.Errorf("(from,to) = (%q,%q), want (%q,%q)", from, to, tc.wantFrom, tc.wantTo)
			}
			if strings.Join(tc.flags, "\x00") != before {
				t.Errorf("input flags mutated in place: %v", tc.flags)
			}
		})
	}
}

func TestClampCodexModelForAuth_NoPolicyNoClamp(t *testing.T) {
	// A manifest without a ChatGPT-safe set must never clamp (e.g. API-key-only
	// CLIs, or a manifest predating the policy).
	m := Manifest{}
	got, from, to := clampCodexModelForAuth([]string{"-m", "gpt-5.4"}, m, "chatgpt")
	if from != "" || to != "" || strings.Join(got, " ") != "-m gpt-5.4" {
		t.Errorf("no-policy manifest must not clamp; got %v (%q→%q)", got, from, to)
	}
}

// TestCodexTmuxManifest_HasChatGPTClampPolicy pins the data contract: the real
// embedded manifest must declare a ChatGPT-safe set + default, or the clamp is
// inert and cycle-142 regresses silently.
func TestCodexTmuxManifest_HasChatGPTClampPolicy(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.ChatGPTSafeModels) == 0 {
		t.Fatal("codex-tmux manifest must declare chatgpt_safe_models")
	}
	if m.ChatGPTDefaultModel == "" {
		t.Fatal("codex-tmux manifest must declare chatgpt_default_model")
	}
	// The default must itself be in the safe set (else the clamp produces an
	// unsafe model).
	safe := false
	for _, s := range m.ChatGPTSafeModels {
		if s == m.ChatGPTDefaultModel {
			safe = true
		}
	}
	if !safe {
		t.Errorf("chatgpt_default_model %q must be a member of chatgpt_safe_models %v", m.ChatGPTDefaultModel, m.ChatGPTSafeModels)
	}
}
