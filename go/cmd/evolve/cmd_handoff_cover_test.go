package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type handoffFailingWriter struct{}

func (handoffFailingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestHandoffCoverage_ProjectRootFallsBackToTheWorkingDirectory(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := handoffProjectRoot("")
	if err != nil || got != cwd {
		t.Fatalf("root = %q, %v; want %q", got, err, cwd)
	}
}

func TestHandoffCoverage_AnUnresolvableProjectRootIsAUsageFailure(t *testing.T) {
	prev := handoffAbsPath
	handoffAbsPath = func(string) (string, error) { return "", errors.New("no working directory") }
	t.Cleanup(func() { handoffAbsPath = prev })
	for _, verb := range []string{"check", "show"} {
		args := []string{verb, "--cycle", "1950", "--json"}
		if verb == "check" {
			args = append(args, "--pair", "tdd-build")
		}
		rc, stdout, stderr := runHC(t, args...)
		var got hcResult
		decodeOneObject(t, stdout, &got)
		if rc != 10 || got.Outcome != "usage" || !strings.Contains(stderr, "usage: evolve handoff") {
			t.Errorf("%s: rc=%d result=%+v stderr=%q, want exit 10 and outcome usage", verb, rc, got, stderr)
		}
	}
}

func TestHandoffCoverage_AFailedJSONWriteIsReportedOnStderr(t *testing.T) {
	t.Parallel()
	var stderr strings.Builder
	writeHandoffJSON(handoffFailingWriter{}, &stderr, handoffResult{})
	if !strings.Contains(stderr.String(), "write json: disk full") {
		t.Errorf("stderr = %q, want the write failure", stderr.String())
	}
}

func TestHandoffCoverage_FailureFileProblemsAreIOErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]func(p hcProject){
		"the failures file is not JSON": func(p hcProject) {
			p.write(".evolve/runs/cycle-1950/handoff-tdd-build-failures.json", "{not json")
		},
		"the failures path is a directory": func(p hcProject) {
			if err := os.MkdirAll(filepath.Join(p.runDir(1950), "handoff-tdd-build-failures.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			p := newHCProject(t)
			req := p.cycle1950()
			p.ledger(1950, req, hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk))
			setup(p)
			res := evaluateHandoff(p.root, 1950, "tdd-build")
			if res.Outcome != "error" || res.Exit != handoffExitIO || len(res.Errors) == 0 {
				t.Errorf("result = %+v, want outcome error and exit %d", res, handoffExitIO)
			}
		})
	}
}
