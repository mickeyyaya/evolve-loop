//go:build acs

package cycle2022

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	binOnce sync.Once
	binDir  string
	binPath string
	binErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		binDir, binErr = os.MkdirTemp("", "acs2022-evolve-*")
		if binErr != nil {
			return
		}
		binPath = filepath.Join(binDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		cmd.WaitDelay = 10 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			binErr = errors.New("go build ./cmd/evolve: " + err.Error() + "\n" + string(out))
		}
	})
	if binErr != nil {
		t.Fatalf("RED: %v", binErr)
	}
	return binPath
}

type shipRun struct {
	rc     int
	stdout string
	stderr string
}

func (r shipRun) combined() string { return r.stdout + r.stderr }

type shipRepo struct {
	*gittest.Repo
}

func newShipRepo(t *testing.T) shipRepo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".gitignore", ".evolve/\n.commit-gate/\n")
	repo.Write(".evolve/state.json", "{}\n")
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "-A")
	repo.Git("-c", "commit.gpgsign=false", "commit", "-q", "-m", "chore: seed")
	repo.Git("remote", "add", "origin", gittest.Bare(t).Dir)
	repo.Git("push", "-q", "origin", "main")
	return shipRepo{repo}
}

func (r shipRepo) head(t *testing.T) string {
	t.Helper()
	return r.Git("rev-parse", "HEAD")
}

func (r shipRepo) status(t *testing.T) string {
	t.Helper()
	return r.Git("status", "--porcelain")
}

func (r shipRepo) committedFiles(t *testing.T) []string {
	t.Helper()
	return strings.Fields(r.Git("show", "--name-only", "--format=", "HEAD"))
}

func (r shipRepo) stagedFiles(t *testing.T) []string {
	t.Helper()
	return strings.Fields(r.Git("diff", "--cached", "--name-only"))
}

func (r shipRepo) attestFor(t *testing.T, paths ...string) {
	t.Helper()
	r.Git(append([]string{"add", "-A", "--"}, paths...)...)
	cmd := exec.Command("git", "-C", r.Dir, "diff", "HEAD")
	diff, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff HEAD: %v", err)
	}
	r.Git("reset", "-q")
	sum := sha256.Sum256(diff)
	att := commitgate.Attestation{TreeStateSHA: hex.EncodeToString(sum[:]), TS: "2026-10-21T00:00:00Z", ReviewWaiver: "comment-only", Tool: "shasum"}
	body, err := att.Marshal()
	if err != nil {
		t.Fatalf("attestation marshal: %v", err)
	}
	r.Write(".commit-gate/attestation.json", string(body))
}

func (r shipRepo) ship(t *testing.T, extra ...string) shipRun {
	t.Helper()
	args := append([]string{"ship", "--class", "manual", "--project-root", r.Dir}, extra...)
	args = append(args, "feat: add the widget")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir = r.Dir
	cmd.WaitDelay = 10 * time.Second
	cmd.Env = scrubbedEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	rc := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("RED: evolve did not run: %v", err)
		}
		rc = exit.ExitCode()
	}
	return shipRun{rc: rc, stdout: stdout.String(), stderr: stderr.String()}
}

func scrubbedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			env = append(env, kv)
		}
	}
	return append(env, "EVOLVE_SHIP_AUTO_CONFIRM=1")
}

func mentions(out, repoDir, file string) bool {
	scrubbed := strings.ReplaceAll(out, repoDir, "<repo>")
	return regexp.MustCompile(`(^|[^A-Za-z0-9_.])` + regexp.QuoteMeta(file) + `($|[^A-Za-z0-9_.])`).MatchString(scrubbed)
}

func equalSets(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	for _, w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}

func TestC2022_001_ManualShipWithPathsCommitsOnlyTheNamedFile(t *testing.T) {
	cases := []struct{ name, named, other string }{
		{"root-level", "a.go", "b.go"},
		{"nested", "pkg/a.go", "pkg/b.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newShipRepo(t)
			repo.Write(tc.named, "package a\n")
			repo.Write(tc.other, "package b\n")
			repo.attestFor(t, tc.named)
			before := repo.head(t)

			run := repo.ship(t, "--paths", tc.named)

			if run.rc != 0 {
				t.Fatalf("RED: ship --paths %s exit=%d, want 0\n%s", tc.named, run.rc, run.combined())
			}
			if repo.head(t) == before {
				t.Fatalf("RED: no commit was made\n%s", run.combined())
			}
			if got := repo.committedFiles(t); !equalSets(got, tc.named) {
				t.Errorf("commit holds %v, want exactly [%s]", got, tc.named)
			}
			if !strings.Contains(repo.status(t), "?? "+tc.other) {
				t.Errorf("%s must stay untracked and out of the commit, status:\n%s", tc.other, repo.status(t))
			}
			if tracked := repo.Git("ls-tree", "-r", "--name-only", "HEAD"); strings.Contains(tracked, tc.other) {
				t.Errorf("%s leaked into HEAD:\n%s", tc.other, tracked)
			}
		})
	}
}

func TestC2022_002_AttestationBoundToADifferentStagedSetIsRefusedUnderPaths(t *testing.T) {
	cases := []struct {
		name     string
		attested []string
	}{
		{"attested-whole-tree", []string{"a.go", "b.go"}},
		{"attested-the-other-file", []string{"b.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newShipRepo(t)
			repo.Write("a.go", "package a\n")
			repo.Write("b.go", "package b\n")
			repo.attestFor(t, tc.attested...)
			before := repo.head(t)

			run := repo.ship(t, "--paths", "a.go")

			if run.rc == 0 {
				t.Errorf("a stale attestation must be refused, exit=0\n%s", run.combined())
			}
			if !strings.Contains(run.combined(), "stale") {
				t.Errorf("RED: refusal must name the stale attestation, got:\n%s", run.combined())
			}
			if repo.head(t) != before {
				t.Errorf("a refused ship must not commit")
			}
		})
	}
}

func TestC2022_003_DryRunWithPathsPrintsTheNamedSetAndChangesNothing(t *testing.T) {
	repo := newShipRepo(t)
	repo.Write("a.go", "package a\n")
	repo.Write("b.go", "package b\n")
	beforeHead, beforeStatus := repo.head(t), repo.status(t)

	run := repo.ship(t, "--dry-run", "--paths", "a.go")

	if run.rc != 0 {
		t.Fatalf("RED: ship --dry-run --paths a.go exit=%d, want 0\n%s", run.rc, run.combined())
	}
	if !mentions(run.combined(), repo.Dir, "a.go") {
		t.Errorf("the staged set must list a.go, got:\n%s", run.combined())
	}
	if mentions(run.combined(), repo.Dir, "b.go") {
		t.Errorf("b.go is outside --paths and must not be listed, got:\n%s", run.combined())
	}
	if repo.head(t) != beforeHead {
		t.Errorf("dry-run moved HEAD")
	}
	if got := repo.stagedFiles(t); len(got) != 0 {
		t.Errorf("dry-run left %v in the index, want it untouched", got)
	}
	if after := repo.status(t); after != beforeStatus {
		t.Errorf("dry-run changed the working tree status:\nbefore:\n%s\nafter:\n%s", beforeStatus, after)
	}
}

func TestC2022_004_PathsNamingOnlyAnUnchangedFileCommitsNothing(t *testing.T) {
	repo := newShipRepo(t)
	repo.Write("b.go", "package b\n")
	repo.attestFor(t, "b.go")
	before := repo.head(t)

	run := repo.ship(t, "--paths", "seed.txt")

	if run.rc != 0 {
		t.Fatalf("RED: naming an unchanged file is a clean no-op, exit=%d\n%s", run.rc, run.combined())
	}
	if repo.head(t) != before {
		t.Errorf("an unchanged named path must not commit the rest of the tree: %v", repo.committedFiles(t))
	}
	if !strings.Contains(repo.status(t), "?? b.go") {
		t.Errorf("b.go must remain untracked, status:\n%s", repo.status(t))
	}
}

func TestC2022_005_WithoutPathsManualShipStillStagesTheWholeTree(t *testing.T) {
	repo := newShipRepo(t)
	repo.Write("a.go", "package a\n")
	repo.Write("b.go", "package b\n")
	repo.attestFor(t, "a.go", "b.go")

	run := repo.ship(t)

	if run.rc != 0 {
		t.Fatalf("whole-tree manual ship exit=%d, want 0\n%s", run.rc, run.combined())
	}
	if got := repo.committedFiles(t); !equalSets(got, "a.go", "b.go") {
		t.Errorf("without --paths the commit holds %v, want [a.go b.go]", got)
	}
}

func TestC2022_006_WithoutPathsAttestationForASubsetIsStillRefused(t *testing.T) {
	repo := newShipRepo(t)
	repo.Write("a.go", "package a\n")
	repo.Write("b.go", "package b\n")
	repo.attestFor(t, "a.go")
	before := repo.head(t)

	run := repo.ship(t)

	if run.rc == 0 || !strings.Contains(run.combined(), "stale") {
		t.Errorf("a subset attestation must be refused as stale, exit=%d\n%s", run.rc, run.combined())
	}
	if repo.head(t) != before {
		t.Errorf("a refused ship must not commit")
	}
}

func TestC2022_007_DryRunWithoutPathsPrintsEveryChangedFileAndChangesNothing(t *testing.T) {
	repo := newShipRepo(t)
	repo.Write("a.go", "package a\n")
	repo.Write("b.go", "package b\n")
	beforeHead, beforeStatus := repo.head(t), repo.status(t)

	run := repo.ship(t, "--dry-run")

	if run.rc != 0 {
		t.Fatalf("ship --dry-run exit=%d, want 0\n%s", run.rc, run.combined())
	}
	for _, file := range []string{"a.go", "b.go"} {
		if !mentions(run.combined(), repo.Dir, file) {
			t.Errorf("RED: the staged set must list %s, got:\n%s", file, run.combined())
		}
	}
	if repo.head(t) != beforeHead {
		t.Errorf("dry-run moved HEAD")
	}
	if got := repo.stagedFiles(t); len(got) != 0 {
		t.Errorf("dry-run left %v in the index, want it untouched", got)
	}
	if after := repo.status(t); after != beforeStatus {
		t.Errorf("dry-run changed the working tree status:\nbefore:\n%s\nafter:\n%s", beforeStatus, after)
	}
}
