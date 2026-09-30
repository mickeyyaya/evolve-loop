//go:build acs

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
