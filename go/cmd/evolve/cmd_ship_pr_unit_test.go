package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

func installScriptedGh(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write scripted gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestShipPrRequestRefusal_NamesEveryRefusedCombination(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		req      shipPrRequest
		class    string
		pushOnly bool
		want     string
	}{
		{"body file without pr", shipPrRequest{bodyFile: "b.md"}, "manual", false, "--pr-body-file requires --pr"},
		{"no pr at all", shipPrRequest{}, "cycle", false, ""},
		{"non manual class", shipPrRequest{enabled: true}, "cycle", false, "--pr is only valid with --class manual"},
		{"push only", shipPrRequest{enabled: true}, "manual", true, "--pr cannot be combined with --push-only"},
		{"accepted", shipPrRequest{enabled: true}, "manual", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.refusal(tc.class, tc.pushOnly); got != tc.want {
				t.Errorf("refusal = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShipPrBranch_FailsOutsideAGitRepository(t *testing.T) {
	t.Parallel()
	_, err := shipPrBranch(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "current branch") {
		t.Fatalf("err = %v, want a current-branch error", err)
	}
}

func TestGhOutput_ReportsEachFailureShape(t *testing.T) {
	t.Run("exit with stderr", func(t *testing.T) {
		installScriptedGh(t, `echo "boom" >&2; exit 1`)
		_, err := ghOutput(context.Background(), t.TempDir(), "pr", "list")
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want the stderr text", err)
		}
	})
	t.Run("exit without stderr", func(t *testing.T) {
		installScriptedGh(t, `exit 1`)
		_, err := ghOutput(context.Background(), t.TempDir(), "pr", "list")
		if err == nil || !strings.Contains(err.Error(), "gh pr list") {
			t.Fatalf("err = %v, want a bare gh pr list error", err)
		}
	})
	t.Run("gh not installed", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := ghOutput(context.Background(), t.TempDir(), "pr", "list")
		if err == nil || !strings.Contains(err.Error(), "gh pr list") {
			t.Fatalf("err = %v, want a gh pr list error", err)
		}
	})
}

func TestFindOpenPullRequest_RejectsAFailingOrMalformedList(t *testing.T) {
	t.Run("gh fails", func(t *testing.T) {
		installScriptedGh(t, `exit 1`)
		if _, _, err := findOpenPullRequest(context.Background(), t.TempDir(), "b"); err == nil {
			t.Fatal("want an error when gh pr list fails")
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		installScriptedGh(t, `echo "not json"`)
		_, _, err := findOpenPullRequest(context.Background(), t.TempDir(), "b")
		if err == nil || !strings.Contains(err.Error(), "gh pr list output") {
			t.Fatalf("err = %v, want a gh pr list output error", err)
		}
	})
}

func TestEnsurePullRequest_StopsWhenTheLookupFails(t *testing.T) {
	installScriptedGh(t, `echo "list down" >&2; exit 1`)
	var out bytes.Buffer

	err := ensurePullRequest(context.Background(), t.TempDir(), "feature/x", "title", "", &out)

	if err == nil || !strings.Contains(err.Error(), "list down") {
		t.Fatalf("err = %v, want the lookup failure", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestResolveShipPrBranch_ReportsEachOutcome(t *testing.T) {
	t.Parallel()
	t.Run("refused combination", func(t *testing.T) {
		var stderr bytes.Buffer
		branch, rc := resolveShipPrBranch(shipPrRequest{enabled: true}, "manual", true, t.TempDir(), &stderr)
		if rc != 10 || branch != "" || !strings.Contains(stderr.String(), "--push-only") {
			t.Errorf("branch=%q rc=%d stderr=%q, want rc 10 naming --push-only", branch, rc, stderr.String())
		}
	})
	t.Run("pr not requested", func(t *testing.T) {
		branch, rc := resolveShipPrBranch(shipPrRequest{}, "manual", false, t.TempDir(), new(bytes.Buffer))
		if rc != 0 || branch != "" {
			t.Errorf("branch=%q rc=%d, want empty and 0", branch, rc)
		}
	})
	t.Run("not a repository", func(t *testing.T) {
		var stderr bytes.Buffer
		_, rc := resolveShipPrBranch(shipPrRequest{enabled: true}, "manual", false, t.TempDir(), &stderr)
		if rc != int(ship.ExitFailure) || !strings.Contains(stderr.String(), "current branch") {
			t.Errorf("rc=%d stderr=%q, want ship.ExitFailure naming the branch lookup", rc, stderr.String())
		}
	})
	t.Run("feature branch", func(t *testing.T) {
		branch, rc := resolveShipPrBranch(shipPrRequest{enabled: true}, "manual", false, repoOnBranch(t, "feature/widget"), new(bytes.Buffer))
		if rc != 0 || branch != "feature/widget" {
			t.Errorf("branch=%q rc=%d, want feature/widget and 0", branch, rc)
		}
	})
}
