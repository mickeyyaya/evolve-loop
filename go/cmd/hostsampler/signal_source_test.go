//go:build linux || darwin

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMain_TwoSecondRunWritesOneRecordPerSecondAndStopsOnSIGTERM(t *testing.T) {
	out := filepath.Join(t.TempDir(), "s.ndjson")
	oldExit, oldArgs := exitProcess, os.Args
	t.Cleanup(func() { exitProcess, os.Args = oldExit, oldArgs })
	code := -1
	exitProcess = func(c int) { code = c }
	os.Args = []string{"hostsampler", "-out", out}
	go func() {
		time.Sleep(2200 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	main()
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readSamples(t, out)
	if len(got) < 2 {
		t.Fatalf("samples=%d want >=2 in a 2.2 s run", len(got))
	}
	prev, _ := time.Parse(time.RFC3339Nano, got[0].Timestamp)
	for _, s := range got[1:] {
		cur, err := time.Parse(time.RFC3339Nano, s.Timestamp)
		if err != nil || cur.Sub(prev) > 1500*time.Millisecond {
			t.Fatalf("gap too large: %v -> %v err=%v", prev, cur, err)
		}
		prev = cur
	}
}
