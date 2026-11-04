package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

func fixProofRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	registry, err := os.ReadFile(filepath.Join("..", "..", "..", journeys.RegistryPath))
	if err != nil {
		t.Fatal(err)
	}
	repo := gittest.Fixture(t)
	repo.Write("README.md", "Read the notes.\n")
	repo.Write(journeys.RegistryPath, string(registry))
	repo.Git("add", ".")
	repo.Git("commit", "-qm", "seed the registry")
	repo.Write("NOTES.md", "Run the check.\n")
	repo.Git("add", "NOTES.md")
	return repo
}

func commitGateWithMessage(t *testing.T, repo, reviewers, msg string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := dispatch([]string{"commit-gate", "run", "--project-root", repo, "--reviewers", reviewers, "--no-install", "--message", msg}, nil, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestJourney_AFixCommitWithNoFixJourneyTrailerGetsFixProofMissingAndStillPasses(t *testing.T) {
	t.Parallel()
	repo := fixProofRepo(t).Dir
	trailers := "\n\nCo-Authored-By: A <a@example.com>\nCloses-Inbox: item\n"
	cases := []struct {
		msg, want string
	}{
		{"fix: resume the paused cycles\n\nThe resume walks each lane." + trailers, "FIX_PROOF_MISSING"},
		{"fix: resume the paused cycles" + trailers + "Fix-Journey: none\n", "FIX_PROOF_MISSING"},
		{"fix: resume the paused cycles" + trailers + "Fix-Journey: no-such-journey\n", "FIX_PROOF_UNKNOWN"},
		{"fix: resume the paused cycles" + trailers + "Fix-Journey: concurrent-resume\n", ""},
		{"fix: correct a log line" + trailers + "Fix-Journey: none the fix changes only a message\n", ""},
		{"feat: add a verb" + trailers, ""},
	}
	for _, c := range cases {
		rc, stdout, stderr := commitGateWithMessage(t, repo, "docs-review", c.msg)
		if rc != 0 || len(strings.TrimSpace(stdout)) != 64 {
			t.Errorf("message %q: rc %d stdout %q stderr %q, want a pass: the shadow stage blocks nothing", c.msg, rc, stdout, stderr)
		}
		got := strings.Contains(stderr, "FIX_PROOF_")
		if c.want == "" && got || c.want != "" && !strings.Contains(stderr, c.want) {
			t.Errorf("message %q: stderr %q, want the code %q", c.msg, stderr, c.want)
		}
	}
}

func TestJourney_TheFixProofEventReachesTheEventChannels(t *testing.T) {
	t.Parallel()
	repo := fixProofRepo(t).Dir

	if rc, _, stderr := commitGateWithMessage(t, repo, "docs-review", "fix: a\n"); rc != 0 {
		t.Fatalf("rc %d stderr %q, want a pass", rc, stderr)
	}

	if !treeHolds(t, filepath.Join(repo, ".evolve"), "FIX_PROOF_MISSING") {
		t.Fatalf("no file under .evolve holds FIX_PROOF_MISSING: the event did not reach the event channels")
	}
}

func treeHolds(t *testing.T, dir, needle string) bool {
	t.Helper()
	found := false
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		found = found || bytes.Contains(raw, []byte(needle))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestJourney_TheOffStageOfThePolicyStopsTheFixProofReport(t *testing.T) {
	t.Parallel()
	repo := fixProofRepo(t).Dir
	if err := os.MkdirAll(filepath.Join(repo, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "policy.json"), []byte(`{"regression_proof":{"stage":"off"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, _, stderr := commitGateWithMessage(t, repo, "docs-review", "fix: a\n")

	if rc != 0 || strings.Contains(stderr, "FIX_PROOF_") {
		t.Fatalf("rc %d stderr %q, want a pass with no fix-proof report at off", rc, stderr)
	}
}

func TestJourney_AJourneyRowGoesAwayOnlyWithRetiredAndAReason(t *testing.T) {
	t.Parallel()
	fixture := fixProofRepo(t)
	repo := fixture.Dir
	path := filepath.Join(repo, journeys.RegistryPath)
	reg, err := journeys.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	write := func(rows []journeys.Row) {
		raw, err := json.MarshalIndent(journeys.Registry{Rows: rows}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		fixture.Git("add", journeys.RegistryPath)
	}

	write(reg.Rows[1:])
	rc, _, stderr := commitGateWithMessage(t, repo, "code-simplifier,code-reviewer", "test: drop a row\n")
	if rc == 0 || !strings.Contains(stderr, reg.Rows[0].ID) {
		t.Fatalf("rc %d stderr %q, want a refusal that names the dropped row %s", rc, stderr, reg.Rows[0].ID)
	}

	reg.Rows[0].Retired = &journeys.Retired{Reason: "the journey moved to a new test"}
	write(reg.Rows)
	if rc, _, stderr := commitGateWithMessage(t, repo, "code-simplifier,code-reviewer", "test: retire a row\n"); rc != 0 {
		t.Fatalf("rc %d stderr %q, want a pass for a row retired with a reason", rc, stderr)
	}
}
