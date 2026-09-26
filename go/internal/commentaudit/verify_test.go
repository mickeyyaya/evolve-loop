package commentaudit

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
)

func TestVerifyChanges(t *testing.T) {
	before := map[string]string{
		"ok.go":      "package p\n\n// narrative\nfunc a() {}\n",
		"code.go":    "package p\n\nfunc b() int { return 1 }\n",
		"deleted.go": "package p\n",
		"notes.md":   "# anything\n",
	}
	after := map[string]string{
		"ok.go":    "package p\n\nfunc a() {}\n",
		"code.go":  "package p\n\nfunc b() int { return 2 }\n",
		"added.go": "package p\n",
		"notes.md": "# changed prose is not code\n",
	}
	read := func(files map[string]string) func(string) ([]byte, error) {
		return func(p string) ([]byte, error) {
			s, ok := files[p]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return []byte(s), nil
		}
	}
	got, err := VerifyChanges([]string{"ok.go", "code.go", "deleted.go", "added.go", "notes.md"}, read(before), read(after))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code.go: code changed", "deleted.go: file deleted", "added.go: file added"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %q, want %q", got, want)
	}
}

func TestVerifyChanges_AReadFaultIsAnError(t *testing.T) {
	boom := func(string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := VerifyChanges([]string{"x.go"}, boom, boom); err == nil {
		t.Fatal("a read fault must stop the verification, not pass it")
	}
}

func TestAddedNarrative(t *testing.T) {
	before := []byte("package p\n\n// cycle-10 moved here\n// plain\nfunc a() {}\n")
	after := []byte("package p\n\nfunc a() {}\n\n// cycle-10 moved here\n// see incident 2026-09-26 for why\nfunc b() {} // F36 trailing\n")
	got := AddedNarrative(before, after)
	want := []string{"// see incident 2026-09-26 for why"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedNarrative = %q, want %q (a moved line is not added; trailing comments on code lines are out of scope)", got, want)
	}
}
