package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveUntrackedCopiesOfOrigin_StopsWhenTheDiffFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	git := func(...string) (string, error) { return "", errors.New("diff failed") }
	removeUntrackedCopiesOfOrigin(git, root, "origin/main", &out)
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err != nil {
		t.Fatalf("file must stay when the diff fails: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("no output expected, got %q", out.String())
	}
}

func TestRemoveUntrackedCopiesOfOrigin_KeepsATrackedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	git := func(args ...string) (string, error) {
		if args[0] == "diff" {
			return "a.txt\n", nil
		}
		if args[0] == "show" {
			return "x", nil
		}
		return "", nil
	}
	removeUntrackedCopiesOfOrigin(git, root, "origin/main", &out)
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err != nil {
		t.Fatalf("tracked file must stay: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("no output expected, got %q", out.String())
	}
}
