package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/faketmux"
)

func TestRun_AnAgyREPLLaunchDrawsTheFooterLabelOfItsModelBeforeTheBootMarkers(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	rc := run([]string{"--model", "Claude Sonnet 5.5 (Medium)", "--dangerously-skip-permissions"}, strings.NewReader(""), &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	lines := strings.Split(stdout.String(), "\n")
	if lines[0] != "? for shortcuts  Claude Sonnet 5.5 (Medium)" || lines[1] != replBootMarkers {
		t.Errorf("an agy REPL must draw its footer label, then the boot markers; got %q", stdout.String())
	}
}

func TestPrintAgyFooter_OnlyAnAgyLaunchWithAModelDrawsTheFooter(t *testing.T) {
	t.Parallel()
	for _, inv := range []Invocation{
		{Style: "agy"},
		{Style: "codex", Model: "gpt-5.6-terra"},
		{Style: "claude", Model: "sonnet"},
	} {
		var stdout bytes.Buffer
		printAgyFooter(&stdout, inv)
		if stdout.Len() != 0 {
			t.Errorf("%+v drew %q, want no footer", inv, stdout.String())
		}
	}
}

func TestParseArgs_TheModelOfEitherFlagSpellingIsKept(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--model", "m1"}, {"-m", "m1"}} {
		inv, err := parseArgs(args, bytes.NewReader(nil))
		if err != nil || inv.Model != "m1" {
			t.Errorf("parseArgs(%q) = %+v, %v; want model m1", args, inv, err)
		}
	}
}

func TestRun_TheFakeTmuxFlagServesTheTmuxCommandLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	rc := run([]string{faketmux.Flag, root, "-L", "s", "start-server"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "s", "sessions")); err != nil {
		t.Errorf("start-server through evolve-fake-cli made no server: %v", err)
	}
	if rc := run([]string{faketmux.Flag, root, "nope"}, nil, &stdout, &stderr); rc != 1 {
		t.Errorf("an unknown tmux command = rc %d, want 1", rc)
	}
}
