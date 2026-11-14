package commitgate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const listingPrefix = "git diff --name-only --no-renames -z HEAD"

func listing(paths ...string) scriptRule {
	return scriptRule{matchPrefix: listingPrefix, stdout: strings.Join(paths, "\x00") + "\x00"}
}

func filesOpts(t *testing.T, files string, rules ...scriptRule) (Options, *scriptRunner) {
	t.Helper()
	o := baseOpts(t.TempDir(), "shasum", "go")
	o.Files = files
	sr := &scriptRunner{rules: rules}
	o.Runner = sr.run()
	return o, sr
}

func TestChangedFiles_ACommaListIsRefusedWithTheFormatHint(t *testing.T) {
	t.Parallel()
	o, _ := filesOpts(t, "a.go,b.go", listing("a.go", "b.go"))
	res := &Result{}
	files, code := o.changedFiles(context.Background(), res)
	if code != ExitBadArgs || files != nil {
		t.Fatalf("code = %d files = %v, want ExitBadArgs and no files", code, files)
	}
	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "a.go,b.go") || !strings.Contains(logs, "--files takes a "+FilesFormat) {
		t.Fatalf("logs = %q, want the path and the hint", logs)
	}
	if len(res.Logs) != 1 {
		t.Errorf("logs = %q, want one line for the one refused path", res.Logs)
	}
}

func TestChangedFiles_AMissingPathIsRefusedAndNamed(t *testing.T) {
	t.Parallel()
	o, _ := filesOpts(t, "a.go nowhere.go", listing("a.go"))
	res := &Result{}
	_, code := o.changedFiles(context.Background(), res)
	logs := strings.Join(res.Logs, "\n")
	if code != ExitBadArgs || !strings.Contains(logs, "nowhere.go") || strings.Contains(logs, "--files takes") {
		t.Fatalf("code = %d logs = %q, want ExitBadArgs naming nowhere.go without the hint", code, logs)
	}
}

func TestChangedFiles_AnUnchangedExistingPathIsRefused(t *testing.T) {
	t.Parallel()
	o, _ := filesOpts(t, "same.go", listing("other.go"))
	if err := os.WriteFile(filepath.Join(o.RepoRoot, "same.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &Result{}
	if _, code := o.changedFiles(context.Background(), res); code != ExitBadArgs {
		t.Fatalf("code = %d, want ExitBadArgs (%v)", code, res.Logs)
	}
}

func TestChangedFiles_AnUnstagedDeletionIsAccepted(t *testing.T) {
	t.Parallel()
	o, _ := filesOpts(t, "gone.go", listing("gone.go"))
	files, code := o.changedFiles(context.Background(), &Result{})
	if code != ExitPass || len(files) != 1 || files[0] != "gone.go" {
		t.Fatalf("code = %d files = %v, want the deleted path accepted", code, files)
	}
}

func TestChangedFiles_ALongCommaListIsBadArgsNotGitFatal(t *testing.T) {
	t.Parallel()
	paths := make([]string, 27)
	for i := range paths {
		paths[i] = "pkg/file" + strings.Repeat("x", 20) + string(rune('a'+i%26)) + ".go"
	}
	o, _ := filesOpts(t, strings.Join(paths, ","), listing(paths...))
	res := o.Run(context.Background())
	if res.ExitCode != ExitBadArgs {
		t.Fatalf("ExitCode = %d, want ExitBadArgs (%v)", res.ExitCode, res.Logs)
	}
}

func TestChangedFiles_AListingFailureIsGitFatal(t *testing.T) {
	t.Parallel()
	o, _ := filesOpts(t, "a.go", scriptRule{matchPrefix: listingPrefix, err: errors.New("boom")})
	res := &Result{}
	if _, code := o.changedFiles(context.Background(), res); code != ExitGitFatal {
		t.Fatalf("code = %d, want ExitGitFatal (%v)", code, res.Logs)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "boom") {
		t.Errorf("logs = %q, want the cause of the listing failure", res.Logs)
	}
}

func TestRun_AValidFilesListReachesTheLanesWithThoseExactPaths(t *testing.T) {
	t.Parallel()
	o, sr := filesOpts(t, "a.go  b.go", listing("a.go", "b.go", "c.go"))
	mustWrite(t, filepath.Join(o.RepoRoot, "a.go"), "package x\n")
	mustWrite(t, filepath.Join(o.RepoRoot, "b.go"), "package x\n")
	mustWrite(t, filepath.Join(o.RepoRoot, "c.go"), "package x\n")
	o.Reviewers = "code-simplifier,go-reviewer"
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want ExitPass (%v)", res.ExitCode, res.Logs)
	}
	var formatted []string
	for _, c := range sr.calls {
		if strings.HasPrefix(c, "gofmt -s -l") {
			formatted = append(formatted, filepath.Base(c))
		}
	}
	if strings.Join(formatted, " ") != "a.go b.go" {
		t.Fatalf("gofmt ran over %v, want exactly a.go b.go (%v)", formatted, sr.calls)
	}
}

func TestChangedFiles_EveryUnlistedPathIsNamed(t *testing.T) {
	t.Parallel()
	o, _ := filesOpts(t, "x.go y.go", listing("a.go"))
	res := &Result{}
	if _, code := o.changedFiles(context.Background(), res); code != ExitBadArgs {
		t.Fatalf("code = %d, want ExitBadArgs (%v)", code, res.Logs)
	}
	logs := strings.Join(res.Logs, "\n")
	for _, path := range []string{"x.go", "y.go"} {
		if !strings.Contains(logs, `"`+path+`"`) {
			t.Errorf("logs = %q, want the refused path %s named", logs, path)
		}
	}
}
