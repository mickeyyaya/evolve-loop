package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/credsource"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

type stubPrSource struct{}

func (stubPrSource) Name() string { return "stub" }

func (stubPrSource) Lookup(context.Context, string) (credsource.Answer, error) {
	return credsource.Answer{Source: "stub", Secret: "tok-secret"}, nil
}

func TestRunShipCmd_Pr_LogsWhichCredentialSourceAnsweredBeforeGh(t *testing.T) {
	gh := installFakeGh(t, "[]", createdPullRequestURL, false)
	stubShipRun(t, ship.RunResult{ExitCode: ship.ExitOK}, nil)
	old := shipPrCredentialSource
	t.Cleanup(func() { shipPrCredentialSource = old })
	shipPrCredentialSource = func() (credsource.Source, error) { return stubPrSource{}, nil }
	root := repoOnBranch(t, "feature/widget")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	policy := `{"isolation":{"credential_source":{"stage":"shadow"}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, _, stderr := runShipPr(t, root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr = %q", rc, stderr)
	}
	want := "[credential] shadow: source stub answered for github.com; the push and gh keep today's credential path\n"
	if !strings.Contains(stderr, want) || strings.Contains(stderr, "tok-secret") {
		t.Fatalf("stderr = %q, want %q and no secret", stderr, want)
	}
	if len(gh.createCalls(t)) != 1 {
		t.Fatalf("gh pr create calls = %v, want today's one create", gh.calls(t))
	}
}
