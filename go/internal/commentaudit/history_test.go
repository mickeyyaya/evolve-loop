package commentaudit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const withHistory = "package p\n\nimport \"fmt\"\n\n// Run starts the loop.\n// Cycle 1675 found the retry double-counted; see the 2026-09-14 incident.\nfunc Run() {\n\tfmt.Println(1) // F36: kept for wave 7\n}\n\n// plain restates the helper.\nfunc plain() {}\n"

const stripped = "package p\n\nimport \"fmt\"\n\nfunc Run() {\n\tfmt.Println(1)\n}\n\nfunc plain() {}\n"

func TestRemovedHistory_RecordsEachRemovedHistoryGroupWithWhereItSat(t *testing.T) {
	got, err := RemovedHistoryAcrossDiff([]string{"go/internal/p/p.go"}, readFrom(map[string]string{"go/internal/p/p.go": withHistory}), readFrom(map[string]string{"go/internal/p/p.go": stripped}))

	if err != nil || len(got) != 2 {
		t.Fatalf("RemovedHistoryAcrossDiff = (%+v, %v), want the two history groups", got, err)
	}
	if got[0].File != "go/internal/p/p.go" || got[0].Line != 5 || got[0].Anchor != "func Run() {" ||
		strings.Join(got[0].Lines, "\n") != "// Run starts the loop.\n// Cycle 1675 found the retry double-counted; see the 2026-09-14 incident." {
		t.Errorf("block group = %+v, want the whole group, its line and the declaration below it", got[0])
	}
	if got[1].Line != 8 || got[1].Anchor != "fmt.Println(1)" || strings.Join(got[1].Lines, "\n") != "// F36: kept for wave 7" {
		t.Errorf("trailing group = %+v, want the code on its own line as the anchor", got[1])
	}
}

func TestRemovedHistory_KeptOrMovedHistoryIsNotRecorded(t *testing.T) {
	moved := strings.Replace(stripped, "func plain() {}\n", "// Run starts the loop.\n// Cycle 1675 found the retry double-counted; see the 2026-09-14 incident.\nfunc plain() {}\n", 1)
	other := "package q\n\n// F36: kept for wave 7\nfunc Q() {}\n"

	got, err := RemovedHistoryAcrossDiff(
		[]string{"go/internal/p/p.go", "go/internal/q/q.go"},
		readFrom(map[string]string{"go/internal/p/p.go": withHistory, "go/internal/q/q.go": "package q\n\nfunc Q() {}\n"}),
		readFrom(map[string]string{"go/internal/p/p.go": moved, "go/internal/q/q.go": other}),
	)

	if err != nil || len(got) != 0 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want nothing recorded: each group reappears whole somewhere in the change", got, err)
	}
}

func TestMain_HistoryAppendsASectionPerPackage(t *testing.T) {
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	git := fakeGit{changed: []string{"go/internal/p/p.go"}, base: map[string]string{"go/internal/p/p.go": withHistory}, root: root}
	out := filepath.Join(root, "docs", "history", "code-comments")
	var stdout, stderr bytes.Buffer

	for _, label := range []string{"round 12 (#749)", "round 13"} {
		if code := Main([]string{"history", "-base", "HEAD", "-label", label, "-out", out}, &stdout, &stderr, git); code != 0 {
			t.Fatalf("%s: exit %d: %s", label, code, stderr.String())
		}
	}

	page, err := os.ReadFile(filepath.Join(out, "internal-p.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Comment history: `internal/p`", "## round 12 (#749)", "## round 13", "`go/internal/p/p.go:5`", "func Run() {", "// Cycle 1675 found the retry double-counted"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("the archive page lacks %q:\n%s", want, page)
		}
	}
	if strings.Count(string(page), "# Comment history:") != 1 {
		t.Errorf("a second run appends a section, never a second header:\n%s", page)
	}
	index, err := os.ReadFile(filepath.Join(out, "README.md"))
	if err != nil || !strings.Contains(string(index), "| `internal/p` | 4 | [internal-p.md](internal-p.md) |") {
		t.Errorf("the index lists each page with its entry count, rewritten on every run (%v):\n%s", err, index)
	}
}

func readFrom(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		body, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(body), nil
	}
}

func TestRemovedHistory_AnUntouchedTrailingCommentInATouchedFileIsKept(t *testing.T) {
	before := "package p\n\nimport \"fmt\"\n\nfunc Run() {\n\tfmt.Println(1) // F36: kept for wave 7\n}\n\nfunc other() {}\n"
	after := strings.Replace(before, "func other() {}", "func other() { println(\"changed\") }", 1)

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 0 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want nothing: the trailing comment is unchanged", got, err)
	}
}

func TestRemovedHistory_IdenticalHistoryTextIsAttributedToTheGroupThatWentAway(t *testing.T) {
	before := "package p\n\n// Cycle 100: fix\nfunc First() {}\n\n// Cycle 100: fix\nfunc Second() {}\n"
	after := "package p\n\nfunc First() {}\n\n// Cycle 100: fix\nfunc Second() {}\n"

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 1 || got[0].Anchor != "func First() {}" || got[0].Line != 3 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want only First's comment", got, err)
	}
}

func TestRemovedHistory_AnchorsATrailingCommentAtItsColumnNotTheFirstSlashes(t *testing.T) {
	before := "package p\n\nfunc f() {\n\turl := \"http://x\" // cycle 12: legacy endpoint\n\t_ = url\n}\n"
	after := "package p\n\nfunc f() {\n\turl := \"http://x\"\n\t_ = url\n}\n"

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 1 || got[0].Anchor != `url := "http://x"` {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want the whole statement as the anchor", got, err)
	}
}

func TestRenderHistorySection_AFenceInsideTheTextNeverClosesTheEntry(t *testing.T) {
	section := RenderHistorySection("r", []HistoryEntry{{File: "p.go", Line: 1, Lines: []string{"// cycle 9: see ```go example```"}}})

	if !strings.Contains(section, "````text\n// cycle 9: see ```go example```\n````\n") {
		t.Errorf("the fence must be longer than any backtick run in the text:\n%s", section)
	}
}

func TestRemovedHistory_ADeletedFileRecordsItsHistory(t *testing.T) {
	got, err := RemovedHistoryAcrossDiff([]string{"go/internal/p/p.go"}, readFrom(map[string]string{"go/internal/p/p.go": withHistory}), readFrom(nil))

	if err != nil || len(got) != 2 || got[0].File != "go/internal/p/p.go" {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want both groups of the deleted file", got, err)
	}
}

func TestRemovedHistory_SkipsWhatIsNotProjectCode(t *testing.T) {
	files := []string{"go/p/testdata/x.go", "go/vendor/v/v.go", "go/.cache/c.go"}
	before := map[string]string{}
	for _, f := range files {
		before[f] = withHistory
	}

	got, err := RemovedHistoryAcrossDiff(files, readFrom(before), readFrom(nil))

	if err != nil || len(got) != 0 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want nothing: testdata, vendor and dot directories are skipped as Rank skips them", got, err)
	}
}

func TestRemovedHistory_OtherHistoryAddedElsewhereExcusesNothing(t *testing.T) {
	got, err := RemovedHistoryAcrossDiff(
		[]string{"p.go", "q.go"},
		readFrom(map[string]string{"p.go": "package p\n\n// Cycle 1: a\nfunc P() {}\n", "q.go": "package q\n\nfunc Q() {}\n"}),
		readFrom(map[string]string{"p.go": "package p\n\nfunc P() {}\n", "q.go": "package q\n\n// Cycle 2: b\nfunc Q() {}\n"}),
	)

	if err != nil || len(got) != 1 || got[0].File != "p.go" {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want p.go's group: a different history text is not a move", got, err)
	}
}

func TestRemovedHistory_AMoveWithinAFileNeverExcusesARemovalInAnother(t *testing.T) {
	got, err := RemovedHistoryAcrossDiff(
		[]string{"a.go", "b.go"},
		readFrom(map[string]string{"a.go": "package a\n\n// Cycle 1: a\nfunc A() {}\n", "b.go": "package b\n\n// Cycle 1: a\nfunc X() {}\n\nfunc Y() {}\n"}),
		readFrom(map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package b\n\nfunc X() {}\n\n// Cycle 1: a\nfunc Y() {}\n"}),
	)

	if err != nil || len(got) != 1 || got[0].File != "a.go" {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want a.go's removal: b.go only moved its group", got, err)
	}
}

func TestRemovedHistory_ARewordedGroupIsRecordedAsItWas(t *testing.T) {
	before := "package p\n\n// Cycle 5: mirrors the legacy inbox-mover.sh exit codes.\n// The pre-cutover bash path.\nfunc P() {}\n"
	after := "package p\n\n// Cycle 5: mirrors the legacy inbox-mover.sh exit codes.\nfunc P() {}\n"

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 1 || len(got[0].Lines) != 2 || got[0].Lines[1] != "// The pre-cutover bash path." {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want the whole old group: its story line went away", got, err)
	}
}

func TestRemovedHistory_ClipsALongAnchor(t *testing.T) {
	name := strings.Repeat("x", 130)
	before := "package p\n\n// Cycle 1: a\nfunc " + name + "() {}\n"
	after := "package p\n\nfunc " + name + "() {}\n"

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 1 || []rune(got[0].Anchor)[maxAnchorRunes] != '…' || len([]rune(got[0].Anchor)) != maxAnchorRunes+1 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want the anchor clipped to %d runes and an ellipsis", got, err, maxAnchorRunes)
	}
}

func TestRemovedHistory_KeepsAnAnchorOfExactlyTheLimitWhole(t *testing.T) {
	decl := "func " + strings.Repeat("x", maxAnchorRunes-10) + "() {}"
	before := "package p\n\n// Cycle 1: a\n" + decl + "\n"
	after := "package p\n\n" + decl + "\n"

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 1 || got[0].Anchor != decl {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want an anchor of exactly %d runes kept whole, with no ellipsis", got, err, maxAnchorRunes)
	}
}

func TestRemovedHistory_WithinAFileOneRewrittenCopyExcusesOneRemoval(t *testing.T) {
	before := "package p\n\n// Cycle 1: x\nfunc A() {}\n\n// Cycle 1: x\nfunc B() {}\n\nfunc C() {}\n"
	after := "package p\n\nfunc A() {}\n\nfunc B() {}\n\n// Cycle 1: x\nfunc C() {}\n"

	got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

	if err != nil || len(got) != 1 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want one entry: one rewritten copy excuses one of the two removed copies", got, err)
	}
}

func TestMain_HistoryWithoutOutPrintsTheSectionAndWritesNothing(t *testing.T) {
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	git := fakeGit{changed: []string{"go/internal/p/p.go"}, base: map[string]string{"go/internal/p/p.go": withHistory}, root: root}
	var stdout, stderr bytes.Buffer

	code := Main([]string{"history", "-base", "HEAD", "-label", "round 12"}, &stdout, &stderr, git)

	if code != 0 || !strings.Contains(stdout.String(), "## round 12") || !strings.Contains(stdout.String(), "// F36: kept for wave 7") {
		t.Fatalf("exit %d, want the section on stdout:\n%s%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
		t.Errorf("a run without -out wrote into the tree (%v)", err)
	}
}

func TestMain_HistoryWithNothingRemovedSaysSo(t *testing.T) {
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	git := fakeGit{changed: []string{"go/internal/p/p.go"}, base: map[string]string{"go/internal/p/p.go": stripped}, root: root}
	var stdout, stderr bytes.Buffer

	code := Main([]string{"history", "-base", "HEAD"}, &stdout, &stderr, git)

	if code != 0 || stdout.String() != "no history-carrying comment removed in 1 changed Go file(s)\n" {
		t.Errorf("exit %d, want the nothing-removed line:\n%s%s", code, stdout.String(), stderr.String())
	}
}

func TestMain_HistoryRecordsAChangeOnce(t *testing.T) {
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	git := fakeGit{changed: []string{"go/internal/p/p.go"}, base: map[string]string{"go/internal/p/p.go": withHistory}, root: root}
	out := filepath.Join(root, "docs", "history", "code-comments")
	args := []string{"history", "-base", "HEAD", "-label", "round 12", "-out", out}
	var stdout, stderr bytes.Buffer
	if code := Main(args, &stdout, &stderr, git); code != 0 {
		t.Fatalf("first run: exit %d: %s", code, stderr.String())
	}
	first, _ := os.ReadFile(filepath.Join(out, "internal-p.md"))

	code := Main(args, &stdout, &stderr, git)

	again, _ := os.ReadFile(filepath.Join(out, "internal-p.md"))
	if code != 1 || !strings.Contains(stderr.String(), `already has a section "round 12"`) || string(again) != string(first) {
		t.Errorf("a second run with the same label must refuse and leave the page as it was (exit %d):\n%s", code, stderr.String())
	}
}

func TestRemovedHistory_OneMoveExcusesOneRemoval(t *testing.T) {
	group := "// Cycle 1: x\n"
	got, err := RemovedHistoryAcrossDiff(
		[]string{"a.go", "b.go", "c.go"},
		readFrom(map[string]string{"a.go": "package a\n\n" + group + "func A() {}\n", "b.go": "package b\n\n" + group + "func B() {}\n", "c.go": "package c\n"}),
		readFrom(map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package b\n\nfunc B() {}\n", "c.go": "package c\n\n" + group + "func C() {}\n"}),
	)

	if err != nil || len(got) != 1 {
		t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want one entry: one written copy excuses one of the two removals", got, err)
	}
}

func TestRemovedHistory_AnchorsOnTheCodeBelowSkippingOtherComments(t *testing.T) {
	cases := map[string]string{
		"package p\n\n// Cycle 1: x\n\n// Foo does.\nfunc Foo() {}\n":              "func Foo() {}",
		"package p\n\n// Cycle 1: x\nconst Timeout = 3 // seconds\n":               "const Timeout = 3",
		"package p\n\n// Cycle 1: x\n\n/* a\n   b */\nvar v = 1\n":                 "var v = 1",
		"package p\n\nfunc f() {\n\t// Cycle 1: x\n\n\t// other\n\tprintln()\n}\n": "println()",
		"package p\n\n// Cycle 1: x\nconst Retries = 2 /* tries */\nvar w = 1\n":   "const Retries = 2",
	}
	for before, want := range cases {
		after := strings.Replace(before, "// Cycle 1: x\n", "", 1)

		got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

		if err != nil || len(got) != 1 || got[0].Anchor != want {
			t.Errorf("for %q: RemovedHistoryAcrossDiff = (%+v, %v), want the anchor %q", before, got, err, want)
		}
	}
}

func TestWriteHistoryArchive_ALabelThatPrefixesAnotherIsItsOwn(t *testing.T) {
	dir := t.TempDir()
	entries := []HistoryEntry{{File: "go/internal/p/p.go", Line: 1, Lines: []string{"// Cycle 1: x"}}}
	if _, err := writeHistoryArchive(dir, "round 12", entries); err != nil {
		t.Fatal(err)
	}

	if _, err := writeHistoryArchive(dir, "round 1", entries); err != nil {
		t.Errorf("round 1 is not round 12: %v", err)
	}
}

func TestWriteHistoryIndex_ListsOnlyArchivePages(t *testing.T) {
	dir := t.TempDir()
	entries := []HistoryEntry{{File: "go/internal/p/p.go", Line: 1, Lines: []string{"// Cycle 1: x"}}}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"round 12", "round 13"} {
		if _, err := writeHistoryArchive(dir, label, entries); err != nil {
			t.Fatal(err)
		}
	}

	index, err := os.ReadFile(filepath.Join(dir, "README.md"))

	if err != nil || strings.Contains(string(index), "(README.md)") || strings.Contains(string(index), "notes.md") || !strings.Contains(string(index), "| `internal/p` | 2 |") {
		t.Errorf("the index lists each archive page once, never itself or a stray page (%v):\n%s", err, index)
	}
}

func TestMain_HistoryLabelsAnUnlabelledRunByItsBase(t *testing.T) {
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	git := fakeGit{changed: []string{"go/internal/p/p.go"}, base: map[string]string{"go/internal/p/p.go": withHistory}, root: root}
	var stdout, stderr bytes.Buffer

	code := Main([]string{"history", "-base", "HEAD"}, &stdout, &stderr, git)

	if code != 0 || !strings.Contains(stdout.String(), "\n## removed against HEAD\n") {
		t.Errorf("exit %d, want the section titled by its base:\n%s%s", code, stdout.String(), stderr.String())
	}
}

func TestMain_HistoryResolvesARelativeOutAgainstTheRepoRoot(t *testing.T) {
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	git := fakeGit{changed: []string{"go/internal/p/p.go"}, base: map[string]string{"go/internal/p/p.go": withHistory}, root: root}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	var stdout, stderr bytes.Buffer

	code := Main([]string{"history", "-base", "HEAD", "-label", "round 12", "-out", historyArchiveDir}, &stdout, &stderr, git)

	if _, err := os.Stat(filepath.Join(root, historyArchiveDir, "internal-p.md")); code != 0 || err != nil {
		t.Errorf("exit %d, want the page under the repo root's archive (%v):\n%s", code, err, stderr.String())
	}
}

func TestWriteHistoryArchive_AFileAtTheRootGetsTheRootPage(t *testing.T) {
	for _, file := range []string{"main.go", "go/main.go"} {
		dir := t.TempDir()

		_, err := writeHistoryArchive(dir, "r", []HistoryEntry{{File: file, Line: 1, Lines: []string{"// Cycle 1: x"}}})

		if _, statErr := os.Stat(filepath.Join(dir, "root.md")); err != nil || statErr != nil {
			t.Errorf("%s, outside any package, is recorded on root.md (%v, %v)", file, err, statErr)
		}
	}
}

func TestWriteHistoryArchive_RefusesALabelThatSpansLines(t *testing.T) {
	dir := t.TempDir()

	_, err := writeHistoryArchive(dir, "round 13\n## round 12", []HistoryEntry{{File: "go/internal/p/p.go", Line: 1, Lines: []string{"// Cycle 1: x"}}})

	if err == nil {
		t.Fatal("a label with a newline was accepted; it would split the page's section heading")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the refused label still wrote %d file(s); a refusal must come before any write", len(entries))
	}
}

func TestWriteHistoryArchive_RewritesTheIndexAfterAFailedAppend(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only file mode")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "internal-q.md")
	if err := os.WriteFile(locked, []byte(pageTitlePrefix+"internal/q`\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	entries := []HistoryEntry{
		{File: "go/internal/p/p.go", Line: 1, Lines: []string{"// Cycle 1: x"}},
		{File: "go/internal/q/q.go", Line: 1, Lines: []string{"// Cycle 2: y"}},
	}

	written, err := writeHistoryArchive(dir, "r", entries)

	index, readErr := os.ReadFile(filepath.Join(dir, historyIndex))
	if err == nil || len(written) != 1 || readErr != nil || !strings.Contains(string(index), "| `internal/p` | 1 |") {
		t.Errorf("a failed append reports its error and still indexes the page it wrote (written %v, err %v):\n%s", written, err, index)
	}
}
