package covchanged

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const wsGoMod = "module example.com/m\n\ngo 1.23\n"

const wsSource = "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"

const wsDiff = "diff --git a/p/a.go b/p/a.go\n--- a/p/a.go\n+++ b/p/a.go\n@@ -4,0 +5 @@\n+\t\treturn 1\n"

const wsProfile = "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 1\nexample.com/m/p/a.go:4.11,6.3 1 0\nexample.com/m/p/a.go:7.2,7.10 1 1\n"

type wsFixture struct {
	dir  string
	exec *fakeGit
}

func newWorkspace(t *testing.T, files map[string]string, scripts map[string]gitReply) wsFixture {
	t.Helper()
	dir := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	profileAfter(t, filepath.Join(dir, "coverage.txt"), time.Hour)
	base := map[string]gitReply{
		"git rev-parse":  {Stdout: "abc\n"},
		"git merge-base": {Stdout: "main\n"},
		"git diff":       {Stdout: wsDiff},
	}
	for k, v := range scripts {
		base[k] = v
	}
	return wsFixture{dir: dir, exec: &fakeGit{replies: base}}
}

func (fx wsFixture) check(t *testing.T) (Verdict, error) {
	t.Helper()
	return fx.checkWith(t, DefaultTags)
}

func (fx wsFixture) checkWith(t *testing.T, tags []string) (Verdict, error) {
	t.Helper()
	ws := Workspace{ModuleDir: fx.dir, Run: fx.exec.Run}
	return ws.Check(context.Background(), Options{Base: "main", Profile: filepath.Join(fx.dir, "coverage.txt"), Tags: tags})
}

func wsFiles(extra map[string]string) map[string]string {
	files := map[string]string{"go.mod": wsGoMod, "p/a.go": wsSource, "coverage.txt": wsProfile}
	for k, v := range extra {
		files[k] = v
	}
	return files
}

func TestWorkspaceCheck_RefusesTheChangedFunctionBelowFullCoverage(t *testing.T) {
	fx := newWorkspace(t, wsFiles(nil), nil)

	v, err := fx.check(t)

	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if v.Clean() || !reflect.DeepEqual(keys(v.Refused), []string{"p.F"}) || v.Refused[0].Covered != 2 || v.Refused[0].Statements != 3 {
		t.Errorf("Check = %+v, want p.F refused at 2/3", v)
	}
	var diffArgs []string
	for _, c := range fx.exec.Calls {
		if len(c.Args) > 0 && c.Args[0] == "diff" {
			diffArgs = c.Args
		}
	}
	want := []string{"diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--relative", "main", "--", "."}
	if !reflect.DeepEqual(diffArgs, want) || len(fx.exec.Calls) == 0 || fx.exec.Calls[0].Dir != fx.dir {
		t.Errorf("diff args %v in calls %+v, want %v in the module dir", diffArgs, fx.exec.Calls, want)
	}
}

func TestWorkspaceCheck_FailsLoudlyOnEveryInputItCannotRead(t *testing.T) {
	for name, tc := range map[string]struct {
		files   map[string]string
		scripts map[string]gitReply
		want    string
	}{
		"a base that is not a commit":        {wsFiles(nil), map[string]gitReply{"git rev-parse": {ExitCode: 128, Stderr: "bad"}}, "base"},
		"no go.mod":                          {map[string]string{"p/a.go": wsSource, "coverage.txt": wsProfile}, nil, "go.mod"},
		"a go.mod without a module":          {wsFiles(map[string]string{"go.mod": "go 1.23\n"}), nil, "module"},
		"a diff that fails":                  {wsFiles(nil), map[string]gitReply{"git diff": {ExitCode: 128, Stderr: "boom"}}, "boom"},
		"a diff that cannot start":           {wsFiles(nil), map[string]gitReply{"git diff": {ExitCode: -1, Err: os.ErrPermission}}, "permission"},
		"a malformed diff":                   {wsFiles(nil), map[string]gitReply{"git diff": {Stdout: "+++ b/a.go\n@@ bad\n"}}, "hunk"},
		"no profile":                         {wsFiles(map[string]string{"coverage.txt": ""}), nil, "mode"},
		"a changed file that is gone":        {wsFiles(nil), map[string]gitReply{"git diff": {Stdout: strings.ReplaceAll(wsDiff, "p/a.go", "p/gone.go")}}, "gone.go"},
		"a profile that is a directory":      {map[string]string{"go.mod": wsGoMod, "p/a.go": wsSource, "coverage.txt/x": ""}, nil, "read profile"},
		"a changed path that is a directory": {wsFiles(map[string]string{"p/dir.go/x": ""}), map[string]gitReply{"git diff": {Stdout: strings.ReplaceAll(wsDiff, "p/a.go", "p/dir.go")}}, "dir.go"},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newWorkspace(t, tc.files, tc.scripts)
			if v, err := fx.check(t); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check = %+v, %v; want an error naming %q", v, err, tc.want)
			}
		})
	}
	missing := newWorkspace(t, wsFiles(nil), nil)
	ws := Workspace{ModuleDir: missing.dir, Run: missing.exec.Run}
	if _, err := ws.Check(context.Background(), Options{Base: "main", Profile: filepath.Join(missing.dir, "none.txt"), Tags: DefaultTags}); err == nil || !strings.Contains(err.Error(), "none.txt") {
		t.Errorf("a missing profile = %v, want an error naming it", err)
	}
}

type gitReply struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

type gitCall struct {
	Dir  string
	Args []string
}

type fakeGit struct {
	replies map[string]gitReply
	Calls   []gitCall
}

func (f *fakeGit) Run(_ context.Context, _, dir string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	f.Calls = append(f.Calls, gitCall{Dir: dir, Args: args})
	reply := f.replies["git "+args[0]]
	_, _ = io.WriteString(stdout, reply.Stdout)
	_, _ = io.WriteString(stderr, reply.Stderr)
	return reply.ExitCode, reply.Err
}

func TestWorkspaceCheck_TheTagsOfTheOptionsSelectTheMeasuredFiles(t *testing.T) {
	tagged := "//go:build integration\n\n" + wsSource
	diff := strings.Replace(wsDiff, "@@ -4,0 +5 @@", "@@ -6,0 +7 @@", 1)
	profile := strings.NewReplacer("3.19,4.11", "5.19,6.11", "4.11,6.3", "6.11,8.3", "7.2,7.10", "9.2,9.10").Replace(wsProfile)
	fx := newWorkspace(t, wsFiles(map[string]string{"p/a.go": tagged, "coverage.txt": profile}), map[string]gitReply{"git diff": {Stdout: diff}})

	tagged0, err0 := fx.checkWith(t, nil)
	v, err := fx.checkWith(t, []string{"integration"})

	if err != nil || len(v.Exempt) != 0 || !reflect.DeepEqual(keys(v.Refused), []string{"p.F"}) {
		t.Errorf("with the integration tag: Check = %+v, %v; want p.F measured and refused", v, err)
	}
	if err0 != nil || tagged0.Changed != 0 || len(tagged0.Exempt) != 1 || tagged0.Exempt[0].Reason != ReasonBuildConstraints {
		t.Errorf("with no tags: Check = %+v, %v; want the file exempt by its build constraints", tagged0, err0)
	}
}

func TestWorkspaceCheck_AnUntrackedFileCountsAsWhollyAdded(t *testing.T) {
	fresh := "package p\n\nfunc G() int {\n\treturn 2\n}\n"
	fx := newWorkspace(t, wsFiles(map[string]string{"p/new.go": fresh}), map[string]gitReply{
		"git diff":     {},
		"git ls-files": {Stdout: "p/new.go\x00coverage.txt\x00"},
	})

	v, err := fx.check(t)

	if err != nil || !reflect.DeepEqual(keys(v.Refused), []string{"p.G"}) || v.Refused[0].Profiled {
		t.Errorf("Check = %+v, %v; want the untracked p.G refused with no profile", v, err)
	}
	var lsArgs []string
	for _, c := range fx.exec.Calls {
		if c.Args[0] == "ls-files" {
			lsArgs = c.Args
		}
	}
	if !reflect.DeepEqual(lsArgs, []string{"ls-files", "-z", "--others", "--exclude-standard", "--", "."}) {
		t.Errorf("ls-files args = %v", lsArgs)
	}
}

func TestWorkspaceCheck_AnUntrackedListingItCannotReadFailsLoudly(t *testing.T) {
	for name, tc := range map[string]struct {
		reply gitReply
		files map[string]string
		want  string
	}{
		"git fails":        {gitReply{ExitCode: 128, Stderr: "no repo"}, wsFiles(nil), "no repo"},
		"the file is gone": {gitReply{Stdout: "p/gone.go\x00"}, wsFiles(nil), "gone.go"},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newWorkspace(t, tc.files, map[string]gitReply{"git ls-files": tc.reply})
			if v, err := fx.check(t); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check = %+v, %v; want an error naming %q", v, err, tc.want)
			}
		})
	}
}

func TestWorkspaceCheck_MeasuresFromTheMergeBaseSoABaseThatMovedOnAddsNothing(t *testing.T) {
	fx := newWorkspace(t, wsFiles(nil), map[string]gitReply{"git merge-base": {Stdout: "fork\n"}})

	if _, err := fx.check(t); err != nil {
		t.Fatalf("Check: %v", err)
	}
	seen := map[string][]string{}
	for _, c := range fx.exec.Calls {
		seen[c.Args[0]] = c.Args
	}
	if got := seen["merge-base"]; !reflect.DeepEqual(got, []string{"merge-base", "main", "HEAD"}) {
		t.Errorf("merge-base args = %v", got)
	}
	if got := seen["diff"]; len(got) < 7 || got[6] != "fork" {
		t.Errorf("diff args = %v, want the merge base fork", got)
	}
	failing := newWorkspace(t, wsFiles(nil), map[string]gitReply{"git merge-base": {ExitCode: 1}})
	if _, err := failing.check(t); err == nil || !strings.Contains(err.Error(), "merge base") {
		t.Errorf("a base with no merge base = %v, want an error naming it", err)
	}
}

func profileAfter(t *testing.T, profile string, by time.Duration) {
	t.Helper()
	at := time.Now().Add(by)
	if err := os.Chtimes(profile, at, at); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestWorkspaceCheck_RefusesAProfileOlderThanAChangedFile(t *testing.T) {
	fx := newWorkspace(t, wsFiles(nil), nil)
	profileAfter(t, filepath.Join(fx.dir, "coverage.txt"), -time.Hour)

	v, err := fx.check(t)

	if err == nil || !strings.Contains(err.Error(), "p/a.go") || !strings.Contains(err.Error(), "make -C go test-integration") {
		t.Errorf("Check = %+v, %v; want an error naming p/a.go and the make target", v, err)
	}
}
