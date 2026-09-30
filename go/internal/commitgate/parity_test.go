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

const goldenAttestation = `{
  "tree_state_sha": "%SHA%",
  "ts": "<TS>",
  "checks_passed": [%CHECKS%],
  "reviewers_run": ["code-simplifier","go-reviewer"],
  "tool": "%TOOL%"
}
`

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
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/golden\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "lib.go"), "package golden\n\n// Add returns a+b.\nfunc Add(a, b int) int { return a + b }\n")
	writeFile(t, filepath.Join(dir, "lib_test.go"), "package golden\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	runGit("add", "go.mod", "lib.go", "lib_test.go")
	runGit("commit", "-q", "-m", "init")
	writeFile(t, filepath.Join(dir, "lib.go"), "package golden\n\n// Add returns the sum a+b.\nfunc Add(a, b int) int { return a + b }\n")

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

	wantSHA := computeTreeSHA(t, gitBin, dir)
	wantTool := "shasum"
	if _, err := exec.LookPath("shasum"); err != nil {
		wantTool = "sha256sum"
	}
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

	gotSHA := fieldValue(t, gotBytes, "tree_state_sha")
	if gotSHA != wantSHA {
		t.Fatalf("tree_state_sha mismatch:\n gate=%s\n indep=%s", gotSHA, wantSHA)
	}
	if !strings.Contains(string(gotBytes), `"go:gofmt"`) {
		t.Fatalf("expected a populated checks_passed array, got:\n%s", gotBytes)
	}
	t.Logf("GOLDEN OK: tree_state_sha=%s, tool=%s, byte-exact layout pinned", gotSHA, wantTool)
}

func computeTreeSHA(t *testing.T, gitBin, dir string) string {
	t.Helper()
	cmd := exec.Command(gitBin, "diff", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() > 1 {
			t.Fatalf("git diff HEAD: %v", err)
		}
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:])
}

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
