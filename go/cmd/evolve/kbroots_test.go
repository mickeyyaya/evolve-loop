package main

import (
	"path/filepath"
	"testing"
)

func TestKBRootsAbs(t *testing.T) {
	root := "/proj"
	got := kbRootsAbs(root)
	if len(got) == 0 {
		t.Fatal("expected at least the default search paths")
	}
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Errorf("resolved root %q is not absolute", p)
		}
	}
	// The default lessons dir must resolve under the project root.
	wantLessons := filepath.Join(root, ".evolve/instincts/lessons")
	found := false
	for _, p := range got {
		if filepath.Clean(p) == wantLessons {
			found = true
		}
	}
	if !found {
		t.Errorf("default roots %v missing the lessons dir %q", got, wantLessons)
	}
}
