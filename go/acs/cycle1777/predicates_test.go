//go:build acs

// Package cycle1777 materialises the cycle-1777 acceptance criteria for the
// three independent size-ratchet shrink tasks pinned to this lane:
//
//   - sizeratchet-verifyeval-shellwords: internal/verifyeval.shellWords
//   - sizeratchet-versionbump-run:       internal/versionbump.Run
//   - sizeratchet-llmcalls-aggregate:    internal/llmcalls.Aggregate
//
// Each function is currently listed in go/internal/sizeratchet/offenders.json
// as exceeding sizeratchet.MaxLines (50); the task is a behavior-preserving
// extraction that shrinks it to the limit and drops its offenders.json entry.
//
// Predicate strategy — every predicate exercises a real artifact or the real
// sizeratchet.Walk/LoadOffenders production code path against the live
// worktree source, never a source-grep for a magic string (the cycle-85
// degenerate-predicate ban):
//
//   - 001/004/007 parse the LIVE offenders.json and assert the task's key is
//     absent — the "entry dropped" half of the acceptance bar.
//   - 002/005/008 run the real sizeratchet.Walk over the live go/ tree and
//     assert the task's function span is at or under MaxLines — the "shrunk"
//     half. This is the crux predicate: today it fails because the function is
//     still 51/52/53 lines.
//   - 003/006/009 run the package's own test suite (a single named package,
//     never a repo-wide sweep) so the extraction is caught the moment it
//     changes observable behavior. These pass today (pre-existing GREEN) and
//     stay green as a regression guard through the refactor.
//
// gofmt/go vet cleanliness on the touched files is dispositioned
// manual+checklist in test-report.md (Builder runs and reports; a leaf
// two-command lint check has no meaningful RED/GREEN state before the
// refactor exists to lint).
package cycle1777

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func loadOffendersRaw(t *testing.T, goRoot string) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goRoot, "internal", "sizeratchet", "offenders.json"))
	if err != nil {
		t.Fatalf("read offenders.json: %v", err)
	}
	var offenders map[string]int
	if err := json.Unmarshal(raw, &offenders); err != nil {
		t.Fatalf("parse offenders.json: %v", err)
	}
	return offenders
}

func spanFor(t *testing.T, goRoot, key string) (sizeratchet.FuncSpan, bool) {
	t.Helper()
	spans, err := sizeratchet.Walk(goRoot)
	if err != nil {
		t.Fatalf("sizeratchet.Walk(%s): %v", goRoot, err)
	}
	for _, span := range spans {
		if span.Key == key {
			return span, true
		}
	}
	return sizeratchet.FuncSpan{}, false
}

// runPackageTests runs `go test -count=1` for exactly one named package
// rooted at goRoot, never a repo-wide sweep (Predicate Reliability rule).
func runPackageTests(t *testing.T, goRoot, pkg string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", pkg)
	cmd.Dir = goRoot
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC1777_001_VerifyevalShellWordsOffenderEntryRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	offenders := loadOffendersRaw(t, goRoot)
	if allowance, present := offenders["internal/verifyeval.shellWords"]; present {
		t.Errorf("RED: offenders.json still lists internal/verifyeval.shellWords (allowance %d) — entry must be dropped once shellWords is shrunk to the ratchet limit", allowance)
	}
}

func TestC1777_002_VerifyevalShellWordsWithinRatchetLimit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	span, found := spanFor(t, goRoot, "internal/verifyeval.shellWords")
	if !found {
		t.Fatalf("RED: internal/verifyeval.shellWords not found by sizeratchet.Walk — function renamed or moved")
	}
	if span.Lines > sizeratchet.MaxLines {
		t.Errorf("RED: internal/verifyeval.shellWords is %d lines, want <= %d (sizeratchet.MaxLines) — extract the quote-state-machine step", span.Lines, sizeratchet.MaxLines)
	}
}

func TestC1777_003_VerifyevalShellWordsTestsPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	out, err := runPackageTests(t, goRoot, "./internal/verifyeval")
	if err != nil {
		t.Errorf("go test ./internal/verifyeval/... -count=1 failed (extraction changed observable behavior):\n%s", out)
	}
}

func TestC1777_004_VersionbumpRunOffenderEntryRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	offenders := loadOffendersRaw(t, goRoot)
	if allowance, present := offenders["internal/versionbump.Run"]; present {
		t.Errorf("RED: offenders.json still lists internal/versionbump.Run (allowance %d) — entry must be dropped once Run is shrunk to the ratchet limit", allowance)
	}
}

func TestC1777_005_VersionbumpRunWithinRatchetLimit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	span, found := spanFor(t, goRoot, "internal/versionbump.Run")
	if !found {
		t.Fatalf("RED: internal/versionbump.Run not found by sizeratchet.Walk — function renamed or moved")
	}
	if span.Lines > sizeratchet.MaxLines {
		t.Errorf("RED: internal/versionbump.Run is %d lines, want <= %d (sizeratchet.MaxLines) — extract the dry-run/real-write branch", span.Lines, sizeratchet.MaxLines)
	}
}

func TestC1777_006_VersionbumpRunTestsPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	out, err := runPackageTests(t, goRoot, "./internal/versionbump")
	if err != nil {
		t.Errorf("go test ./internal/versionbump/... -count=1 failed (extraction changed observable behavior, including the dry-run zero-write contract):\n%s", out)
	}
}

func TestC1777_007_LlmcallsAggregateOffenderEntryRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	offenders := loadOffendersRaw(t, goRoot)
	if allowance, present := offenders["internal/llmcalls.Aggregate"]; present {
		t.Errorf("RED: offenders.json still lists internal/llmcalls.Aggregate (allowance %d) — entry must be dropped once Aggregate is shrunk to the ratchet limit", allowance)
	}
}

func TestC1777_008_LlmcallsAggregateWithinRatchetLimit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	span, found := spanFor(t, goRoot, "internal/llmcalls.Aggregate")
	if !found {
		t.Fatalf("RED: internal/llmcalls.Aggregate not found by sizeratchet.Walk — function renamed or moved")
	}
	if span.Lines > sizeratchet.MaxLines {
		t.Errorf("RED: internal/llmcalls.Aggregate is %d lines, want <= %d (sizeratchet.MaxLines) — extract the grouping/keying step", span.Lines, sizeratchet.MaxLines)
	}
}

func TestC1777_009_LlmcallsAggregateTestsPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	out, err := runPackageTests(t, goRoot, "./internal/llmcalls")
	if err != nil {
		t.Errorf("go test ./internal/llmcalls/... -count=1 failed (extraction changed observable behavior, including deterministic output ordering):\n%s", out)
	}
}
