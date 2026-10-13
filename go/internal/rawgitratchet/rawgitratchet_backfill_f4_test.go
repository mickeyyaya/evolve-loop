package rawgitratchet

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTreeF4(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBoundTestFiles_WithoutGitEveryOnDiskTestFileIsBoundAndTheNoteSaysWhy(t *testing.T) {
	root := writeTreeF4(t, map[string]string{
		"a/a_test.go":        "package a\n",
		"a/a.go":             "package a\n",
		"vendor/v/v_test.go": "package v\n",
		"testdata/t_test.go": "package t\n",
		".hidden/h_test.go":  "package h\n",
		"_skip/s_test.go":    "package s\n",
		"b/c/deep_test.go":   "package c\n",
		"top_test.go":        "package top\n",
	})

	files, note, err := BoundTestFiles(root)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/a_test.go", "b/c/deep_test.go", "top_test.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	if !strings.Contains(note, "git cannot list tracked files, binding every on-disk test file") {
		t.Errorf("note = %q", note)
	}
}

func TestBoundTestFiles_NoTestFilesIsAnErrorNotAnEmptyPass(t *testing.T) {
	root := writeTreeF4(t, map[string]string{"a/a.go": "package a\n"})

	files, note, err := BoundTestFiles(root)

	if err == nil || !strings.Contains(err.Error(), "no test files bound") || files != nil {
		t.Fatalf("files=%v err=%v", files, err)
	}
	if note == "" {
		t.Error("the note explaining the git fallback must still be returned with the error")
	}
}

func TestOnDiskTestFiles_WalkFailuresAreWrappedWithTheRoot(t *testing.T) {
	t.Run("root that does not exist", func(t *testing.T) {
		files, err := onDiskTestFiles(filepath.Join(t.TempDir(), "absent"))
		if err == nil || !strings.Contains(err.Error(), "rawgitratchet: walk ") || files != nil {
			t.Fatalf("files=%v err=%v", files, err)
		}
	})
	t.Run("a directory that cannot be listed", func(t *testing.T) {
		root := writeTreeF4(t, map[string]string{"locked/x_test.go": "package x\n"})
		locked := filepath.Join(root, "locked")
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		if _, err := os.ReadDir(locked); err == nil {
			t.Skip("directory permissions are not enforced for this user")
		}
		if files, err := onDiskTestFiles(root); err == nil || files != nil {
			t.Fatalf("an unreadable directory must fail the walk, got files=%v err=%v", files, err)
		}
	})
	t.Run("a path that cannot be made relative", func(t *testing.T) {
		orig := onDiskRel
		t.Cleanup(func() { onDiskRel = orig })
		boom := errors.New("rel failed")
		onDiskRel = func(string, string) (string, error) { return "", boom }
		root := writeTreeF4(t, map[string]string{"a_test.go": "package a\n"})
		if _, err := onDiskTestFiles(root); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestScan_StageFailuresReturnTheNoteAndTheCause(t *testing.T) {
	t.Run("nothing to bind", func(t *testing.T) {
		note, err := Scan(writeTreeF4(t, map[string]string{"a.go": "package a\n"}))
		if err == nil || !strings.Contains(err.Error(), "no test files bound") || note == "" {
			t.Fatalf("note=%q err=%v", note, err)
		}
	})
	t.Run("a bound test file that does not parse", func(t *testing.T) {
		note, err := Scan(writeTreeF4(t, map[string]string{"a/a_test.go": "package a\nfunc ("}))
		if err == nil || !strings.Contains(err.Error(), "a/a_test.go") || note == "" {
			t.Fatalf("note=%q err=%v", note, err)
		}
	})
	t.Run("a clean tree without a baseline file", func(t *testing.T) {
		note, err := Scan(writeTreeF4(t, map[string]string{"a/a_test.go": "package a\n"}))
		if err == nil || note == "" {
			t.Fatalf("note=%q err=%v: a missing baseline must fail loudly", note, err)
		}
	})
}

func TestBoundTestFiles_UnreadableDirectoryWithoutGitFailsWithTheFallbackNote(t *testing.T) {
	root := writeTreeF4(t, map[string]string{"locked/x_test.go": "package x\n", "ok_test.go": "package ok\n"})
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("directory permissions are not enforced for this user")
	}

	files, note, err := BoundTestFiles(root)

	if err == nil || files != nil || !strings.Contains(note, "git cannot list tracked files") {
		t.Fatalf("files=%v note=%q err=%v", files, note, err)
	}
}
