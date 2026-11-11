package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

func TestAmpCycleRetireRejectsBadInputWithoutChangingState(t *testing.T) {
	t.Parallel()
	moduleDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := builtBinaries.Build(gotooltest.Spec{Dir: moduleDir, Pkg: "./cmd/evolve", Name: "evolve"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"missing cycle", []string{"cycle", "retire"}, 10},
		{"non numeric cycle", []string{"cycle", "retire", "--cycle", "abc"}, 10},
		{"unknown cycle in empty project", []string{"cycle", "retire", "--cycle", "99", "--reason", "x"}, 11},
		{"negative cycle", []string{"cycle", "retire", "--cycle", "-5"}, 10},
		{"zero cycle", []string{"cycle", "retire", "--cycle", "0"}, 10},
		{"huge cycle", []string{"cycle", "retire", "--cycle", "99999999999999"}, 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			args := append(append([]string{}, tc.args...), "--project-root", root)
			cmd := exec.Command(bin, args...)
			cmd.Dir = root
			out, runErr := cmd.CombinedOutput()
			code := 0
			var ee *exec.ExitError
			if errors.As(runErr, &ee) {
				code = ee.ExitCode()
			} else if runErr != nil {
				t.Fatalf("run: %v", runErr)
			}
			if code != tc.want {
				t.Fatalf("exit %d, want %d\n%s", code, tc.want, out)
			}
		})
	}
}
