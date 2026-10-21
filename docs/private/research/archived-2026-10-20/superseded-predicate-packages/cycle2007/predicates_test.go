//go:build acs

package cycle2007

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var stagedWord = regexp.MustCompile(`(?i)\bstaged\b`)

const flagUndefinedMarker = "flag provided but not defined"

type shipResult struct {
	stdout string
	stderr string
	code   int
}

func (r shipResult) combined() string { return r.stdout + r.stderr }

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}
	return bin
}

type shipFixture struct {
	repo   *gittest.Repo
	remote *gittest.Repo
}

func newShipFixture(t *testing.T) shipFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	remote := gittest.Bare(t)
	write(t, repo, ".gitignore", ".commit-gate/\n.evolve/\n")
	write(t, repo, "a.go", "package fixture\n")
	repo.Git("add", "-A")
	repo.Git("-c", "commit.gpgsign=false", "commit", "-q", "-m", "chore: seed")
	repo.Git("remote", "add", "origin", remote.Dir)
	repo.Git("push", "-q", "origin", "main")
	write(t, repo, "a.go", "package fixture\n\nconst A = 1\n")
	write(t, repo, "b.go", "package fixture\n\nconst B = 2\n")
	return shipFixture{repo: repo, remote: remote}
}

func write(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo.Dir, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f shipFixture) attestStagedSet(t *testing.T, paths ...string) {
	t.Helper()
	f.repo.Git(append([]string{"add", "-A", "--"}, paths...)...)
	sum := sha256.Sum256([]byte(f.repo.Git("diff", "HEAD") + "\n"))
	f.repo.Git("reset", "-q")
	att := commitgate.Attestation{
		TreeStateSHA: hex.EncodeToString(sum[:]),
		TS:           "2026-10-20T00:00:00Z",
		ChecksPassed: []string{"go:gofmt", "go:test"},
		ReviewersRun: []string{"code-simplifier", "code-reviewer", "go-reviewer"},
		Tool:         "shasum",
	}
	body, err := att.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.repo.Dir, ".commit-gate"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.repo.Dir, ".commit-gate", "attestation.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f shipFixture) ship(t *testing.T, bin string, args ...string) shipResult {
	t.Helper()
	full := append([]string{"ship", "--class", "manual", "--project-root", f.repo.Dir}, args...)
	cmd := exec.Command(bin, full...)
	cmd.Dir = f.repo.Dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "EVOLVE_SHIP_AUTO_CONFIRM=1", "GIT_TERMINAL_PROMPT=0"}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := shipResult{stdout: stdout.String(), stderr: stderr.String()}
	if exit, ok := err.(*exec.ExitError); ok {
		res.code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run evolve ship: %v", err)
	}
	return res
}

func (f shipFixture) requirePathsFlagDefined(t *testing.T, res shipResult) {
	t.Helper()
	if strings.Contains(res.combined(), flagUndefinedMarker) {
		t.Fatalf("RED: evolve ship has no --paths flag yet: %s", res.stderr)
	}
}

func (f shipFixture) headFiles(t *testing.T) []string {
	t.Helper()
	out := f.repo.Git("show", "--pretty=format:", "--name-only", "HEAD")
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files
}

func TestC2007_001_ManualShipWithPathsCommitsOnlyTheNamedFileAndLeavesUntrackedSiblingAlone(t *testing.T) {
	bin := buildEvolve(t)
	f := newShipFixture(t)
	f.attestStagedSet(t, "a.go")
	seedHead := f.repo.Git("rev-parse", "HEAD")

	res := f.ship(t, bin, "--paths", "a.go", "feat: ship a only")

	f.requirePathsFlagDefined(t, res)
	if res.code != 0 {
		t.Fatalf("ship --paths a.go exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
	}
	if f.repo.Git("rev-parse", "HEAD") == seedHead {
		t.Fatalf("no commit was created\nstderr:\n%s", res.stderr)
	}
	if files := f.headFiles(t); len(files) != 1 || files[0] != "a.go" {
		t.Errorf("commit holds %v, want exactly [a.go]", files)
	}
	if status := f.repo.Git("status", "--porcelain", "--", "b.go"); status != "?? b.go" {
		t.Errorf("b.go status = %q, want it still untracked and unstaged (\"?? b.go\")", status)
	}
}

func TestC2007_002_ManualShipWithPathsRefusesAnAttestationBoundToAWiderStagedSet(t *testing.T) {
	bin := buildEvolve(t)
	f := newShipFixture(t)
	f.attestStagedSet(t, "a.go", "b.go")
	seedHead := f.repo.Git("rev-parse", "HEAD")

	res := f.ship(t, bin, "--paths", "a.go", "feat: ship a only")

	f.requirePathsFlagDefined(t, res)
	if res.code == 0 {
		t.Errorf("ship --paths a.go with an attestation of {a.go,b.go} exit = 0, want a refusal\nstderr:\n%s", res.stderr)
	}
	if !strings.Contains(res.combined(), "stale") {
		t.Errorf("refusal must name the stale attestation, got:\n%s", res.combined())
	}
	if f.repo.Git("rev-parse", "HEAD") != seedHead {
		t.Errorf("a refused ship moved HEAD")
	}
}

func TestC2007_003_ManualShipWithoutPathsStillRefusesAnAttestationBoundToANarrowerStagedSet(t *testing.T) {
	bin := buildEvolve(t)
	f := newShipFixture(t)
	f.attestStagedSet(t, "a.go")
	seedHead := f.repo.Git("rev-parse", "HEAD")

	res := f.ship(t, bin, "feat: ship everything")

	if res.code == 0 || !strings.Contains(res.combined(), "stale") {
		t.Errorf("ship without --paths over an a.go-only attestation: exit = %d, output:\n%s\nwant a stale-attestation refusal", res.code, res.combined())
	}
	if f.repo.Git("rev-parse", "HEAD") != seedHead {
		t.Errorf("a refused ship moved HEAD")
	}
}

func TestC2007_004_ManualShipWithoutPathsStillCommitsTheWholeTree(t *testing.T) {
	bin := buildEvolve(t)
	f := newShipFixture(t)
	f.attestStagedSet(t, "a.go", "b.go")

	res := f.ship(t, bin, "feat: ship everything")

	if res.code != 0 {
		t.Fatalf("ship without --paths exit = %d, want 0\nstderr:\n%s", res.code, res.stderr)
	}
	files := f.headFiles(t)
	if len(files) != 2 {
		t.Errorf("commit holds %v, want a.go and b.go", files)
	}
}

func TestC2007_005_DryRunWithPathsPrintsTheNamedFileAsTheStagedSetAndChangesNothing(t *testing.T) {
	bin := buildEvolve(t)
	f := newShipFixture(t)
	seedHead := f.repo.Git("rev-parse", "HEAD")
	remoteHead := f.remote.Git("rev-parse", "main")

	res := f.ship(t, bin, "--dry-run", "--paths", "a.go", "feat: ship a only")

	f.requirePathsFlagDefined(t, res)
	if res.code != 0 {
		t.Fatalf("ship --dry-run --paths a.go exit = %d, want 0\nstderr:\n%s", res.code, res.stderr)
	}
	staged := stagedSetLines(res.combined())
	if len(staged) == 0 {
		t.Fatalf("dry-run printed no staged-set line:\n%s", res.combined())
	}
	joined := strings.Join(staged, "\n")
	if !strings.Contains(joined, "a.go") || strings.Contains(joined, "b.go") {
		t.Errorf("staged-set lines = %q, want a.go and not b.go", staged)
	}
	if f.repo.Git("rev-parse", "HEAD") != seedHead {
		t.Errorf("dry-run moved HEAD")
	}
	if f.remote.Git("rev-parse", "main") != remoteHead {
		t.Errorf("dry-run pushed to the remote")
	}
	if status := f.repo.Git("status", "--porcelain", "--", "b.go"); status != "?? b.go" {
		t.Errorf("dry-run touched b.go: status = %q, want \"?? b.go\"", status)
	}
}

func TestC2007_006_DryRunWithoutPathsPrintsEveryChangedFileAsTheStagedSet(t *testing.T) {
	bin := buildEvolve(t)
	f := newShipFixture(t)

	res := f.ship(t, bin, "--dry-run", "feat: ship everything")

	if res.code != 0 {
		t.Fatalf("ship --dry-run exit = %d, want 0\nstderr:\n%s", res.code, res.stderr)
	}
	joined := strings.Join(stagedSetLines(res.combined()), "\n")
	if !strings.Contains(joined, "a.go") || !strings.Contains(joined, "b.go") {
		t.Errorf("dry-run staged-set lines = %q, want both a.go and b.go", joined)
	}
}

func stagedSetLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if stagedWord.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return lines
}
