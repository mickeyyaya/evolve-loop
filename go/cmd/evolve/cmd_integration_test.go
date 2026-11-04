package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const integrationTestTip = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func integrationFakeGh(watch func() (string, int, error), calls *[][]string) integration.GhFunc {
	conclusion := "success"
	return func(_ context.Context, _ string, args ...string) (string, int, error) {
		*calls = append(*calls, args)
		switch {
		case len(args) > 1 && args[0] == "run" && args[1] == "watch":
			out, code, err := watch()
			if code != 0 {
				conclusion = "failure"
			}
			return out, code, err
		case len(args) > 1 && args[0] == "run" && args[1] == "view":
			return `{"conclusion":"` + conclusion + `"}` + "\n", 0, nil
		}
		return `[{"databaseId":12}]` + "\n", 0, nil
	}
}

func runIntegrationWith(gh integration.GhFunc, args []string, stdout, stderr io.Writer) int {
	return runIntegrationEnv(integrationEnv{Gh: gh}, args, stdout, stderr)
}

func TestIntegrationWatchExitCodes(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		name  string
		watch func() (string, int, error)
		want  int
		out   string
	}{
		{"green", func() (string, int, error) { return "", 0, nil }, 0, "green"},
		{"red", func() (string, int, error) { return "", 1, nil }, exitRefused, "red"},
		{"error", func() (string, int, error) { return "", 0, boom }, exitIO, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var calls [][]string
			var stdout, stderr bytes.Buffer
			rc := runIntegrationWith(integrationFakeGh(c.watch, &calls), []string{"watch", "--tip", integrationTestTip}, &stdout, &stderr)
			if rc != c.want || !strings.Contains(stdout.String(), c.out) {
				t.Errorf("rc=%d stdout=%q stderr=%q, want rc=%d out %q", rc, stdout.String(), stderr.String(), c.want, c.out)
			}
		})
	}
}

func TestIntegrationUsageErrors(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"bogus"}, {"watch"}, {"watch", "--tip", "main"}, {"watch", "--tip", integrationTestTip, "extra"}, {"watch", "--bogus"}} {
		var stdout, stderr bytes.Buffer
		if rc := runIntegrationWith(nil, args, &stdout, &stderr); rc != exitUsage {
			t.Errorf("args %v: rc=%d, want %d", args, rc, exitUsage)
		}
	}
}

func TestIntegrationHelpPrintsUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runIntegrationWith(nil, []string{"watch", "--help"}, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "usage: evolve integration watch") {
		t.Errorf("rc=%d stdout=%q", rc, stdout.String())
	}
}

func TestRunIntegrationNamesTheVerbsWhenNoneIsGiven(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runIntegration(nil, nil, &stdout, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), "record-pr") || !strings.Contains(stderr.String(), "batch") {
		t.Errorf("rc=%d stderr=%q", rc, stderr.String())
	}
}
