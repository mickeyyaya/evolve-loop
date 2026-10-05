package commentaudit

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestAddedComments_RewritingAnExistingPackageDocAddsNothing(t *testing.T) {
	before := []byte("// Package p does x for the loop.\npackage p\n")
	after := []byte("// Package p does y for the loop.\npackage p\n")

	if got := AddedComments(before, after); got != nil {
		t.Errorf("AddedComments = %v, want nil: a package doc rewritten in place is still the package doc", got)
	}
	if got := AddedComments([]byte("package p\n"), after); !slices.Equal(got, []string{"// Package p does y for the loop."}) {
		t.Errorf("a package doc added to a file that had none: AddedComments = %v, want it counted", got)
	}
}

func TestAddedComments_ALineInARawStringIsNotAComment(t *testing.T) {
	after := []byte("package p\n\nconst fixture = `\n// not a comment, a fixture line\n/* nor this */\n`\n\nfunc f() {\n\t// a real comment\n}\n")

	if got := AddedComments([]byte("package p\n"), after); !slices.Equal(got, []string{"// a real comment"}) {
		t.Errorf("AddedComments = %v, want only the real comment", got)
	}
}

func TestAddedAcrossDiff_SkipsTestdata(t *testing.T) {
	read := func(src string) func(string) ([]byte, error) {
		return func(string) ([]byte, error) { return []byte(src), nil }
	}
	added, err := AddedAcrossDiff([]string{"go/internal/x/testdata/fixture.go"}, read("package x\n"), read("package x\n\n// an input the test reads\nfunc F() {}\n"))

	if err != nil || len(added) != 0 {
		t.Errorf("AddedAcrossDiff = (%v, %v), want testdata fixtures skipped", added, err)
	}
}

func TestAddedComments_ACommentTrailingARawStringIsNotAWholeLineComment(t *testing.T) {
	after := []byte("package p\n\nvar fixture = `a\nb` // trailing the closing backtick\n")

	if got := AddedComments([]byte("package p\n"), after); got != nil {
		t.Errorf("AddedComments = %v, want nil: the comment trails code on the literal's closing line", got)
	}
}

func TestAddedComments_ARewrittenPackageDocMayNotGrowIntoNarrative(t *testing.T) {
	before := []byte("// Package p does x for the loop.\npackage p\n")
	grown := []byte("// Package p does x for the loop.\n// It began in cycle 12.\n// Then it grew.\n// And grew.\npackage p\n")

	if got := AddedComments(before, grown); len(got) == 0 {
		t.Errorf("a package doc grown past three lines: AddedComments = %v, want the growth counted", got)
	}
}

func TestAddedAcrossDiff_SkipsVendoredCode(t *testing.T) {
	read := func(src string) func(string) ([]byte, error) {
		return func(string) ([]byte, error) { return []byte(src), nil }
	}
	added, err := AddedAcrossDiff([]string{"go/vendor/gopkg.in/yaml.v3/yaml.go"}, read("package yaml\n"), read("package yaml\n\n// third-party documentation\nfunc F() {}\n"))

	if err != nil || len(added) != 0 {
		t.Errorf("AddedAcrossDiff = (%v, %v), want vendored code skipped", added, err)
	}
}

func TestAddedAcrossDiff_SkipsCodeUnderADotDirectoryAsRankDoes(t *testing.T) {
	read := func(src string) func(string) ([]byte, error) {
		return func(string) ([]byte, error) { return []byte(src), nil }
	}
	added, err := AddedAcrossDiff([]string{"go/.cache/gen/c.go"}, read("package c\n"), read("package c\n\n// a tool's cached output\nfunc F() {}\n"))

	if err != nil || len(added) != 0 {
		t.Errorf("AddedAcrossDiff = (%v, %v), want code under a dot directory skipped, as Rank and the go tool skip it", added, err)
	}
}

func TestAddedComments_APackageDocIsSparedUpToThreeLines(t *testing.T) {
	threeLines := "// Package p does x for the loop.\n// It also does y for the loop.\n// And z for the loop.\npackage p\n"
	fourLines := strings.TrimSuffix(threeLines, "package p\n") + "// And w for the loop.\npackage p\n"

	if got := AddedComments(nil, []byte(threeLines)); got != nil {
		t.Errorf("a new file's three-line package doc: AddedComments = %v, want nil", got)
	}
	if got := AddedComments(nil, []byte(fourLines)); len(got) == 0 {
		t.Error("a new file's four-line package doc must be counted")
	}
	if got := AddedComments([]byte("// Package p does x for the loop.\npackage p\n"), []byte(threeLines)); got != nil {
		t.Errorf("a one-line doc rewritten to three lines: AddedComments = %v, want nil", got)
	}
}

func TestAddedComments_AnUnchangedCommentBesideASparedPackageDocIsHeld(t *testing.T) {
	before := []byte("// Package p does x for the loop.\npackage p\n\n// legacy note stays.\nfunc f() {}\n")
	after := []byte("// Package p does x for the loop.\npackage p\n\n// legacy note stays.\nfunc f() { _ = 1 }\n")

	if got := AddedComments(before, after); got != nil {
		t.Errorf("AddedComments = %v, want nil: the legacy note was already there", got)
	}
}

func TestAddedComments_ALineDirectiveHidesNoLaterComment(t *testing.T) {
	before := []byte("package p\n\nfunc f() {}\n")
	after := []byte("package p\n\n//line other.go:100\nfunc f() {}\n\n// real comment after\nfunc g() {}\n")

	if got := AddedComments(before, after); !slices.Contains(got, "// real comment after") {
		t.Errorf("AddedComments = %v, want the comment after the //line directive counted", got)
	}
}

const archivedPredicate = "docs/private/research/archived-2026-09-29/superseded-predicate-packages/cycle1764/predicates_test.go"

func TestAddedAcrossDiff_ACommentArchivedUnderDocsIsARecordNotCode(t *testing.T) {
	after := map[string]string{archivedPredicate: "package cycle1764\n\n// Cycle 1764 pins the retry budget.\nfunc TestPredicate() {}\n"}

	for name, r := range map[string]rule{"comments": commentsRule, "check": narrativeRule} {
		got, err := r.acrossDiff([]string{archivedPredicate}, readerOf(nil), readerOf(after))
		if err != nil || len(got) != 0 {
			t.Errorf("%s: acrossDiff = %v, %v; want nothing added: docs/ holds records, never compiled code", name, got, err)
		}
	}
}

func TestAddedAcrossDiff_ACommentBroughtBackFromDocsIntoCodeIsAdded(t *testing.T) {
	before := map[string]string{archivedPredicate: "package cycle1764\n\n// restates the helper.\nfunc h() {}\n"}
	after := map[string]string{"go/internal/p/p.go": "package p\n\n// restates the helper.\nfunc h() {}\n"}

	got, err := AddedAcrossDiff([]string{archivedPredicate, "go/internal/p/p.go"}, readerOf(before), readerOf(after))

	if want := []Added{{File: "go/internal/p/p.go", Line: "// restates the helper."}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedAcrossDiff = %v, %v; want %v: a record leaving docs/ for code is a comment added to code", got, err, want)
	}
}

func TestAddedAcrossDiff_OnlyTheRootDocsDirectoryIsDocumentation(t *testing.T) {
	const likeDocs = "go/internal/explanationdocs/x.go"
	after := map[string]string{likeDocs: "package explanationdocs\n\n// restates the helper.\nfunc h() {}\n"}

	got, err := AddedAcrossDiff([]string{likeDocs}, readerOf(nil), readerOf(after))

	if want := []Added{{File: likeDocs, Line: "// restates the helper."}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedAcrossDiff = %v, %v; want %v: a package whose name ends in docs is code", got, err, want)
	}
}
