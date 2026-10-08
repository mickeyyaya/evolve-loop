//go:build acs

package cycle1723

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var hermeticGit = []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0"}

var listedLine = regexp.MustCompile(`^\S+\.go: `)

func buildCommentaudit(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "commentaudit")
	out, errOut, code := run(t, filepath.Join(acsassert.RepoRoot(t), "go"), "go", "build", "-o", bin, "./cmd/commentaudit")
	if code != 0 {
		t.Fatalf("go build ./cmd/commentaudit exited %d:\n%s%s", code, out, errOut)
	}
	return bin
}

func run(t *testing.T, dir, name string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), hermeticGit...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return stdout.String(), stderr.String(), 0
	case errors.As(err, &exitErr):
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	default:
		t.Fatalf("%s %v: %v", name, args, err)
		return "", "", -1
	}
}

func git(t *testing.T, repo string, args ...string) {
	t.Helper()
	full := append([]string{"-C", repo, "-c", "user.email=acs@example.invalid", "-c", "user.name=acs", "-c", "commit.gpgsign=false"}, args...)
	if out, errOut, code := run(t, repo, "git", full...); code != 0 {
		t.Fatalf("git %v exited %d:\n%s%s", args, code, out, errOut)
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func baseRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	writeFiles(t, repo, files)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "base")
	return repo
}

func listed(stdout string) []string {
	var lines []string
	for _, line := range strings.Split(stdout, "\n") {
		if listedLine.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return lines
}

func requireListed(t *testing.T, what, stdout, stderr string, code int, want []string) {
	t.Helper()
	got := listed(stdout)
	if code != 1 || len(got) != len(want) {
		t.Fatalf("%s: want exit 1 listing exactly %d line(s), got exit %d listing %d:\nstdout:\n%s\nstderr:\n%s", what, len(want), code, len(got), stdout, stderr)
	}
	for _, w := range want {
		if !strings.Contains(stdout, w+"\n") {
			t.Errorf("%s: missing listed line %q\nstdout:\n%s", what, w, stdout)
		}
	}
}

func requireNothingListed(t *testing.T, what, stdout, stderr string, code, files int) {
	t.Helper()
	summary := "no comments added in " + strconv.Itoa(files) + " changed Go file(s)"
	if code != 0 || len(listed(stdout)) != 0 || !strings.Contains(stdout, summary) {
		t.Fatalf("%s: want exit 0, no listed line and %q, got exit %d:\nstdout:\n%s\nstderr:\n%s", what, summary, code, stdout, stderr)
	}
}

const plainBase = "package p\n\n// kept is a base comment that stays put.\nfunc a() int {\n\treturn 1\n}\n"

const plainTestBase = "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\tif a() != 1 {\n\t\tt.Fatal(\"a\")\n\t}\n}\n"

const plainAfter = `package p

// kept is a base comment that stays put.
func a() int {
	// plain why: callers hold the lock.
	return 1
}

// See ADR-0100.
func b() int { return 2 }

// Widen doubles n.
func Widen(n int) int {
	// cycle-12 retold history.
	return n * 2
}

/* block note */
func c() {}
`

const plainTestAfter = "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\t// fixture note: one is the smallest valid value.\n\tif a() != 1 {\n\t\tt.Fatal(\"a\")\n\t}\n}\n"

func plainRepo(t *testing.T) string {
	t.Helper()
	repo := baseRepo(t, map[string]string{"p/a.go": plainBase, "p/a_test.go": plainTestBase})
	writeFiles(t, repo, map[string]string{
		"p/a.go":      plainAfter,
		"p/a_test.go": plainTestAfter,
		"p/b.go":      "package p\n\n// helper comment restates the code.\nfunc helper() {}\n",
	})
	return repo
}

func TestC1723_001_CommentsListsEveryAddedCommentLine(t *testing.T) {
	bin := buildCommentaudit(t)
	repo := plainRepo(t)
	stdout, stderr, code := run(t, repo, bin, "comments", "-base", "HEAD")
	requireListed(t, "plain, pointer, exported-doc, narrative, block, test-file and new-file comments", stdout, stderr, code, []string{
		"p/a.go: // plain why: callers hold the lock.",
		"p/a.go: // See ADR-0100.",
		"p/a.go: // Widen doubles n.",
		"p/a.go: // cycle-12 retold history.",
		"p/a.go: /* block note */",
		"p/a_test.go: // fixture note: one is the smallest valid value.",
		"p/b.go: // helper comment restates the code.",
	})
	if strings.Contains(stdout, "kept is a base comment") {
		t.Errorf("a comment already at base is not added by this diff:\n%s", stdout)
	}
}

const directiveBase = "package p\n\nimport \"fmt\"\n\nfunc A() {\n\tfmt.Println(\"a\")\n}\n"

const directiveTestBase = "package p\n\nfunc ExampleA() {\n\tA()\n}\n"

const directiveAfter = `package p

import "fmt"

//go:generate stringer -type=Kind

// Deprecated: use B instead.
func A() {
	//nolint:errcheck
	fmt.Println("a")
}

//go:noinline
//apicover:ignore reason=internal-only
// minimal: no retry until a caller needs one
func B() {
	// plain added line explains nothing.
}

// acs-predicate: config-check
// SSOT IPC-protocol-allowed
var v = 1
`

const directiveTestAfter = "package p\n\nfunc ExampleA() {\n\tA()\n\t// Output: a\n}\n"

var addedDirectives = []string{
	"//go:generate", "// Deprecated:", "//nolint", "//go:noinline", "//apicover:ignore",
	"// minimal:", "// acs-predicate:", "IPC-protocol-allowed", "// Output:",
}

func TestC1723_002_CommentsExcludesMachineReadDirectives(t *testing.T) {
	bin := buildCommentaudit(t)
	repo := baseRepo(t, map[string]string{"p/a.go": directiveBase, "p/a_test.go": directiveTestBase})
	writeFiles(t, repo, map[string]string{"p/a.go": directiveAfter, "p/a_test.go": directiveTestAfter})

	stdout, stderr, code := run(t, repo, bin, "comments", "-base", "HEAD")
	requireListed(t, "directives beside one plain comment", stdout, stderr, code, []string{"p/a.go: // plain added line explains nothing."})
	for _, d := range addedDirectives {
		if strings.Contains(stdout, d) {
			t.Errorf("directive %q is machine-read and must not be listed:\n%s", d, stdout)
		}
	}

	writeFiles(t, repo, map[string]string{"p/a.go": strings.Replace(directiveAfter, "\t// plain added line explains nothing.\n", "", 1)})
	stdout, stderr, code = run(t, repo, bin, "comments", "-base", "HEAD")
	requireNothingListed(t, "a directive-only diff", stdout, stderr, code, 2)
}

func TestC1723_003_CommentsExemptsOnlyANewFilesPackageDoc(t *testing.T) {
	bin := buildCommentaudit(t)
	base := map[string]string{"p/a.go": "package p\n\nfunc a() {}\n"}

	newPkg := baseRepo(t, base)
	writeFiles(t, newPkg, map[string]string{
		"widget/widget.go":       "//go:build !plan9\n\n// Package widget counts the widgets a dashboard shows.\npackage widget\n\nfunc Count() int { return 1 }\n",
		"widget/zz_generated.go": "// Code generated by widgetgen. DO NOT EDIT.\n\npackage widget\n\nconst n = 1\n",
	})
	stdout, stderr, code := run(t, newPkg, bin, "comments", "-base", "HEAD")
	requireNothingListed(t, "a new package's doc, build tag and generated header", stdout, stderr, code, 2)

	docThenCode := baseRepo(t, base)
	writeFiles(t, docThenCode, map[string]string{"w/w.go": "// Package w wraps the widget counter.\npackage w\n\n// count restates its name.\nfunc count() int { return 1 }\n"})
	stdout, stderr, code = run(t, docThenCode, bin, "comments", "-base", "HEAD")
	requireListed(t, "a new file's other comments", stdout, stderr, code, []string{"w/w.go: // count restates its name."})
	if strings.Contains(stdout, "Package w") {
		t.Errorf("a new package's doc is exempt:\n%s", stdout)
	}

	existing := baseRepo(t, map[string]string{
		"p/a.go": "package p\n\nfunc a() {}\n",
		"q/q.go": "// Package q holds the old doc text.\npackage q\n",
		"r/r.go": "// Package r holds the old doc text.\npackage r\n",
		"s/s.go": "// Package s holds the old doc text.\n// It spans five lines at base:\n// alpha,\n// beta,\n// gamma.\npackage s\n",
	})
	writeFiles(t, existing, map[string]string{
		"p/a.go": "// Package p now documents itself here.\npackage p\n\nfunc a() {}\n",
		"q/q.go": "// Package q holds the new doc text.\npackage q\n",
		"r/r.go": "// Package r holds the new doc text.\n// It grows past three lines:\n// one,\n// two.\npackage r\n",
		"s/s.go": "// Package s holds the new doc text.\n// It keeps its five lines:\n// uno,\n// dos,\n// tres.\npackage s\n",
	})
	stdout, stderr, code = run(t, existing, bin, "comments", "-base", "HEAD")
	requireListed(t, "a doc added where none was, and a rewrite past both three lines and its old length", stdout, stderr, code, []string{
		"p/a.go: // Package p now documents itself here.",
		"r/r.go: // Package r holds the new doc text.",
		"r/r.go: // It grows past three lines:",
		"r/r.go: // one,",
		"r/r.go: // two.",
	})
	for _, spared := range []string{"q/q.go", "s/s.go"} {
		if strings.Contains(stdout, spared+": ") {
			t.Errorf("a rewrite of an existing package doc within three lines or its old length is spared, but %s is listed:\n%s", spared, stdout)
		}
	}
}

const movedBase = "package p\n\n// callers hold the lock.\nfunc a() {}\n\nfunc b() {\n\t// the zero value is ready to use.\n\t_ = 0\n}\n"

const movedAfter = "package p\n\nfunc b() {\n\t_ = 0\n}\n\n// callers hold the lock.\nfunc a() {\n\t// the zero value is ready to use.\n}\n"

func TestC1723_004_CommentsDoesNotCountAMovedLineAsAdded(t *testing.T) {
	bin := buildCommentaudit(t)
	repo := baseRepo(t, map[string]string{"p/a.go": movedBase})

	writeFiles(t, repo, map[string]string{"p/a.go": movedAfter})
	stdout, stderr, code := run(t, repo, bin, "comments", "-base", "HEAD")
	requireNothingListed(t, "comments moved within their file", stdout, stderr, code, 1)

	writeFiles(t, repo, map[string]string{"p/a.go": movedAfter + "\n// callers hold the lock.\nfunc c() {}\n"})
	stdout, stderr, code = run(t, repo, bin, "comments", "-base", "HEAD")
	requireListed(t, "a second copy of a base comment", stdout, stderr, code, []string{"p/a.go: // callers hold the lock."})

	writeFiles(t, repo, map[string]string{"p/a.go": "package p\n\nfunc a() {}\n\nfunc b() {\n\t_ = 0\n}\n"})
	stdout, stderr, code = run(t, repo, bin, "comments", "-base", "HEAD")
	requireNothingListed(t, "comments removed", stdout, stderr, code, 1)
}

func TestC1723_005_CommentsExitCodesAndUsage(t *testing.T) {
	bin := buildCommentaudit(t)
	repo := baseRepo(t, map[string]string{"p/a.go": "package p\n\n// kept.\nfunc a() int { return 1 }\n"})
	writeFiles(t, repo, map[string]string{"p/a.go": "package p\n\n// kept.\nfunc a() int { return 2 }\n"})

	stdout, stderr, code := run(t, repo, bin, "comments", "-base", "HEAD")
	requireNothingListed(t, "a code-only diff", stdout, stderr, code, 1)

	for _, args := range [][]string{{"comments"}, {"comments", "-base"}, {"comments", "-base", ""}} {
		stdout, stderr, code = run(t, repo, bin, args...)
		if code != 2 || !strings.Contains(stderr, "usage") || !strings.Contains(stderr, "comments") {
			t.Errorf("%q is a usage error: want exit 2 and a usage line naming comments on stderr, got exit %d:\nstdout:\n%s\nstderr:\n%s", args, code, stdout, stderr)
		}
	}

	stdout, stderr, code = run(t, repo, bin, "comments", "-base", "no-such-ref")
	if code != 1 || strings.TrimSpace(stderr) == "" || strings.Contains(stdout, "no comments added") {
		t.Errorf("an unreadable base must fail loudly with exit 1, got exit %d:\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestC1723_006_CommentsScopesToDirs(t *testing.T) {
	bin := buildCommentaudit(t)
	repo := baseRepo(t, map[string]string{"a/a.go": "package a\n\nfunc a() {}\n", "b/b.go": "package b\n\nfunc b() {}\n"})
	writeFiles(t, repo, map[string]string{
		"a/a.go": "package a\n\n// added in a.\nfunc a() {}\n",
		"b/b.go": "package b\n\n// added in b.\nfunc b() {}\n",
	})

	stdout, stderr, code := run(t, repo, bin, "comments", "-base", "HEAD")
	requireListed(t, "no dir scope", stdout, stderr, code, []string{"a/a.go: // added in a.", "b/b.go: // added in b."})

	stdout, stderr, code = run(t, repo, bin, "comments", "-base", "HEAD", "a")
	requireListed(t, "scoped to a", stdout, stderr, code, []string{"a/a.go: // added in a."})

	stdout, stderr, code = run(t, repo, bin, "comments", "-base", "HEAD", "nowhere")
	if code != 1 || !strings.Contains(stderr, "no changed Go files under") {
		t.Errorf("a scope matching no changed Go file must fail, got exit %d:\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestC1723_007_CheckKeepsItsNarrativeOnlyContract(t *testing.T) {
	bin := buildCommentaudit(t)
	repo := plainRepo(t)
	stdout, stderr, code := run(t, repo, bin, "check", "-base", "HEAD")
	requireListed(t, "check over the same diff", stdout, stderr, code, []string{"p/a.go: // cycle-12 retold history."})
}

func TestC1723_008_CommentauditPackagesStayGreenAndCovered(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, pkg := range []string{"./internal/commentaudit", "./cmd/commentaudit"} {
		if out, errOut, code := run(t, goDir, "go", "vet", pkg); code != 0 {
			t.Errorf("go vet %s exited %d:\n%s%s", pkg, code, out, errOut)
		}
		if out, errOut, code := run(t, goDir, "gofmt", "-l", strings.TrimPrefix(pkg, "./")); code != 0 || strings.TrimSpace(out) != "" {
			t.Errorf("gofmt -l %s exited %d, unformatted: %q %s", pkg, code, out, errOut)
		}
		if out, errOut, code := run(t, goDir, "go", "test", "-count=1", pkg); code != 0 {
			t.Errorf("go test %s exited %d:\n%s%s", pkg, code, out, errOut)
		}
	}

	out, errOut, code := run(t, goDir, "go", "test", "-count=1", "-v", "-run", "Comments", "./internal/commentaudit")
	if code != 0 || !strings.Contains(out, "--- PASS: ") {
		t.Errorf("./internal/commentaudit needs durable unit tests for the comments subcommand (go test -run Comments), exit %d:\n%s%s", code, out, errOut)
	}

	tmp := t.TempDir()
	profile, funcs, apicover := filepath.Join(tmp, "cover.txt"), filepath.Join(tmp, "func.txt"), filepath.Join(tmp, "apicover")
	steps := [][]string{
		{"go", "test", "-count=1", "-tags", "integration", "-coverprofile=" + profile, "./internal/commentaudit"},
		{"go", "tool", "cover", "-func=" + profile, "-o", funcs},
		{"go", "build", "-o", apicover, "./cmd/apicover"},
		{apicover, "-enforce", "-cover", funcs, filepath.Join(goDir, "internal", "commentaudit")},
	}
	for _, s := range steps {
		if out, errOut, code := run(t, goDir, s[0], s[1:]...); code != 0 {
			t.Fatalf("%v exited %d — every export of the enrolled package must be named and executed by a test:\n%s%s", s, code, out, errOut)
		}
	}
}
