package main

import (
	"bytes"
	"strings"
	"testing"
)

// cmd_bridge_test.go — tests the `evolve bridge` CLI shim dispatch.

func TestRunBridge_Help(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runBridge([]string{"help"}, nil, &out, &errb); code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "evolve bridge") {
		t.Fatalf("help output missing banner; got %q", out.String())
	}
}

func TestRunBridge_NoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runBridge(nil, nil, &out, &errb); code != 10 {
		t.Fatalf("no-args exit = %d, want 10", code)
	}
}

func TestRunBridge_UnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"frobnicate"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("unknown-sub exit = %d, want 10", code)
	}
	if !strings.Contains(errb.String(), "unknown subcommand") {
		t.Fatalf("stderr should report unknown subcommand; got %q", errb.String())
	}
}

func TestRunBridge_Probe(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runBridge([]string{"probe"}, nil, &out, &errb); code != 0 {
		t.Fatalf("probe exit = %d, want 0", code)
	}
	s := out.String()
	if !strings.Contains(s, `"os"`) || !strings.Contains(s, `"results"`) {
		t.Fatalf("probe JSON should carry os + results; got %q", s)
	}
	// Every known driver should appear in the probe output.
	for _, cli := range []string{"claude-p", "claude-tmux", "codex", "agy"} {
		if !strings.Contains(s, `"`+cli+`"`) {
			t.Fatalf("probe output missing cli %q; got %q", cli, s)
		}
	}
}

func TestRunBridge_LaunchMissingRequiredFlags(t *testing.T) {
	var out, errb bytes.Buffer
	code := runBridge([]string{"launch", "--cli=claude-p"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("launch-missing-flags exit = %d, want 10 (ExitBadFlags)", code)
	}
	if !strings.Contains(errb.String(), "missing required") {
		t.Fatalf("stderr should report missing required flags; got %q", errb.String())
	}
}
