package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMain_PrintsThePlanOfTheNamedPackagesAndExitsWithItsCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cover-split"), []byte("./s/st ./s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := -1
	oldExit, oldArgs, oldOut := exitProcess, os.Args, os.Stdout
	t.Cleanup(func() { exitProcess, os.Args, os.Stdout = oldExit, oldArgs, oldOut })
	exitProcess = func(c int) { code = c }
	os.Args = []string{"coversplit", "-dir", dir, "m/a", "m/s"}
	os.Stdout = pipeWrite

	main()

	os.Stdout = oldOut
	pipeWrite.Close()
	out, err := io.ReadAll(pipeRead)
	if err != nil {
		t.Fatal(err)
	}
	if want := "m/a m/s\n-coverpkg=./s,./s/st ./s/st\n"; code != 0 || string(out) != want {
		t.Fatalf("main exited %d with %q; want 0 and %q", code, out, want)
	}
}
