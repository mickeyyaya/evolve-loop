package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseLoopArgs_PerAgentCLIFlag(t *testing.T) {
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{
		"--cli", "auditor=claude-tmux",
		"--cli", "builder=agy-tmux",
		"--goal-text", "x",
	}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	want := map[string]string{
		"auditor": "claude-tmux",
		"builder": "agy-tmux",
	}
	if !reflect.DeepEqual(cfg.PerAgentCLI, want) {
		t.Errorf("PerAgentCLI=%v, want %v", cfg.PerAgentCLI, want)
	}
}

func TestParseLoopArgs_PerAgentModelFlag(t *testing.T) {
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{
		"--model", "auditor=opus",
		"--model", "builder=llama3.1:8b",
		"--goal-text", "x",
	}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	want := map[string]string{
		"auditor": "opus",
		"builder": "llama3.1:8b",
	}
	if !reflect.DeepEqual(cfg.PerAgentModel, want) {
		t.Errorf("PerAgentModel=%v, want %v", cfg.PerAgentModel, want)
	}
}

func TestParseLoopArgs_MalformedCLIFlagRejected(t *testing.T) {
	var stderr bytes.Buffer
	_, rc := parseLoopArgs([]string{"--cli", "noequals", "--goal-text", "x"}, &stderr)
	if rc != 10 {
		t.Errorf("rc=%d, want 10 on malformed --cli", rc)
	}
	if !strings.Contains(stderr.String(), "agent=cli") {
		t.Errorf("stderr should hint the agent=cli format; got %q", stderr.String())
	}
}

func TestBuildCycleEnv_PerAgentCLI_TranslatesToEvolveEnvKey(t *testing.T) {
	cfg := loopConfig{
		Strategy: "balanced",
		PerAgentCLI: map[string]string{
			"auditor":      "claude-tmux",
			"tdd-engineer": "agy-tmux",
		},
	}
	env := buildCycleEnv(cfg, nil)
	if got := env["EVOLVE_AUDITOR_CLI"]; got != "claude-tmux" {
		t.Errorf("EVOLVE_AUDITOR_CLI=%q, want claude-tmux", got)
	}
	if got := env["EVOLVE_TDD_ENGINEER_CLI"]; got != "agy-tmux" {
		t.Errorf("EVOLVE_TDD_ENGINEER_CLI=%q, want agy-tmux (dash→underscore in agent name)", got)
	}
}

func TestBuildCycleEnv_PerAgentModel_TranslatesToEvolveEnvKey(t *testing.T) {
	cfg := loopConfig{
		Strategy: "balanced",
		PerAgentModel: map[string]string{
			"auditor":       "opus",
			"build-planner": "haiku",
		},
	}
	env := buildCycleEnv(cfg, nil)
	if got := env["EVOLVE_AUDITOR_MODEL"]; got != "opus" {
		t.Errorf("EVOLVE_AUDITOR_MODEL=%q, want opus", got)
	}
	if got := env["EVOLVE_BUILD_PLANNER_MODEL"]; got != "haiku" {
		t.Errorf("EVOLVE_BUILD_PLANNER_MODEL=%q, want haiku", got)
	}
}

func TestBuildCycleEnv_FlagOverridesInheritedEnv(t *testing.T) {
	cfg := loopConfig{
		Strategy:    "balanced",
		PerAgentCLI: map[string]string{"auditor": "claude-tmux"},
	}
	env := buildCycleEnv(cfg, []string{"EVOLVE_AUDITOR_CLI=inherited-from-shell"})
	if env["EVOLVE_AUDITOR_CLI"] != "claude-tmux" {
		t.Errorf("flag must override inherited env; got %q", env["EVOLVE_AUDITOR_CLI"])
	}
}

func TestPhaseEnvAgentKey(t *testing.T) {
	// phaseEnvAgentKey must mirror envchain.PhaseEnvKey's normalization, or the
	// runner's EVOLVE_<AGENT>_CLI lookup misses.
	cases := []struct {
		in, want string
	}{
		{"auditor", "AUDITOR"},
		{"tdd-engineer", "TDD_ENGINEER"},
		{"build-planner", "BUILD_PLANNER"},
		{"scout", "SCOUT"},
		{"AlreadyUpper", "ALREADYUPPER"},
	}
	for _, c := range cases {
		if got := phaseEnvAgentKey(c.in); got != c.want {
			t.Errorf("phaseEnvAgentKey(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}
