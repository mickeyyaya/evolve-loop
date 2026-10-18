package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_PrintsThePatternOfTheTaggedTestsAndExitsWithItsCode(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cmd", "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"a.go":          "package a\n",
		"e2e_test.go":   "//go:build e2e\n\npackage a\nimport \"testing\"\nfunc TestTagged(t *testing.T) {}\n",
		"plain_test.go": "package a\nimport \"testing\"\nfunc TestPlain(t *testing.T) {}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := -1
	oldExit, oldArgs, oldOut := exitProcess, os.Args, os.Stdout
	t.Cleanup(func() { exitProcess, os.Args, os.Stdout = oldExit, oldArgs, oldOut })
	exitProcess = func(c int) { code = c }
	os.Args = []string{"e2elist", "-dir", root, "-tags", "e2e", "cmd"}
	os.Stdout = pipeWrite

	main()

	os.Stdout = oldOut
	pipeWrite.Close()
	out, err := io.ReadAll(pipeRead)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || strings.TrimSpace(string(out)) != "^(TestTagged)$" {
		t.Fatalf("main exited %d with %q; want 0 and the anchored pattern of the e2e-tagged test only", code, out)
	}
}
