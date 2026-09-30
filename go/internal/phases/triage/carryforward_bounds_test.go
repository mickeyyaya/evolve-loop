package triage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// boundsRunGit runs a git subcommand in dir, failing loudly on error —
// fixture setup is not the behavior under test.
func boundsRunGit(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestCarryforwardCandidatesSection_NewestCommittedFirst(t *testing.T) {
	dir := t.TempDir()
	boundsRunGit(t, dir, nil, "init", "-q", "-b", "main")
	boundsRunGit(t, dir, nil, "config", "user.email", "acs@example.invalid")
	boundsRunGit(t, dir, nil, "config", "user.name", "acs")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	boundsRunGit(t, dir, nil, "add", "base.txt")
	boundsRunGit(t, dir, nil, "commit", "-q", "-m", "base")

	// cycle-alpha sorts first alphabetically but is committed older;
	// cycle-zulu sorts last but is committed newer, so refname order and
	// committerdate order disagree.
	mkBranch := func(name, file string, ts string) {
		boundsRunGit(t, dir, nil, "checkout", "-q", "-b", name, "main")
		if err := os.WriteFile(filepath.Join(dir, file), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		boundsRunGit(t, dir, nil, "add", file)
		env := []string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts}
		boundsRunGit(t, dir, env, "commit", "-q", "-m", "feat "+name)
		boundsRunGit(t, dir, nil, "checkout", "-q", "main")
	}
	mkBranch("cycle-alpha", "alpha.txt", "2026-01-01T00:00:00")
	mkBranch("cycle-zulu", "zulu.txt", "2026-06-01T00:00:00")

	section := CarryforwardCandidatesSection(context.Background(), dir, "main")
	zuluIdx := strings.Index(section, "cycle-zulu")
	alphaIdx := strings.Index(section, "cycle-alpha")
	if zuluIdx == -1 || alphaIdx == -1 {
		t.Fatalf("expected both candidates present, got:\n%s", section)
	}
	if zuluIdx > alphaIdx {
		t.Errorf("expected newer cycle-zulu before older cycle-alpha (newest-committed-first), got:\n%s", section)
	}
}

func TestCarryforwardCandidatesSection_CapsBranchesAndSurfacesTruncation(t *testing.T) {
	dir := t.TempDir()
	boundsRunGit(t, dir, nil, "init", "-q", "-b", "main")
	boundsRunGit(t, dir, nil, "config", "user.email", "acs@example.invalid")
	boundsRunGit(t, dir, nil, "config", "user.name", "acs")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	boundsRunGit(t, dir, nil, "add", "base.txt")
	boundsRunGit(t, dir, nil, "commit", "-q", "-m", "base")

	total := carryforwardCandidatesMaxBranches + 5
	for i := 0; i < total; i++ {
		name := "cycle-b" + strconv.Itoa(i)
		file := "f" + strconv.Itoa(i) + ".txt"
		boundsRunGit(t, dir, nil, "checkout", "-q", "-b", name, "main")
		if err := os.WriteFile(filepath.Join(dir, file), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		boundsRunGit(t, dir, nil, "add", file)
		ts := fmt.Sprintf("2026-01-%02dT00:00:00", (i%28)+1)
		env := []string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts}
		boundsRunGit(t, dir, env, "commit", "-q", "-m", "feat "+name)
		boundsRunGit(t, dir, nil, "checkout", "-q", "main")
	}

	section := CarryforwardCandidatesSection(context.Background(), dir, "main")
	want := fmt.Sprintf("(partial: %d of %d branches probed", carryforwardCandidatesMaxBranches, total)
	if !strings.Contains(section, want) {
		t.Errorf("expected truncation line containing %q, got:\n%s", want, section)
	}
	lines := strings.Split(section, "\n")
	candidateLines := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  - cycle-") {
			candidateLines++
		}
	}
	if candidateLines > carryforwardCandidatesMaxBranches {
		t.Errorf("rendered %d candidate lines, want <= cap %d", candidateLines, carryforwardCandidatesMaxBranches)
	}
}

func TestCarryforwardCandidatesSection_RespectsExpiredContext(t *testing.T) {
	dir := t.TempDir()
	boundsRunGit(t, dir, nil, "init", "-q", "-b", "main")
	boundsRunGit(t, dir, nil, "config", "user.email", "acs@example.invalid")
	boundsRunGit(t, dir, nil, "config", "user.name", "acs")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	boundsRunGit(t, dir, nil, "add", "base.txt")
	boundsRunGit(t, dir, nil, "commit", "-q", "-m", "base")
	boundsRunGit(t, dir, nil, "checkout", "-q", "-b", "cycle-late", "main")
	if err := os.WriteFile(filepath.Join(dir, "late.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	boundsRunGit(t, dir, nil, "add", "late.txt")
	boundsRunGit(t, dir, nil, "commit", "-q", "-m", "feat")
	boundsRunGit(t, dir, nil, "checkout", "-q", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // guarantee the deadline has elapsed
	section := CarryforwardCandidatesSection(ctx, dir, "main")
	if section != "" {
		t.Errorf("expired context must fail-open to \"\", got:\n%s", section)
	}
}
