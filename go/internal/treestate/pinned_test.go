package treestate

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type hostileRow struct {
	name         string
	config       [][2]string
	env          []string
	subdir       string
	plainDiffers bool
}

func hostileRows(t *testing.T) []hostileRow {
	t.Helper()
	orderFile := writeSideFile(t, "order", "notes.txt\n")
	attrs := writeSideFile(t, "attrs", "notes.txt diff=upper\nmain.go diff=custom\n")
	return []hostileRow{
		{name: "noprefix", config: [][2]string{{"diff.noprefix", "true"}}, plainDiffers: true},
		{name: "mnemonicPrefix", config: [][2]string{{"diff.mnemonicPrefix", "true"}}, plainDiffers: true},
		{name: "srcPrefix", config: [][2]string{{"diff.srcPrefix", "SRC/"}}, plainDiffers: true},
		{name: "dstPrefix", config: [][2]string{{"diff.dstPrefix", "DST/"}}, plainDiffers: true},
		{name: "renamesOff", config: [][2]string{{"diff.renames", "false"}}, plainDiffers: true},
		{name: "algorithmPatience", config: [][2]string{{"diff.algorithm", "patience"}}, plainDiffers: true},
		{name: "indentHeuristicOff", config: [][2]string{{"diff.indentHeuristic", "false"}}, plainDiffers: true},
		{name: "interHunkContext", config: [][2]string{{"diff.interHunkContext", "20"}}, plainDiffers: true},
		{name: "context", config: [][2]string{{"diff.context", "1"}}, plainDiffers: true},
		{name: "externalDiff", config: [][2]string{{"diff.external", "echo"}}, plainDiffers: true},
		{name: "textconv", config: [][2]string{{"core.attributesFile", attrs}, {"diff.upper.textconv", "sed s/o/0/g"}}, plainDiffers: true},
		{name: "funcnameDriver", config: [][2]string{{"core.attributesFile", attrs}, {"diff.custom.xfuncname", "^package.*"}}, plainDiffers: true},
		{name: "color", config: [][2]string{{"color.diff", "always"}}, plainDiffers: true},
		{name: "abbrev12", config: [][2]string{{"core.abbrev", "12"}}, plainDiffers: true},
		{name: "abbrev20", config: [][2]string{{"core.abbrev", "20"}}, plainDiffers: true},
		{name: "orderFile", config: [][2]string{{"diff.orderFile", orderFile}}, plainDiffers: true},
		{name: "suppressBlankEmpty", config: [][2]string{{"diff.suppressBlankEmpty", "true"}}, plainDiffers: true},
		{name: "submoduleLog", config: [][2]string{{"diff.submodule", "log"}}, plainDiffers: true},
		{name: "submoduleDiff", config: [][2]string{{"diff.submodule", "diff"}}, plainDiffers: true},
		{name: "ignoreSubmodulesAll", config: [][2]string{{"diff.ignoreSubmodules", "all"}}, plainDiffers: true},
		{name: "quotePathOff", config: [][2]string{{"core.quotePath", "false"}}, plainDiffers: true},
		{name: "bigFileThreshold", config: [][2]string{{"core.bigFileThreshold", "1k"}}, plainDiffers: true},
		{name: "relativeInSubdir", config: [][2]string{{"diff.relative", "true"}}, subdir: "pkg", plainDiffers: true},
		{name: "compressionNine", config: [][2]string{{"core.compression", "9"}}},
		{name: "gitDiffOptsEnv", env: []string{"GIT_DIFF_OPTS=--unified=1"}, plainDiffers: true},
	}
}

func hostileEnv(extra []string) []string {
	return append(os.Environ(), extra...)
}

func digestIn(t *testing.T, g *gittest.Repo, subdir string, env []string, pinned bool) string {
	t.Helper()
	dir := g.Dir
	if subdir != "" {
		dir = dir + "/" + subdir
	}
	run := sysexec.DefaultRunner
	var got string
	var err error
	if pinned {
		got, err = Pinned(context.Background(), run, dir, env)
	} else {
		got, err = SHA(context.Background(), run, dir, env)
	}
	if err != nil {
		t.Fatalf("digest (pinned=%v) in %s: %v", pinned, dir, err)
	}
	return got
}

func TestPinned_InvariantUnderHostileConfig(t *testing.T) {
	t.Parallel()
	g := matrixFixture(t)
	changeMatrixFixture(t, g)
	want := digestIn(t, g, "", nil, true)
	if want == "" || want == sha256Hex("") {
		t.Fatalf("fixture produced an empty change: %q", want)
	}
	for _, kv := range [][2]string{{"diff.noprefix", "true"}, {"core.abbrev", "12"}} {
		g.Git("config", kv[0], kv[1])
		if got := digestIn(t, g, "", nil, true); got != want {
			t.Errorf("Pinned under %s=%s = %s, want %s", kv[0], kv[1], got, want)
		}
	}
}

func TestPinned_HostileConfigMatrix(t *testing.T) {
	t.Parallel()
	base := matrixFixture(t)
	changeMatrixFixture(t, base)
	wantPinned := digestIn(t, base, "", nil, true)
	wantPlain := digestIn(t, base, "", nil, false)
	for _, row := range hostileRows(t) {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			g := matrixFixture(t)
			changeMatrixFixture(t, g)
			for _, kv := range row.config {
				g.Git("config", kv[0], kv[1])
			}
			env := hostileEnv(row.env)
			if got := digestIn(t, g, row.subdir, env, true); got != wantPinned {
				t.Errorf("Pinned digest moved under %s: got %s, want %s", row.name, got, wantPinned)
			}
			plain := digestIn(t, g, row.subdir, env, false) != wantPlain
			if plain != row.plainDiffers {
				t.Errorf("plain digest differs under %s = %v, want %v: the row does not probe its input", row.name, plain, row.plainDiffers)
			}
		})
	}
}

func TestPinned_BinaryContentChangesTheDigest(t *testing.T) {
	t.Parallel()
	digest := func(content, compression string) string {
		g := gittest.Fixture(t)
		g.Write("bin.dat", "\x00base")
		g.Git("add", ".")
		g.Git("commit", "-q", "-m", "base")
		g.Git("config", "core.compression", compression)
		g.Write("bin.dat", content)
		return digestIn(t, g, "", nil, true)
	}
	one, two := digest("\x00one", "0"), digest("\x00two", "0")
	if one == two {
		t.Fatal("two different binary contents gave one digest")
	}
	if got := digest("\x00one", "9"); got != one {
		t.Errorf("digest moved with core.compression: %s != %s", got, one)
	}
}

func contentInputRows(t *testing.T) []hostileRow {
	t.Helper()
	attrs := writeSideFile(t, "attrs", "* text eol=lf\n")
	return []hostileRow{
		{name: "globalAttributesEolLf", config: [][2]string{{"core.attributesFile", attrs}}},
		{name: "autocrlfTrue", config: [][2]string{{"core.autocrlf", "true"}}},
		{name: "autocrlfInput", config: [][2]string{{"core.autocrlf", "input"}}},
	}
}

func TestPinned_WorktreeChangeEqualsPinnedBetween(t *testing.T) {
	t.Parallel()
	for _, row := range contentInputRows(t) {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			g := baseFixture(t)
			for _, kv := range row.config {
				g.Git("config", kv[0], kv[1])
			}
			parent := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
			g.Write("a.txt", "one\r\nTWO\r\nthree\r\n")
			g.Write("crlf.txt", "x\r\ny\r\n")
			g.Git("add", ".")
			staged := digestIn(t, g, "", nil, true)
			g.Git("commit", "-q", "-m", "change")
			commit := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
			got, err := PinnedBetween(context.Background(), sysexec.DefaultRunner, g.Dir, nil, parent, commit)
			if err != nil {
				t.Fatal(err)
			}
			if got != staged {
				t.Errorf("PinnedBetween(parent, commit) = %s, want the staged digest %s", got, staged)
			}
		})
	}
}

func TestPinned_DirtySubmoduleDoesNotSeparateWorktreeFromCommit(t *testing.T) {
	t.Parallel()
	g := baseFixture(t)
	parent := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	if err := os.MkdirAll(g.Dir+"/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	sub := gittest.At(t, g.Dir+"/sub")
	sub.Write("s.txt", "s\n")
	sub.Git("add", ".")
	sub.Git("commit", "-q", "-m", "s")
	g.Git("add", "sub")
	sub.Write("s.txt", "dirty\n")
	staged := digestIn(t, g, "", nil, true)
	g.Git("commit", "-q", "-m", "gitlink")
	commit := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	got, err := PinnedBetween(context.Background(), sysexec.DefaultRunner, g.Dir, nil, parent, commit)
	if err != nil {
		t.Fatal(err)
	}
	if got != staged {
		t.Errorf("PinnedBetween = %s, want the dirty-submodule worktree digest %s", got, staged)
	}
}

func TestSHA_OutputIsFrozen(t *testing.T) {
	t.Parallel()
	var args []string
	if _, err := SHA(context.Background(), fakeRunner("", 0, nil, &args, nil), "", nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "git diff HEAD" {
		t.Errorf("SHA argv = %q, want the plain git diff HEAD", got)
	}
	if _, err := Between(context.Background(), fakeRunner("", 0, nil, &args, nil), "", nil, "p", "c"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "git diff p c" {
		t.Errorf("Between argv = %q, want the plain git diff p c", got)
	}
}
