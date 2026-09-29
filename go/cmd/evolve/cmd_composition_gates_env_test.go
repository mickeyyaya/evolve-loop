package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunComposedGates_RunsInTheCIEnvAndNamesAFailingGate(t *testing.T) {
	worktree := t.TempDir()
	makefile := "build:\n\t@test -n \"$$HOME\"\n" +
		"test:\n\t@test -z \"$$EVOLVE_LANE_PROBE\"\n" +
		"test-acs-durable:\n\t@true\n" +
		"apicover-enforce:\n\t@echo \"--- FAIL: TestExportedThingIsCovered\"; exit 1\n"
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "go", "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_LANE_PROBE", "lane")
	var log bytes.Buffer

	results := composedGatesTo(&log)(context.Background(), worktree)

	if results["compile"] != "pass" || results["test"] != "pass" || results["acs"] != "pass" {
		t.Errorf("results = %v; a gate run in the lane's env went red on lane state (the 14 declined carries of waves 27-41)", results)
	}
	if results["apicover"] != "fail" || !strings.Contains(log.String(), "apicover") || !strings.Contains(log.String(), "TestExportedThingIsCovered") || !strings.Contains(log.String(), worktree) {
		t.Errorf("results = %v log = %q; a failing gate names itself and its own output", results, log.String())
	}
}

func TestOutputTail_KeepsTheLastLines(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 1; i <= 25; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	tail := newTailWriter(4096)
	fmt.Fprint(tail, strings.Join(lines, "\n")+"\n")

	got := strings.Split(tail.lastLines(20), "\n")

	if len(got) != 20 || got[0] != "line 6" || got[19] != "line 25" {
		t.Errorf("tail = %q; the last 20 lines, the trailing newline trimmed", got)
	}
	small := newTailWriter(8)
	fmt.Fprint(small, "0123456789abcdef")
	if small.lastLines(20) != "89abcdef" {
		t.Errorf("a tail holds at most its byte cap, the newest bytes: %q", small.lastLines(20))
	}
}
