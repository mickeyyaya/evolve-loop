//go:build acs

package cycle1865

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func buildEvolve(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	bin := filepath.Join(t.TempDir(), "evolve")
	build := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	build.Dir = goDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}
	return bin
}

func fakeClaudeOnPath(t *testing.T, version string) []string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"claude " + version + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + dir + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
}

func runVersions(t *testing.T, bin string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"doctor", "versions"}, args...)...)
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run: %v", err)
	}
	return out.String(), code
}

func TestC1865_001_DoctorVersionsJSONListsInstalledCLI(t *testing.T) {
	bin := buildEvolve(t)
	out, code := runVersions(t, bin, fakeClaudeOnPath(t, "7.4.2"), "--json")
	if code != 0 {
		t.Fatalf("RED: exit=%d, want 0 (unknown subcommand exits 10)", code)
	}
	var p struct {
		Versions []struct {
			CLI       string `json:"cli"`
			Installed string `json:"installed"`
		} `json:"versions"`
	}
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	for _, r := range p.Versions {
		if r.CLI == "claude" && r.Installed == "7.4.2" {
			return
		}
	}
	t.Errorf("claude 7.4.2 absent from %s", out)
}

func TestC1865_002_DoctorVersionsWithoutLatestExitsZeroAndOmitsLookup(t *testing.T) {
	bin := buildEvolve(t)
	out, code := runVersions(t, bin, fakeClaudeOnPath(t, "7.4.2"))
	if code != 0 {
		t.Fatalf("RED: exit=%d, want 0", code)
	}
	if !strings.Contains(out, "7.4.2") {
		t.Errorf("text output lacks installed version: %q", out)
	}
}

func TestC1865_003_RuntimeReferenceDocumentsDoctorVersions(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, doc, "evolve doctor versions [--latest] [--json]") {
		t.Errorf("RED: verb undocumented")
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", "docs/operations/runtime-reference.md"); code != 0 {
		t.Errorf("RED: runtime-reference.md untracked")
	}
}
