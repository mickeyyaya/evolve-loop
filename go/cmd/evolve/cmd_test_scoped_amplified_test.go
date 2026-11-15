package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTestScoped_CustomBaseRefAndErrorPaths(t *testing.T) {
	t.Parallel()
	repo := testScopedFixture(t)
	repo.Git("branch", "release-v1")
	commitScopedFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	var out, errBuf bytes.Buffer
	code := runTestScoped([]string{"--base", "release-v1", "--root", repo.Dir, "--json"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("custom base: exit %d, want 0; stderr=%s", code, errBuf.String())
	}
	var env testScopedJSON
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v; out=%q", err, out.String())
	}
	if env.Schema != "evolve.test-scoped/1" || !env.Pass || len(env.Selected) != 2 || env.DurationMS < 0 {
		t.Errorf("got %+v, want schema evolve.test-scoped/1, pass true, 2 selected, duration_ms>=0", env)
	}

	out.Reset()
	errBuf.Reset()
	if code := runTestScoped([]string{"--base", "missing-ref", "--root", repo.Dir, "--json"}, nil, &out, &errBuf); code != exitIO {
		t.Errorf("missing base ref: code=%d, want %d; stderr=%s", code, exitIO, errBuf.String())
	}

	fileRoot := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errBuf.Reset()
	if code := runTestScoped([]string{"--root", fileRoot, "--json"}, nil, &out, &errBuf); code != exitIO {
		t.Errorf("file root: code=%d, want %d; stderr=%s", code, exitIO, errBuf.String())
	}
}
