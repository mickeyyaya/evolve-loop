package commentaudit

import (
	"bytes"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func readerOf(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		body, ok := files[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return []byte(body), nil
	}
}

func TestAddedAcrossDiff_ACommentMovedIntoAnotherFileIsNotAdded(t *testing.T) {
	before := map[string]string{"a.go": "package p\n\n// keeps the lease across resume.\nfunc a() {}\n\nfunc b() {}\n"}
	after := map[string]string{
		"a.go":     "package p\n\nfunc b() {}\n",
		"lease.go": "package p\n\n// keeps the lease across resume.\nfunc a() {}\n\n// restates the helper.\nfunc h() {}\n",
		"notes.md": "// not Go\n",
	}

	got, err := AddedAcrossDiff([]string{"a.go", "lease.go", "notes.md"}, readerOf(before), readerOf(after))

	want := []Added{{File: "lease.go", Line: "// restates the helper."}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedAcrossDiff = %v, %v; want %v", got, err, want)
	}
}

func TestAddedAcrossDiff_AMoveCoversOnlyAsManyCopiesAsTheDiffRemoved(t *testing.T) {
	before := map[string]string{"a.go": "package p\n\n// shared.\nfunc a() {}\n"}
	after := map[string]string{
		"a.go": "package p\n\nfunc a() {}\n",
		"b.go": "package p\n\n// shared.\nfunc b() {}\n\n// shared.\nfunc c() {}\n",
	}

	got, err := AddedAcrossDiff([]string{"a.go", "b.go"}, readerOf(before), readerOf(after))

	if want := []Added{{File: "b.go", Line: "// shared."}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedAcrossDiff = %v, %v; want %v", got, err, want)
	}
}

func TestMain_CommentsTreatsAMoveAcrossFilesAsMoved(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.go":     "package p\n\nfunc b() {}\n",
		"lease.go": "package p\n\n// keeps the lease across resume.\nfunc a() {}\n",
	})
	git := fakeGit{changed: []string{"a.go", "lease.go"}, base: map[string]string{"a.go": "package p\n\n// keeps the lease across resume.\nfunc a() {}\n\nfunc b() {}\n"}, root: root}
	var out, errOut bytes.Buffer

	code := Main([]string{"comments", "-base", "origin/main"}, &out, &errOut, git)

	if code != 0 || !strings.Contains(out.String(), "no comments added in 2 changed Go file(s)") {
		t.Fatalf("a comment moved into a new file is not added (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestReadAtBase_AFileTheBaseLacksIsAbsentAndAnyOtherFailureIsAnError(t *testing.T) {
	calls := map[string][]byte{"cat-file -e base:a.go": nil, "show base:a.go": []byte("package p\n"), "cat-file -e base^{commit}": nil}
	run := func(args ...string) ([]byte, error) {
		out, ok := calls[strings.Join(args, " ")]
		if !ok {
			return nil, errors.New("git failed")
		}
		return out, nil
	}
	read := ReadAtBase(run, "base")

	if got, err := read("a.go"); err != nil || string(got) != "package p\n" {
		t.Errorf("read(a.go) = %q, %v; want the base content", got, err)
	}
	if _, err := read("new.go"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("read(new.go) error = %v, want fs.ErrNotExist", err)
	}
	calls["cat-file -e base:broken.go"] = nil
	if _, err := read("broken.go"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("read(broken.go) error = %v, want the show failure", err)
	}
}

func TestAddedAcrossDiff_ACommentAnotherFileKeepsIsNotAMoveSource(t *testing.T) {
	before := map[string]string{"a.go": "package p\n\n// shared.\nfunc a() {}\n"}
	after := map[string]string{
		"a.go": "package p\n\n// shared.\nfunc a() {}\n\nfunc z() {}\n",
		"b.go": "package p\n\n// shared.\nfunc b() {}\n",
	}

	got, err := AddedAcrossDiff([]string{"a.go", "b.go"}, readerOf(before), readerOf(after))

	if want := []Added{{File: "b.go", Line: "// shared."}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedAcrossDiff = %v, %v; want %v", got, err, want)
	}
}

func TestMain_CheckTreatsAMoveAcrossFilesAsMoved(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.go":     "package p\n\nfunc b() {}\n",
		"lease.go": "package p\n\n// cycle 1719 moved the lease here.\nfunc a() {}\n",
	})
	git := fakeGit{changed: []string{"a.go", "lease.go"}, base: map[string]string{"a.go": "package p\n\n// cycle 1719 moved the lease here.\nfunc a() {}\n\nfunc b() {}\n"}, root: root}
	var out, errOut bytes.Buffer

	code := Main([]string{"check", "-base", "origin/main"}, &out, &errOut, git)

	if code != 0 || !strings.Contains(out.String(), "no narrative comments added in 2 changed Go file(s)") {
		t.Fatalf("a history comment moved into a new file is not added (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestReadAtBase_AnUnreadableBaseIsAnErrorNotAbsence(t *testing.T) {
	run := func(args ...string) ([]byte, error) { return nil, errors.New("git failed") }

	if _, err := ReadAtBase(run, "gone")("a.go"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read on an unreadable base = %v, want an error that is not absence", err)
	}
}
