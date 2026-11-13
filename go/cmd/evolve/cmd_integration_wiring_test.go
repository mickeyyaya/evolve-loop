package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestIntegrationProductionEnvSetsEveryPort(t *testing.T) {
	t.Parallel()
	env := productionIntegrationEnv("/repo", time.Date(2026, 11, 4, 5, 41, 12, 0, time.UTC))
	if env.Gh == nil || env.Git == nil || env.Settle == nil || env.Root != "/repo" || env.Wave != "pr20261104t054112" {
		t.Fatalf("env = %+v, want gh, git, settle, root and a wave named by the clock", env)
	}
	if env.Emit == nil || env.KeyRun == nil || !env.Now.Equal(time.Date(2026, 11, 4, 5, 41, 12, 0, time.UTC)) {
		t.Errorf("env = %+v, want the signal emitter, the key run and the clock of the main-red stop", env)
	}
	train := env.train("/repo", io.Discard)
	if train.CI == nil || train.Main == nil || train.Host == nil || train.Inbox == nil || train.Root != "/repo" || train.Wave != env.Wave || train.Composer.Stage != policy.FleetIntegrationShadow {
		t.Errorf("train = %+v, want every port set from the env", train)
	}
	if ci := env.ports().CI; ci.Gh == nil || ci.Settle == nil || ci.Repo.Dir != "/repo" {
		t.Errorf("CI = %#v, want a GhCI with gh and the registration wait", ci)
	}
	if host, ok := train.Host.(integration.GhHost); !ok || host.CI.Gh == nil {
		t.Errorf("Host = %#v, want a GhHost on the bare CI", train.Host)
	}
}

func TestIntegrationPauseForWaitsAndStopsOnCancel(t *testing.T) {
	t.Parallel()
	if err := pauseFor(time.Millisecond)(context.Background()); err != nil {
		t.Errorf("pause = %v, want nil after the wait", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pauseFor(time.Hour)(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("pause = %v, want the cancel", err)
	}
}

func TestIntegrationDocsReviewClassNeverCoversInstructionFiles(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"AGENTS.md", "docs/CLAUDE.md", "agents/evolve-x.md", "skills/a/SKILL.md", ".claude/agents/x.md", "commands/x.md", "guidance/rules/r.md", "GEMINI.md"} {
		if docsOnlyData([]string{"docs/a.md", p}) {
			t.Errorf("the class docs-review covered %s", p)
		}
	}
	if !docsOnlyData([]string{"docs/operations/wave-operations.md", "README.md"}) || docsOnlyData([]string{"a.go"}) {
		t.Error("the class docs-review must cover plain Markdown and only Markdown")
	}
}

func TestIntegrationClassBinderKnowsOnlyTheClosedList(t *testing.T) {
	t.Parallel()
	ev := integration.Evidence{Commits: []string{"c"}, Paths: []string{"a.md"}, DiffSHA: strings.Repeat("a", 64)}
	if _, err := classBinder("docs-review")(ev); err != nil {
		t.Errorf("docs-review on Markdown: %v", err)
	}
	_, err := classBinder("self-declared")(ev)
	if err == nil || !strings.Contains(err.Error(), "docs-review") {
		t.Errorf("an unknown class = %v, want a refusal that lists the known classes", err)
	}
}

func TestIntegrationInboxCloserMovesTheNamedItems(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-z.json"), []byte(`{"id":"item-z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	moved, err := inboxCloser(root)(context.Background(), []string{"item-z", "never-here"})
	if err != nil || !slices.Equal(moved, []string{"item-z"}) {
		t.Fatalf("inboxCloser = %v, %v, want only the item that was in the inbox", moved, err)
	}
	if !inboxProcessed(t, inbox, "item-z") {
		t.Error("item-z is not under processed/")
	}
}

func TestIntegrationBatchFailsOnAnUnreadableRecordDirectoryLoudly(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	env := integrationEnv{Root: t.TempDir(), Wave: "w"}
	if rc := runIntegrationEnv(env, []string{"batch", "5"}, &out, &errb); rc != exitRefused || !strings.Contains(errb.String(), "PR #5") {
		t.Errorf("rc=%d err=%q, want a refusal that names PR #5", rc, errb.String())
	}
}

func TestIntegrationRootComesFromTheWorkingDirectoryAndItsFailureStops(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	broken := func() (string, error) { return "", errors.New("cwd gone") }
	if rc := runIntegrationAt(broken, []string{"watch", "--help"}, &out, &errb); rc != exitIO || !strings.Contains(errb.String(), "cwd gone") {
		t.Errorf("rc=%d err=%q, want exit %d and the cause", rc, errb.String(), exitIO)
	}
	out.Reset()
	errb.Reset()
	dir := func() (string, error) { return t.TempDir(), nil }
	if rc := runIntegrationAt(dir, []string{"watch", "--help"}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "usage: evolve integration watch") {
		t.Errorf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
}
