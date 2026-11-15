package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func testScopedFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	files := map[string]string{
		"go/go.mod":      "module example.com/m\n\ngo 1.23\n",
		"go/a/a.go":      "package a\n",
		"go/a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"go/b/b.go":      "package b\n\nimport _ \"example.com/m/a\"\n",
		"go/b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
		"docs/x.md":      "x\n",
	}
	for rel, body := range files {
		full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	repo.Git("checkout", "-q", "-b", "lane")
	return repo
}

func commitScopedFile(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "change")
}

func TestTestScoped_JSONPassAndEmpty(t *testing.T) {
	t.Parallel()
	repo := testScopedFixture(t)
	commitScopedFile(t, repo, "docs/x.md", "changed\n")

	var out, errBuf bytes.Buffer
	code := runTestScoped([]string{"--base", "main", "--root", repo.Dir, "--json"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("empty change: exit %d, want 0; stderr=%s", code, errBuf.String())
	}
	var env testScopedJSON
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v; out=%q", err, out.String())
	}
	if env.Schema != "evolve.test-scoped/1" || !env.Pass || len(env.Selected) != 0 {
		t.Errorf("got %+v, want schema evolve.test-scoped/1, pass true, empty selected", env)
	}

	commitScopedFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	out.Reset()
	errBuf.Reset()
	code = runTestScoped([]string{"--base", "main", "--root", repo.Dir, "--json"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("go change: exit %d, want 0; stderr=%s", code, errBuf.String())
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.Pass || len(env.Selected) != 2 {
		t.Errorf("got %+v, want pass true and 2 selected packages", env)
	}
}

func TestTestScoped_FailingTestExitsOne(t *testing.T) {
	t.Parallel()
	repo := testScopedFixture(t)
	commitScopedFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	commitScopedFile(t, repo, "go/b/b_test.go", "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { t.Fatal(\"fail\") }\n")

	var out, errBuf bytes.Buffer
	code := runTestScoped([]string{"--base", "main", "--root", repo.Dir, "--json"}, nil, &out, &errBuf)
	if code != exitRefused {
		t.Fatalf("exit %d, want %d; stderr=%s", code, exitRefused, errBuf.String())
	}
	var env testScopedJSON
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v; out=%q", err, out.String())
	}
	if env.Pass {
		t.Errorf("pass = true, want false")
	}

	out.Reset()
	errBuf.Reset()
	code = runTestScoped([]string{"--base", "main", "--root", repo.Dir}, nil, &out, &errBuf)
	if code != exitRefused || !strings.Contains(out.String(), "FAIL") {
		t.Errorf("human fail: code=%d out=%q", code, out.String())
	}
}

func TestTestScoped_HumanPassAndEmpty(t *testing.T) {
	t.Parallel()
	repo := testScopedFixture(t)
	var out, errBuf bytes.Buffer
	code := runTestScoped([]string{"--base", "main", "--root", repo.Dir}, nil, &out, &errBuf)
	if code != 0 || !strings.Contains(out.String(), "selected=0") {
		t.Errorf("human empty: code=%d out=%q err=%q", code, out.String(), errBuf.String())
	}

	commitScopedFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	out.Reset()
	code = runTestScoped([]string{"--base", "main", "--root", repo.Dir}, nil, &out, &errBuf)
	if code != 0 || !strings.Contains(out.String(), "pass=true") {
		t.Errorf("human pass: code=%d out=%q err=%q", code, out.String(), errBuf.String())
	}
}

func TestTestScoped_UsageAndIOErrors(t *testing.T) {
	t.Parallel()
	var out, errBuf bytes.Buffer
	if code := runTestScoped([]string{"--no-such-flag"}, nil, &out, &errBuf); code != exitUsage {
		t.Errorf("unknown flag: code=%d, want %d", code, exitUsage)
	}
	errBuf.Reset()
	if code := runTestScoped([]string{"extra"}, nil, &out, &errBuf); code != exitUsage {
		t.Errorf("positional arg: code=%d, want %d", code, exitUsage)
	}
	errBuf.Reset()
	if code := runTestScoped([]string{"--root", t.TempDir(), "--json"}, nil, &out, &errBuf); code != exitIO {
		t.Errorf("non-git root: code=%d, want %d", code, exitIO)
	}
	errBuf.Reset()
	repo := testScopedFixture(t)
	if code := runTestScoped([]string{"--base", "main", "--root", repo.Dir, "--json"}, nil, &failAfterWriter{remaining: 0}, &errBuf); code != exitIO {
		t.Errorf("stdout write error: code=%d, want %d", code, exitIO)
	}
}

type failAfterWriter struct {
	remaining int
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("write failed")
	}
	w.remaining--
	return len(p), nil
}

func TestPrintTestScoped_WriteErrors(t *testing.T) {
	t.Parallel()
	res := changedpkgs.ShadowTestResult{
		Selected: []string{"./a/..."},
		Pass:     false,
		Output:   "FAIL\n",
	}
	for _, remaining := range []int{0, 1, 2} {
		if err := printTestScoped(&failAfterWriter{remaining: remaining}, res, false); err == nil {
			t.Errorf("remaining=%d: expected write error", remaining)
		}
	}
	if err := printTestScoped(&failAfterWriter{remaining: 0}, res, true); err == nil {
		t.Errorf("json: expected write error")
	}
}
