package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge/inbox"
)

// cmd_bridge_send_test.go — tests `evolve bridge send`, the deterministic
// CLI that appends one envelope to an agent inbox for live injection.

func TestRunBridge_Send_AppendsEnvelope(t *testing.T) {
	ws := t.TempDir()
	var out, errb bytes.Buffer
	code := runBridge([]string{"send", "--workspace=" + ws, "--agent=build", "hello", "world"}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("send exit = %d, want 0; stderr=%q", code, errb.String())
	}
	envs, err := inbox.NewCursor(ws, "build").Drain()
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("want 1 envelope, got %d", len(envs))
	}
	got := envs[0]
	if got.Kind != inbox.KindCommand {
		t.Errorf("kind = %q, want command (default)", got.Kind)
	}
	if got.Body != "hello world" {
		t.Errorf("body = %q, want %q", got.Body, "hello world")
	}
	if got.Source != "cli" {
		t.Errorf("source = %q, want cli (default)", got.Source)
	}
}

func TestRunBridge_Send_KindAndSource(t *testing.T) {
	ws := t.TempDir()
	var out, errb bytes.Buffer
	code := runBridge([]string{"send", "--workspace=" + ws, "--agent=audit", "--kind=interrupt", "--source=operator", "stop"}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("send exit = %d, want 0; stderr=%q", code, errb.String())
	}
	envs, _ := inbox.NewCursor(ws, "audit").Drain()
	if len(envs) != 1 || envs[0].Kind != inbox.KindInterrupt || envs[0].Source != "operator" {
		t.Fatalf("unexpected envelope: %+v", envs)
	}
}

func TestRunBridge_Send_MissingRequired(t *testing.T) {
	cases := [][]string{
		{"send", "--agent=build", "body"},           // no workspace
		{"send", "--workspace=/tmp/x", "body"},      // no agent
		{"send", "--workspace=/tmp/x", "--agent=b"}, // no body
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		if code := runBridge(args, nil, &out, &errb); code != 10 {
			t.Errorf("args %v: exit = %d, want 10", args, code)
		}
	}
}

func TestRunBridge_Send_BadKind(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"send", "--workspace=/tmp/x", "--agent=b", "--kind=bogus", "body"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("bad-kind exit = %d, want 10", code)
	}
	if !strings.Contains(errb.String(), "kind") {
		t.Errorf("stderr should mention kind; got %q", errb.String())
	}
}
