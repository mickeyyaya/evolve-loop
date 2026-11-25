package treestate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	sliderOld   = "{\n\n\n\n\n}\n\n\t\ty\n{\n\tx\n}\n\tx\n}\n}\n"
	sliderNew   = "}\n\tx\n\n\n}\n}\n\n\t\ty\n\tx\n\n\n"
	patienceOld = "t4\nt1\nt1\nt4\nt4\nt0\nt0\nt1\nt3\n"
	patienceNew = "t4\nt3\nt4\nt4\nt1\nt0\nt0\nt1\nt2\nt2\n"
	gitlinkOld  = "1111111111111111111111111111111111111111"
	gitlinkNew  = "2222222222222222222222222222222222222222"
)

func numberedLines(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(prefix)
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString("\n")
	}
	return b.String()
}

func matrixFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	g := gittest.Fixture(t)
	g.Write("main.go", "package p\n\nfunc Alpha() {\n"+numberedLines("\ta", 12)+"}\n\nfunc Beta() {\n"+numberedLines("\tb", 12)+"}\n")
	g.Write("notes.txt", "one\n\ntwo\n\nthree\nfour\nfive\nsix\nseven\n")
	g.Write("rename_me.txt", numberedLines("r", 20))
	g.Write("é.txt", "accent\nbase\n")
	g.Write("bin.dat", "\x00\x01\x02base\x00")
	g.Write("big.txt", numberedLines("big line ", 600))
	g.Write("near.txt", "k1\nk2\nk3\nk4\nk5\nk6\nk7\nk8\nk9\nk10\nk11\nk12\nk13\nk14\nk15\n")
	g.Write("pkg/x.txt", "pkg one\npkg two\n")
	g.Write("slider.txt", sliderOld)
	g.Write("patience.txt", patienceOld)
	g.Git("add", ".")
	g.Git("update-index", "--add", "--cacheinfo", "160000,"+gitlinkOld+",sub")
	g.Git("commit", "-q", "-m", "base")
	return g
}

func changeMatrixFixture(t *testing.T, g *gittest.Repo) {
	t.Helper()
	g.Write("main.go", "package p\n\nfunc Alpha() {\n"+numberedLines("\ta", 12)+"\tchanged()\n}\n\nfunc Beta() {\n"+numberedLines("\tb", 12)+"}\n")
	g.Write("notes.txt", "one\n\nTWO\n\nthree\nfour\nfive\nsix\nseven\n")
	g.Git("mv", "rename_me.txt", "renamed.txt")
	g.Write("é.txt", "accent\nchanged\n")
	g.Write("bin.dat", "\x00\x01\x02changed\x00")
	g.Write("big.txt", strings.Replace(numberedLines("big line ", 600), "big line xxx\n", "BIG LINE\n", 1))
	g.Write("near.txt", strings.NewReplacer("k4\n", "K4\n", "k12\n", "K12\n").Replace("k1\nk2\nk3\nk4\nk5\nk6\nk7\nk8\nk9\nk10\nk11\nk12\nk13\nk14\nk15\n"))
	g.Write("pkg/x.txt", "pkg one\npkg changed\n")
	g.Write("slider.txt", sliderNew)
	g.Write("patience.txt", patienceNew)
	g.Git("update-index", "--cacheinfo", "160000,"+gitlinkNew+",sub")
}

func writeSideFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
