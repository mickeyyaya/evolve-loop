package gatehermetic

import (
	"context"
	"strings"
	"testing"
)

func TestAudit_FindsMtimeGitDirHistoryAndGitWordsInEachBodyOfAPackage(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os"
	"runtime/debug"`, `func A() bool {
	fi, _ := os.Stat(".")
	return fi.ModTime().IsZero()
}

func B() string { return "x/.git/y" }

func C() { _, _ = debug.ReadBuildInfo() }

func D() string { return "HEAD" }

func E() string { return "main" }
`)
	g := loadFixture(t, files, gateRoot("gates", "A"))

	got := Audit(g, modulePath+"/internal/gates", Contract())

	wantClasses(t, got, "mtime", "git-dir", "history", "git-word")
}

func TestAudit_RealRepostateHoldsNoMtimeGitDirHistoryOrGitWord(t *testing.T) {
	t.Parallel()
	g, err := Load(context.Background(), realLoader, Source{ModuleDir: "../..", Roots: []Root{{Package: repostatePath, Func: "TrackedFiles"}}})
	if err != nil {
		t.Fatal(err)
	}

	got := Audit(g, repostatePath, Contract())
	imports := toolchain{g: g, pkg: repostatePath, pi: g.pkgs[repostatePath], rules: Contract()}.imports()

	if len(got) != 0 || len(imports) != 0 {
		t.Errorf("audit = %+v imports = %+v", got, imports)
	}
}

func TestAudit_TheRepostateStubFixtureHoldsAGitWord(t *testing.T) {
	t.Parallel()
	g := loadFixture(t, nil, Root{Package: repostatePath, Func: "Log"})

	got := Audit(g, repostatePath, Contract())

	if len(got) != 1 || got[0].Class != "git-word" || !strings.Contains(got[0].Site, "repostate.go") {
		t.Errorf("audit = %+v", got)
	}
}
