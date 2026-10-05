package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

func TestLoopPreflightOnly_RemovesOnlyTheGateProbeDirItCreated(t *testing.T) {
	cases := map[string]struct {
		preexisting bool
		gateFile    bool
		wantDir     bool
	}{
		"empty probe dir created by the gate is removed": {wantDir: false},
		"a probe dir the gate filled is kept":            {gateFile: true, wantDir: true},
		"a pre-existing worktrees dir is kept":           {preexisting: true, wantDir: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root, evolveDir := preflightOnlyProject(t)
			worktrees := filepath.Join(evolveDir, "worktrees")
			if tc.preexisting {
				if err := os.MkdirAll(worktrees, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stubPreflightOnlyGate(t, gateOutcome(gateCheck("host-capabilities", looppreflight.LevelPass)))
			stubbed := runLoopPreflightFn
			runLoopPreflightFn = func(cfg loopConfig, w io.Writer) looppreflight.Result {
				if err := os.MkdirAll(worktrees, 0o755); err != nil {
					t.Fatal(err)
				}
				if tc.gateFile {
					if err := os.WriteFile(filepath.Join(worktrees, "keep"), nil, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				return stubbed(cfg, w)
			}
			var stdout, stderr bytes.Buffer
			if rc := runLoop([]string{"--preflight-only", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
				t.Fatalf("rc=%d want 0\nstderr:\n%s", rc, stderr.String())
			}
			_, err := os.Stat(worktrees)
			if exists := err == nil; exists != tc.wantDir {
				t.Errorf(".evolve/worktrees exists=%v want %v (stderr:\n%s)", exists, tc.wantDir, stderr.String())
			}
		})
	}
}
