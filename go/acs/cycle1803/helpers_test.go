//go:build acs

package cycle1803

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	binDir    string
	binPath   string
	buildOut  string
	buildErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		os.RemoveAll(binDir)
	}
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		if binDir, buildErr = os.MkdirTemp("", "cycle1803-evolve-"); buildErr != nil {
			return
		}
		binPath = filepath.Join(binDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/evolve")
		cmd.Dir = goDir(t)
		out, err := cmd.CombinedOutput()
		buildOut, buildErr = string(out), err
	})
	if buildErr != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", buildErr, buildOut)
	}
	return binPath
}

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "TMUX") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_TERMINAL_PROMPT=0")
}

type evolveRun struct {
	stdout, stderr string
	code           int
}

func runEvolveIn(t *testing.T, dir string, args ...string) evolveRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir = dir
	cmd.Env = isolatedEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	run := evolveRun{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		run.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %v: %v", args, err)
	}
	return run
}

func runEvolve(t *testing.T, args ...string) evolveRun {
	t.Helper()
	return runEvolveIn(t, t.TempDir(), args...)
}

func projectWithState(t *testing.T, state string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(evolveDir, "state.json"), state)
	return root, evolveDir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type fileStamp struct {
	body    string
	modTime time.Time
}

func stampAgedFile(t *testing.T, path string) fileStamp {
	t.Helper()
	aged := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path, aged, aged); err != nil {
		t.Fatal(err)
	}
	return currentStamp(t, path)
}

func currentStamp(t *testing.T, path string) fileStamp {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileStamp{body: readFile(t, path), modTime: info.ModTime()}
}

type stateArrays struct {
	FailedApproaches []map[string]any `json:"failedApproaches"`
	CarryoverTodos   []map[string]any `json:"carryoverTodos"`
}

func readStateArrays(t *testing.T, evolveDir string) stateArrays {
	t.Helper()
	body := readFile(t, filepath.Join(evolveDir, "state.json"))
	var st stateArrays
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatalf("state.json: %v (%q)", err, body)
	}
	return st
}

func failedCycleSet(st stateArrays) map[int]bool {
	set := map[int]bool{}
	for _, e := range st.FailedApproaches {
		if c, ok := e["cycle"].(float64); ok {
			set[int(c)] = true
		}
	}
	return set
}

func carryoverByID(st stateArrays) map[string]map[string]any {
	byID := map[string]map[string]any{}
	for _, e := range st.CarryoverTodos {
		if id, ok := e["id"].(string); ok {
			byID[id] = e
		}
	}
	return byID
}

func writeTarGz(t *testing.T, path string, names ...string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		body := []byte("untracked " + name + "\n")
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, buf.String())
}

func patchTouching(files ...string) string {
	var b strings.Builder
	for _, f := range files {
		b.WriteString("diff --git a/" + f + " b/" + f + "\n")
		b.WriteString("--- a/" + f + "\n+++ b/" + f + "\n@@ -1 +1,2 @@\n base\n+salvaged edit\n")
	}
	return b.String()
}

type treeEntry struct {
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func treeSnapshot(t *testing.T, dir string) map[string]treeEntry {
	t.Helper()
	snap := map[string]treeEntry{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		snap[rel] = treeEntry{size: info.Size(), mode: info.Mode(), modTime: info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func decodeRows(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("salvage list --json stdout is not a JSON array: %v\n%s", err, stdout)
	}
	return rows
}

func rowByLeaf(rows []map[string]any) map[string]map[string]any {
	byLeaf := map[string]map[string]any{}
	for _, r := range rows {
		if leaf, ok := r["leaf"].(string); ok {
			byLeaf[leaf] = r
		}
	}
	return byLeaf
}

func nonBlankLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
