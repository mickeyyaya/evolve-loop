package gotypes_test

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var fixtureIndex string

func TestMain(m *testing.M) {
	os.Exit(runWithFixture(m))
}

func runWithFixture(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "gotypes-fixture-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(tmp)
	repo := filepath.Join(tmp, "repo")
	if err := copyTree(filepath.Join("testdata", "fixture"), filepath.Join(repo, "go")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fixtureIndex = filepath.Join(tmp, "out", "index.gob")
	if err := gotypes.Build(filepath.Join(repo, "go"), fixtureIndex); err != nil {
		fmt.Fprintln(os.Stderr, "build the fixture index:", err)
	}
	return m.Run()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

func fixtureBackend() gotypes.Backend {
	return gotypes.New(fixtureIndex)
}

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func callers(t *testing.T, symbol string, dir codeindex.Direction) []codeindex.Hit {
	t.Helper()
	hits, err := fixtureBackend().Callers(context.Background(), symbol, dir)
	if err != nil {
		t.Fatalf("Callers(%q, %s): %v", symbol, dir, err)
	}
	return hits
}

func replying(stdout string, exit int, err error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		_, _ = io.WriteString(out, stdout)
		return exit, err
	}
}

type place struct {
	file   string
	line   int
	symbol string
	text   string
}

func places(hits []codeindex.Hit) []place {
	out := make([]place, len(hits))
	for i, h := range hits {
		out[i] = place{file: h.File, line: h.Line, symbol: h.Symbol, text: h.Text}
	}
	return out
}
