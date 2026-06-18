package commitgate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// goldenAttestation is the byte-exact layout the ship-gate reader
// (go/internal/phases/ship/commitgate.go) consumes: field order, 2-space indent,
// inline space-free arrays, trailing newline. The bash commit-gate-runner.sh was
// deleted in Wave B2 once a differential-parity test proved bash↔Go wrote this
// byte-for-byte; this golden now PINS that contract from Go alone (the `ts` line
// is the only legitimately non-deterministic field, normalized out for compare).
//
//	{
//	  "tree_state_sha": "<64-hex>",
//	  "ts": "<TS>",
//	  "checks_passed": ["go:gofmt","go:vet",<"go:golangci-lint" if present>,"go:test"],
//	  "reviewers_run": ["code-simplifier","go-reviewer"],
//	  "tool": "shasum"|"sha256sum"
//	}
//
// %CHECKS% / %SHA% / %TOOL% are substituted at runtime: golangci-lint is an
// OPTIONAL lane (the Go lane records "go:golangci-lint" only when the binary is on
// PATH), so the checks array is host-dependent and built from exec.LookPath.
const goldenAttestation = `{
  "tree_state_sha": "%SHA%",
  "ts": "<TS>",
  "checks_passed": [%CHECKS%],
  "reviewers_run": ["code-simplifier","go-reviewer"],
  "tool": "%TOOL%"
}
`

// TestGolden_GoPipelineWritesByteExactAttestation is the Go-only successor to the
// (now-deleted) bash↔Go differential-parity test. It runs the REAL Options.Run
// pipeline over a real git repo with a staged .go change, then asserts the
// on-disk attestation:
//
//  1. matches the golden byte layout (ts-normalized) — pins the JSON the
//     ship-gate reader parses (field order, indent, inline arrays, trailing nl);
//  2. binds tree_state_sha == sha256(`git diff HEAD`) — the SHA the reader checks
//     against the staged tree — computed here INDEPENDENTLY of the gate.
//
// Together these are the two contracts the deleted bash differential proved; this
// keeps both enforced without a bash runner on PATH.
func TestGolden_GoPipelineWritesByteExactAttestation(t *testing.T) {
	t.Parallel()
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
	// A minimal, gofmt-clean, vet-clean, test-passing module so the Go lane
	// records the full ["go:gofmt","go:vet","go:test"] checks array — the
	// format-sensitive populated-array case.
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/golden\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "lib.go"), "package golden\n\n// Add returns a+b.\nfunc Add(a, b int) int { return a + b }\n")
	writeFile(t, filepath.Join(dir, "lib_test.go"), "package golden\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	runGit("add", "go.mod", "lib.go", "lib_test.go")
	runGit("commit", "-q", "-m", "init")
	// Mutate lib.go so `git diff HEAD` is non-empty (an empty change set is
	// ExitFail) and the Go lane has a file to format/vet/test.
	writeFile(t, filepath.Join(dir, "lib.go"), "package golden\n\n// Add returns the sum a+b.\nfunc Add(a, b int) int { return a + b }\n")

	// ── Go writer → attestation on disk. ────────────────────────────────────
	attDir := filepath.Join(dir, "att")
	o := Options{
		RepoRoot:  dir,
		Reviewers: reviewers,
		Files:     "lib.go",
		AttestDir: attDir,
		Env:       os.Environ(),
		Runner:    sysexec.DefaultRunner,
		Now:       func() time.Time { return time.Now() },
	}
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("Go gate exit=%d, want %d (%v)", res.ExitCode, ExitPass, res.Logs)
	}
	gotBytes, err := os.ReadFile(filepath.Join(attDir, "attestation.json"))
	if err != nil {
		t.Fatalf("read attestation: %v", err)
	}

	// ── (1) Byte-layout: compare against the golden, ts-normalized. ─────────
	wantSHA := computeTreeSHA(t, gitBin, dir)
	wantTool := "shasum"
	if _, err := exec.LookPath("shasum"); err != nil {
		wantTool = "sha256sum"
	}
	// golangci-lint is optional; the Go lane records it only when on PATH.
	checks := []string{`"go:gofmt"`, `"go:vet"`}
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		checks = append(checks, `"go:golangci-lint"`)
	}
	checks = append(checks, `"go:test"`)
	want := strings.ReplaceAll(goldenAttestation, "%SHA%", wantSHA)
	want = strings.ReplaceAll(want, "%TOOL%", wantTool)
	want = strings.ReplaceAll(want, "%CHECKS%", strings.Join(checks, ","))
	if got := normalizeTS(string(gotBytes)); got != want {
		t.Fatalf("attestation bytes differ from golden (ts-normalized):\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}

	// ── (2) SHA-binding: tree_state_sha == independently computed sha256. ───
	gotSHA := fieldValue(t, gotBytes, "tree_state_sha")
	if gotSHA != wantSHA {
		t.Fatalf("tree_state_sha mismatch:\n gate=%s\n indep=%s", gotSHA, wantSHA)
	}
	// Sanity: the checks array is actually populated (not the empty-array case).
	if !strings.Contains(string(gotBytes), `"go:gofmt"`) {
		t.Fatalf("expected a populated checks_passed array, got:\n%s", gotBytes)
	}
	t.Logf("GOLDEN OK: tree_state_sha=%s, tool=%s, byte-exact layout pinned", gotSHA, wantTool)
}

// computeTreeSHA returns sha256(`git diff HEAD`) over dir, computed independently
// of the gate — the same binding go/internal/phases/ship/audit.go verifies.
func computeTreeSHA(t *testing.T, gitBin, dir string) string {
	t.Helper()
	cmd := exec.Command(gitBin, "diff", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// `git diff` exits 0 with a non-empty diff; a non-zero exit here is fatal.
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() > 1 {
			t.Fatalf("git diff HEAD: %v", err)
		}
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:])
}

// fieldValue extracts the value of a `"key": "value"` line from attestation
// bytes (the writer emits each scalar field on its own line).
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

// normalizeTS replaces the `ts` line's value with "<TS>" so the byte compare
// ignores the wall-clock write time (the ONLY non-deterministic field).
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
