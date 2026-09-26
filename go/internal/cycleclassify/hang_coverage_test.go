package cycleclassify

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClassify_HangClassifier_NoShippedFallthrough(t *testing.T) {
	t.Setenv("EVOLVE_HANG_CLASSIFIER", "1")
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return true }

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-99")
	_ = os.MkdirAll(ws, 0o755)
	report := "Some neutral content with no recognized markers anywhere.\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach (no SHIPPED → no reclassify)", r.Class)
	}
}

func TestGitLogFn_ErrorPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir() // NOT a git repo
	prevWD, _ := os.Getwd()
	defer os.Chdir(prevWD)
	_ = os.Chdir(dir)
	if gitLogFn("42") {
		t.Fatalf("gitLogFn outside a git repo should return false")
	}
}

func TestGitLogFn_ProductionPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644)
	run("add", ".")
	run("commit", "-m", "cycle 42 — shipped scout-only")

	// gitLogFn runs git in the process working directory.
	prevWD, _ := os.Getwd()
	defer os.Chdir(prevWD)
	_ = os.Chdir(dir)

	if !gitLogFn("42") {
		t.Fatalf("gitLogFn(42) should find the commit")
	}
	if gitLogFn("999") {
		t.Fatalf("gitLogFn(999) should not match")
	}
}
