package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
)

func TestParseLoopArgs_PerAgentCLIFlag(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	var stderr bytes.Buffer
	_, rc := parseLoopArgs([]string{"--cli", "noequals", "--goal-text", "x"}, &stderr)
	if rc != 10 {
		t.Errorf("rc=%d, want 10 on malformed --cli", rc)
	}
	if !strings.Contains(stderr.String(), "agent=cli") {
		t.Errorf("stderr should hint the agent=cli format; got %q", stderr.String())
	}
}

func TestAgentOverrides_CarryTheCLIAndModelFlagsAsTypedOverrides(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{
		Strategy:      "balanced",
		PerAgentCLI:   map[string]string{"auditor": "claude-tmux", "tdd-engineer": "agy-tmux"},
		PerAgentModel: map[string]string{"auditor": "opus", "build-planner": "haiku"},
	}
	got := cfg.agentOverrides()
	want := cliroute.Overrides{
		"auditor":       {CLI: "claude-tmux", Model: "opus"},
		"tdd-engineer":  {CLI: "agy-tmux"},
		"build-planner": {Model: "haiku"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("agentOverrides = %+v, want %+v", got, want)
	}
	if env := carriedEnv(nil); len(env) != 0 {
		t.Errorf("the overrides must not reach the cycle env: %v", env)
	}
}
