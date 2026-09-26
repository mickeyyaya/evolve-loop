package commentaudit

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var _ Git = fakeGit{}

type fakeGit struct {
	changed []string
	base    map[string]string
	root    string
}

func (g fakeGit) ChangedFiles(string) ([]string, error) { return g.changed, nil }

func (g fakeGit) Root() (string, error) { return g.root, nil }

func (g fakeGit) Show(_, path string) ([]byte, error) {
	s, ok := g.base[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(s), nil
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestMain_Rank(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a/a.go": "package a\n\n// cycle-1 note\nfunc a() {}\n",
		"b/b.go": "package b\n\n// plain\nfunc b() {}\n",
	})
	var out, errOut bytes.Buffer
	if code := Main([]string{"rank", "-n", "1", root}, &out, &errOut, nil); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "a") || strings.Contains(out.String(), "\tb\n") || strings.Count(out.String(), "\n") != 2 {
		t.Fatalf("rank -n 1 prints the header and the top package only:\n%s", out.String())
	}
}

func TestMain_Verify(t *testing.T) {
	root := writeTree(t, map[string]string{
		"ok.go":   "package p\n\nfunc a() {}\n",
		"code.go": "package p\n\nfunc b() int { return 2 }\n",
	})
	git := fakeGit{
		changed: []string{"ok.go", "code.go"},
		base:    map[string]string{"ok.go": "package p\n\n// narrative\nfunc a() {}\n", "code.go": "package p\n\nfunc b() int { return 1 }\n"},
		root:    root,
	}
	var out, errOut bytes.Buffer
	code := Main([]string{"verify", "-base", "origin/main"}, &out, &errOut, git)
	if code != 1 || !strings.Contains(out.String(), "code.go: code changed") || strings.Contains(out.String(), "ok.go") {
		t.Fatalf("verify must fail naming only code.go (exit %d):\n%s%s", code, out.String(), errOut.String())
	}

	git.changed = []string{"ok.go"}
	out.Reset()
	if code := Main([]string{"verify", "-base", "origin/main"}, &out, &errOut, git); code != 0 {
		t.Fatalf("a comment-only change must pass (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestMain_Usage(t *testing.T) {
	var out, errOut bytes.Buffer
	for _, args := range [][]string{nil, {"nope"}, {"verify"}} {
		if code := Main(args, &out, &errOut, fakeGit{}); code != 2 {
			t.Fatalf("Main(%q) = %d, want usage exit 2", args, code)
		}
	}
}

func TestMain_Check(t *testing.T) {
	root := writeTree(t, map[string]string{"a.go": "package p\n\n// cycle-1234 note\nfunc a() {}\n", "new.go": "package p\n\n// F99 story\n"})
	git := fakeGit{changed: []string{"a.go", "new.go"}, base: map[string]string{"a.go": "package p\n\nfunc a() {}\n"}, root: root}
	var out, errOut bytes.Buffer
	code := Main([]string{"check", "-base", "origin/main"}, &out, &errOut, git)
	if code != 1 || !strings.Contains(out.String(), "a.go: // cycle-1234 note") || !strings.Contains(out.String(), "new.go: // F99 story") {
		t.Fatalf("check must name each added narrative line (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
	git.base["a.go"] = "package p\n\n// cycle-1234 note\nfunc a() {}\n"
	git.changed = []string{"a.go"}
	out.Reset()
	if code := Main([]string{"check", "-base", "origin/main"}, &out, &errOut, git); code != 0 {
		t.Fatalf("pre-existing narrative is the workstream's, not this diff's (exit %d):\n%s", code, out.String())
	}
}

func TestMain_VerifyScopedToDirs(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a/ok.go":   "package a\n\nfunc a() {}\n",
		"b/code.go": "package b\n\nfunc b() int { return 2 }\n",
	})
	git := fakeGit{
		changed: []string{"a/ok.go", "b/code.go"},
		base:    map[string]string{"a/ok.go": "package a\n\n// narrative\nfunc a() {}\n", "b/code.go": "package b\n\nfunc b() int { return 1 }\n"},
		root:    root,
	}
	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD", "a"}, &out, &errOut, git); code != 0 {
		t.Fatalf("a batch scoped to a/ must not see b/'s change (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := Main([]string{"verify", "-base", "HEAD", "b"}, &out, &errOut, git); code != 1 {
		t.Fatalf("scoped to b/, the code change must fail (exit %d):\n%s", code, out.String())
	}
}

func TestMain_VerifyReadsTheAfterSideFromTheRepoRoot(t *testing.T) {
	root := writeTree(t, map[string]string{"go/p/a.go": "package p\n\nfunc a() {}\n"})
	git := fakeGit{changed: []string{"go/p/a.go"}, base: map[string]string{"go/p/a.go": "package p\n\n// note\nfunc a() {}\n"}, root: root}
	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD"}, &out, &errOut, git); code != 0 {
		t.Fatalf("the after side must be read from the repo root (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestMain_ADirAtTheRepoRootScopesNothingOut(t *testing.T) {
	root := writeTree(t, map[string]string{"a/ok.go": "package a\n\nfunc a() {}\n"})
	git := fakeGit{changed: []string{"a/ok.go"}, base: map[string]string{"a/ok.go": "package a\n\n// note\nfunc a() {}\n"}, root: root}
	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD", root}, &out, &errOut, git); code != 0 {
		t.Fatalf("the repo root as a dir covers every change (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestMain_VerifyWithNothingToProveFails(t *testing.T) {
	root := writeTree(t, map[string]string{"notes.md": "# notes\n"})
	git := fakeGit{changed: []string{"notes.md"}, root: root}
	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD"}, &out, &errOut, git); code != 1 || !strings.Contains(errOut.String(), "no changed Go files") {
		t.Fatalf("a proof over zero files proves nothing (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestMain_DirsResolveAgainstTheWorkingDirectoryAndMustMatch(t *testing.T) {
	root := writeTree(t, map[string]string{"go/p/a.go": "package p\n\nfunc a() int { return 2 }\n"})
	git := fakeGit{changed: []string{"go/p/a.go"}, base: map[string]string{"go/p/a.go": "package p\n\nfunc a() int { return 1 }\n"}, root: root}
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join(root, "go")); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD", "p"}, &out, &errOut, git); code != 1 || !strings.Contains(out.String(), "go/p/a.go: code changed") {
		t.Fatalf("a cwd-relative dir must reach the code change (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Main([]string{"verify", "-base", "HEAD", "nowhere"}, &out, &errOut, git); code != 1 || !strings.Contains(errOut.String(), "no changed Go files under") {
		t.Fatalf("dirs that match nothing must fail, not pass vacuously (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestMain_EveryScopedDirMustMatch(t *testing.T) {
	root := writeTree(t, map[string]string{"a/ok.go": "package a\n\nfunc a() {}\n"})
	git := fakeGit{changed: []string{"a/ok.go"}, base: map[string]string{"a/ok.go": "package a\n\n// note\nfunc a() {}\n"}, root: root}
	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD", filepath.Join(root, "a"), filepath.Join(root, "typo")}, &out, &errOut, git); code != 1 || !strings.Contains(errOut.String(), "typo") {
		t.Fatalf("a scoped dir with no changed Go file must fail even when another dir matches (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}
