package landcmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestRunCI_DocsTests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testFile := filepath.Join(root, "go", "internal", "x", "x_test.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module example.test/fx\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("package x\nimport \"testing\"\nfunc TestOne(t *testing.T) { _ = \"docs/a.md\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := Deps{ResolveRoot: func(p string, _ io.Writer) (string, error) { return p, nil }}

	t.Run("help", func(t *testing.T) {
		var out, errb bytes.Buffer
		if rc := RunCI([]string{"docs-tests", "--help"}, nil, &out, &errb, deps); rc != exitOK || !strings.Contains(out.String(), "usage: evolve ci docs-tests") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
		}
	})
	t.Run("selects", func(t *testing.T) {
		var out, errb bytes.Buffer
		rc := RunCI([]string{"docs-tests", "--project-root", root}, strings.NewReader("docs/a.md\x00"), &out, &errb, deps)
		if rc != exitOK || out.String() != "./internal/x\t^TestOne$\n" {
			t.Fatalf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
		}
	})
	t.Run("usage errors", func(t *testing.T) {
		for _, args := range [][]string{{"docs-tests", "--bogus"}, {"docs-tests", "extra"}} {
			var out, errb bytes.Buffer
			if rc := RunCI(args, strings.NewReader("docs/a.md\x00"), &out, &errb, deps); rc != exitUsage || out.Len() != 0 {
				t.Errorf("%v: rc=%d out=%q err=%q", args, rc, out.String(), errb.String())
			}
		}
	})
	t.Run("io errors", func(t *testing.T) {
		var out, errb bytes.Buffer
		if rc := RunCI([]string{"docs-tests", "--project-root", root}, failingReader{}, &out, &errb, deps); rc != exitIO {
			t.Errorf("stdin fail rc=%d", rc)
		}
		if rc := RunCI([]string{"docs-tests", "--project-root", t.TempDir()}, nil, &out, &errb, deps); rc != exitOK || out.Len() != 0 {
			t.Errorf("empty stdin rc=%d out=%q", rc, out.String())
		}
		if rc := RunCI([]string{"docs-tests", "--project-root", t.TempDir()}, strings.NewReader("docs/a.md\x00"), &out, &errb, deps); rc != exitIO {
			t.Errorf("missing go dir rc=%d", rc)
		}
		badDeps := Deps{ResolveRoot: func(string, io.Writer) (string, error) { return "", errors.New("bad root") }}
		if rc := RunCI([]string{"docs-tests"}, strings.NewReader("docs/a.md\x00"), &out, &errb, badDeps); rc != exitIO {
			t.Errorf("resolveRoot fail rc=%d", rc)
		}
		if rc := RunCI([]string{"docs-tests", "--project-root", root}, strings.NewReader("docs/a.md\x00"), &failingWriter{}, &errb, deps); rc != exitIO {
			t.Errorf("stdout write fail rc=%d", rc)
		}
	})
}
