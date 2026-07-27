package core

// hermetic_project_root_test.go — no test in this package may pin a shared,
// machine-global project root. Every RunCycle writes real state under its
// ProjectRoot (.evolve/runs/, worktree bases, archived-polluted moves), so a
// FIXED path shared by every test binary on the host is cross-process mutable
// state: concurrent fleet lanes each running this suite sampled each other's
// writes. Measured live 2026-07-27: the shared root had accumulated 19,521
// run entries, and its pollution signature ("archived polluted workspace…",
// "fatal: not a git repository") is verbatim what failed the suites_stay_green
// meta-predicates of cycles 1107 and 1116 — false FAILs on an idle-host-green
// suite. Per-test t.TempDir() makes each test own its root; this guard keeps
// the class dead.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedTmpRoot is assembled by concatenation so this guard's own source can
// never match itself (the Read-tool/cat-n self-trigger lesson from the
// exhaustion-regex fixtures).
var sharedTmpRoot = `"/tmp/` + `p"`

func TestCoreTests_NeverPinSharedTmpProjectRoot(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(".", name))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if strings.Contains(string(raw), sharedTmpRoot) {
			offenders = append(offenders, name)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("%d test file(s) pin the shared machine-global %s project root: %v — use t.TempDir() so each test owns its root; "+
			"a fixed shared path is cross-process mutable state and produced the false suites_stay_green reds of cycles 1107/1116",
			len(offenders), sharedTmpRoot, offenders)
	}
}
