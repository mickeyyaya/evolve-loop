//go:build integration

package covchanged

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestWorkspaceCheck_RealGit_RefusesExactlyTheChangedFunctionBelowFull(t *testing.T) {
	repo := gittest.Fixture(t)
	module := filepath.Join(repo.Dir, "go")
	write := func(rel, text string) {
		t.Helper()
		path := filepath.Join(module, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", wsGoMod)
	write("p/a.go", "package p\n\nfunc Changed(x int) int {\n\treturn x\n}\n\nfunc Untouched(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n\nfunc Noted() int {\n\treturn 2\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	write("p/a.go", "package p\n\nfunc Changed(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n\nfunc Untouched(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n\nfunc Noted() int {\n\t// a note only\n\treturn 2\n}\n")
	repo.Git("commit", "-q", "-am", "change one function")
	repo.Git("branch", "ahead", base)
	repo.Git("checkout", "-q", "ahead")
	write("p/a.go", "package p\n\nfunc Changed(x int) int {\n\treturn x\n}\n\nfunc Untouched(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn -1\n}\n\nfunc Noted() int {\n\treturn 2\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "main moved on")
	repo.Git("checkout", "-q", "main")
	write("coverage.txt", "mode: set\n"+
		"example.com/m/p/a.go:3.26,4.11 1 1\n"+
		"example.com/m/p/a.go:4.11,6.3 1 0\n"+
		"example.com/m/p/a.go:7.2,7.10 1 1\n"+
		"example.com/m/p/a.go:10.27,11.11 1 1\n"+
		"example.com/m/p/a.go:11.11,13.3 1 0\n"+
		"example.com/m/p/a.go:14.2,14.10 1 1\n"+
		"example.com/m/p/a.go:17.18,19.10 1 1\n")
	ws := Workspace{ModuleDir: module, Run: sysexec.DefaultRunner}

	v, err := ws.Check(context.Background(), Options{Base: base, Profile: filepath.Join(module, "coverage.txt"), Tags: DefaultTags})

	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if v.Clean() || v.Changed != 1 || !reflect.DeepEqual(keys(v.Refused), []string{"p.Changed"}) {
		t.Fatalf("Check = %+v, want only p.Changed refused", v)
	}
	if ahead, err := ws.Check(context.Background(), Options{Base: "ahead", Profile: filepath.Join(module, "coverage.txt"), Tags: DefaultTags}); err != nil || !reflect.DeepEqual(keys(ahead.Refused), []string{"p.Changed"}) {
		t.Errorf("a base that moved on: Check = %+v, %v; want only p.Changed, not the later commit of the base", ahead, err)
	}
	if got := v.Refused[0]; got.File != "p/a.go" || got.Line != 3 || got.Covered != 2 || got.Statements != 3 {
		t.Errorf("refused %+v, want p/a.go:3 at 2/3", got)
	}
	write("p/b.go", "package p\n\nfunc Fresh() int {\n\treturn 3\n}\n")
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(filepath.Join(module, "coverage.txt"), now, now); err != nil {
		t.Fatal(err)
	}
	if v, err := ws.Check(context.Background(), Options{Base: base, Profile: filepath.Join(module, "coverage.txt"), Tags: DefaultTags}); err != nil || !reflect.DeepEqual(keys(v.Refused), []string{"p.Changed", "p.Fresh"}) {
		t.Errorf("an untracked file: Check = %+v, %v; want p.Changed and p.Fresh refused", v, err)
	}
}
