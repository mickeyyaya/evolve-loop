package commitgate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// TestDifferentialParity_BashVsGo is the proof-before-delete for Wave B2: it runs
// the ORIGINAL bash commit-gate-runner.sh AND the Go Options.Run over the SAME
// staged fixture tree and asserts they write a byte-identical attestation
// (identical tree_state_sha + identical bytes). The fixture stages a non-code
// (.txt) change so both code paths skip every lint lane — isolating the two
// things byte-compatibility hinges on: the sha256(`git diff HEAD`) binding and
// the attestation's JSON byte layout (field order, indent, inline arrays,
// trailing newline). Lint-lane parity is covered by the unit tests; this test
// proves the artifact the ship-gate reader consumes is the same from either
// writer.
func TestDifferentialParity_BashVsGo(t *testing.T) {
	bashBin, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	// The bash runner stamps `tool` with whichever hasher exists; require one so
	// both writers agree on that field.
	if _, err := exec.LookPath("shasum"); err != nil {
		if _, err := exec.LookPath("sha256sum"); err != nil {
			t.Skip("no shasum/sha256sum on PATH")
		}
	}
	runnerScript := findRunnerScript(t)

	reviewers := "code-simplifier,code-reviewer"

	// ── Build a real git repo with one staged non-code change. ──────────────
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	writeFile(t, filepath.Join(dir, "notes.txt"), "one\n")
	runGit("add", "notes.txt")
	runGit("commit", "-q", "-m", "init")
	// Mutate the tracked file so `git diff HEAD` is non-empty (an empty change
	// set is ExitFail in both writers).
	writeFile(t, filepath.Join(dir, "notes.txt"), "one\ntwo\n")

	// ── (1) Bash writer → .commit-gate/attestation.json ─────────────────────
	bashAttDir := filepath.Join(dir, "bash-att")
	bashCmd := exec.Command(bashBin, runnerScript, "--reviewers", reviewers, "--files", "notes.txt")
	bashCmd.Dir = dir
	bashCmd.Env = append(os.Environ(), "CG_ATTEST_DIR="+bashAttDir)
	if out, err := bashCmd.CombinedOutput(); err != nil {
		t.Fatalf("bash runner failed (exit): %v\n%s", err, out)
	}
	bashBytes, err := os.ReadFile(filepath.Join(bashAttDir, "attestation.json"))
	if err != nil {
		t.Fatalf("read bash attestation: %v", err)
	}

	// ── (2) Go writer → a separate attestation dir ──────────────────────────
	goAttDir := filepath.Join(dir, "go-att")
	o := Options{
		RepoRoot:  dir,
		Reviewers: reviewers,
		Files:     "notes.txt",
		AttestDir: goAttDir,
		Env:       os.Environ(),
		Runner:    sysexec.DefaultRunner,
		Now:       func() time.Time { return time.Now() },
	}
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("Go gate exit=%d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	goBytes, err := os.ReadFile(filepath.Join(goAttDir, "attestation.json"))
	if err != nil {
		t.Fatalf("read go attestation: %v", err)
	}

	// ── Compare. The `ts` line is a wall-clock timestamp that legitimately
	//    differs between the two runs; everything else must be byte-identical. ─
	bashSHA := fieldValue(t, bashBytes, "tree_state_sha")
	goSHA := fieldValue(t, goBytes, "tree_state_sha")
	if bashSHA != goSHA {
		t.Fatalf("tree_state_sha mismatch:\n bash=%s\n go  =%s", bashSHA, goSHA)
	}
	if got, want := normalizeTS(string(goBytes)), normalizeTS(string(bashBytes)); got != want {
		t.Fatalf("attestation bytes differ (ts-normalized):\n--- bash ---\n%s\n--- go ---\n%s", want, got)
	}
	t.Logf("PARITY OK: tree_state_sha=%s (both writers byte-identical, ts-normalized)", goSHA)
	t.Logf("bash attestation:\n%s", bashBytes)
	t.Logf("go   attestation:\n%s", goBytes)
}

// TestDifferentialParity_GoLane_ChecksArray strengthens the parity proof for the
// most format-sensitive field: a populated checks_passed array. It stages a real
// .go change so BOTH writers run the Go lane (gofmt -s / go vet / go test) and
// record ["go:gofmt","go:vet","go:test"], then asserts byte-identical
// attestations (ts-normalized). This is the case where an inline-array spacing
// bug would diverge.
func TestDifferentialParity_GoLane_ChecksArray(t *testing.T) {
	bashBin, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not on PATH")
	}
	if _, err := exec.LookPath("shasum"); err != nil {
		if _, err := exec.LookPath("sha256sum"); err != nil {
			t.Skip("no shasum/sha256sum on PATH")
		}
	}
	runnerScript := findRunnerScript(t)
	// golangci-lint, if present on the host, would make BOTH writers record
	// go:golangci-lint — still identical, so we don't special-case it; but the
	// presence is logged for clarity.

	reviewers := "code-simplifier,go-reviewer"
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	// A minimal, gofmt-clean, vet-clean, test-passing module.
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/parity\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "lib.go"), "package parity\n\n// Add returns a+b.\nfunc Add(a, b int) int { return a + b }\n")
	writeFile(t, filepath.Join(dir, "lib_test.go"), "package parity\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	runGit("add", "go.mod", "lib.go", "lib_test.go")
	runGit("commit", "-q", "-m", "init")
	// Mutate lib.go so git diff HEAD is non-empty and the Go lane has a file.
	writeFile(t, filepath.Join(dir, "lib.go"), "package parity\n\n// Add returns the sum a+b.\nfunc Add(a, b int) int { return a + b }\n")

	// Bash writer.
	bashAttDir := filepath.Join(dir, "bash-att")
	bashCmd := exec.Command(bashBin, runnerScript, "--reviewers", reviewers, "--files", "lib.go")
	bashCmd.Dir = dir
	bashCmd.Env = append(os.Environ(), "CG_ATTEST_DIR="+bashAttDir)
	if out, err := bashCmd.CombinedOutput(); err != nil {
		t.Fatalf("bash runner failed: %v\n%s", err, out)
	}
	bashBytes, err := os.ReadFile(filepath.Join(bashAttDir, "attestation.json"))
	if err != nil {
		t.Fatalf("read bash attestation: %v", err)
	}

	// Go writer.
	goAttDir := filepath.Join(dir, "go-att")
	o := Options{
		RepoRoot:  dir,
		Reviewers: reviewers,
		Files:     "lib.go",
		AttestDir: goAttDir,
		Env:       os.Environ(),
		Runner:    sysexec.DefaultRunner,
		Now:       func() time.Time { return time.Now() },
	}
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("Go gate exit=%d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	goBytes, err := os.ReadFile(filepath.Join(goAttDir, "attestation.json"))
	if err != nil {
		t.Fatalf("read go attestation: %v", err)
	}

	if got, want := normalizeTS(string(goBytes)), normalizeTS(string(bashBytes)); got != want {
		t.Fatalf("attestation bytes differ (ts-normalized):\n--- bash ---\n%s\n--- go ---\n%s", want, got)
	}
	// Sanity: the checks array is actually populated (not the empty-array case).
	if !strings.Contains(string(goBytes), `"go:gofmt"`) {
		t.Fatalf("expected a populated checks_passed array, got:\n%s", goBytes)
	}
	t.Logf("GO-LANE PARITY OK (checks_passed populated, byte-identical):\n%s", goBytes)
}

// findRunnerScript locates commit-gate/commit-gate-runner.sh by walking up from
// the test's working directory to the repo root.
func findRunnerScript(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		cand := filepath.Join(dir, "commit-gate", "commit-gate-runner.sh")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("commit-gate-runner.sh not found above CWD")
		}
		dir = parent
	}
}

// fieldValue extracts the value of a `"key": "value"` line from attestation
// bytes (the writers emit each scalar field on its own line).
func fieldValue(t *testing.T, b []byte, key string) string {
	t.Helper()
	needle := `"` + key + `": "`
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, needle) {
			rest := strings.TrimPrefix(line, needle)
			rest = strings.TrimSuffix(rest, ",")
			return strings.TrimSuffix(rest, `"`)
		}
	}
	t.Fatalf("field %q not found in:\n%s", key, b)
	return ""
}

// normalizeTS replaces the `ts` line's value with a constant so the byte compare
// ignores the wall-clock difference between the two runs (the ONLY legitimately
// non-deterministic field).
func normalizeTS(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), `"ts": "`) {
			out = append(out, `  "ts": "<TS>",`)
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
