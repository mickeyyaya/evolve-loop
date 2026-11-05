package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func stagedDocsRepo(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "Read the notes.\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-qm", "init")
	write("NOTES.md", "Run the check.\n")
	repo.Git("add", "NOTES.md")
	return repo.Dir
}

func TestRunCommitGate_APassingRunPrintsTheAttestedTree(t *testing.T) {
	repo := stagedDocsRepo(t)
	t.Setenv("CG_ATTEST_DIR", t.TempDir())
	var out, errb bytes.Buffer

	rc := RunCommitGate([]string{"run", "--project-root", repo, "--reviewers", "docs-review", "--no-install"}, nil, &out, &errb)

	if rc != 0 || len(strings.TrimSpace(out.String())) != 64 || !strings.Contains(errb.String(), "PASS") {
		t.Fatalf("rc %d stdout %q stderr %q: want PASS and the 64-hex tree sha", rc, out.String(), errb.String())
	}
}

func TestRunCommitGate_RefusesAnUnknownSubcommandAndABadFlag(t *testing.T) {
	for _, args := range [][]string{{"verify"}, {"run", "--no-such-flag"}} {
		var out, errb bytes.Buffer
		if rc := RunCommitGate(args, nil, &out, &errb); rc != commitgate.ExitBadArgs || errb.Len() == 0 {
			t.Errorf("RunCommitGate(%q) rc %d stderr %q, want ExitBadArgs and a reason", args, rc, errb.String())
		}
	}
}

func TestRunCommitGate_WithNoProjectRootTakesTheRepoOfTheWorkingDirectory(t *testing.T) {
	repo := stagedDocsRepo(t)
	t.Setenv("CG_ATTEST_DIR", t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := RunCommitGate([]string{"run", "--reviewers", "docs-review", "--no-install"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc %d stderr %q: the repo of the working directory must be the root", rc, errb.String())
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if rc := RunCommitGate([]string{"run", "--reviewers", "docs-review"}, nil, &out, &errb); rc != commitgate.ExitGitFatal || strings.TrimSpace(errb.String()) != "[commit-gate] not in a git repo" {
		t.Fatalf("rc %d stderr %q: outside a git repo the gate must refuse, with the root reason alone", rc, errb.String())
	}
}

func TestRunCommitGate_FromASubdirectoryTheAttestationIsAtTheRepoRoot(t *testing.T) {
	repo := stagedDocsRepo(t)
	t.Setenv("CG_ATTEST_DIR", "")
	sub := filepath.Join(repo, "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	rc := RunCommitGate([]string{"run", "--reviewers", "docs-review", "--no-install"}, nil, &out, &errb)

	if rc != 0 {
		t.Fatalf("rc %d stderr %q, want a pass", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(sub, commitgate.AttestDirName)); err == nil {
		t.Errorf("the attestation landed under the subdirectory %s, want the repo root", sub)
	}
	if _, err := os.Stat(filepath.Join(repo, commitgate.AttestDirName, "attestation.json")); err != nil {
		t.Errorf("no attestation at the repo root: %v", err)
	}
}

func TestRunCommitGate_AnUnresolvableReviewStoreIsGitFatalWithItsReason(t *testing.T) {
	var out, errb bytes.Buffer

	rc := RunCommitGate([]string{"run", "--project-root", t.TempDir(), "--reviewers", "docs-review", "--no-install"}, nil, &out, &errb)

	if rc != commitgate.ExitGitFatal || !strings.Contains(errb.String(), "[commit-gate] cannot resolve the review store") {
		t.Errorf("rc %d stderr %q, want ExitGitFatal and the review store reason", rc, errb.String())
	}
}
