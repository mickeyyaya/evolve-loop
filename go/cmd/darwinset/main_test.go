package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_PrintsTheDarwinSetOfTheModuleAndExitsWithItsCode(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":            "module m\n",
		"mac/a_darwin.go":   "package mac\n",
		"plain/a.go":        "package plain\n",
		"plain/a_test.go":   "package plain\n",
		"unix/open_unix.go": "//go:build darwin || linux\n\npackage unix\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
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
	os.Args = []string{"darwinset", "-dir", root}
	os.Stdout = pipeWrite

	main()

	os.Stdout = oldOut
	pipeWrite.Close()
	out, err := io.ReadAll(pipeRead)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || !strings.HasSuffix(string(out), "\n./mac\n./unix\n") || strings.Contains(string(out), "./plain") {
		t.Fatalf("main exited %d with %q; want 0 and the darwin packages mac and unix only", code, out)
	}
}
