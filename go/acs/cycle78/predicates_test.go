// Package cycle78 ports the cycle-78 ACS predicates (1 bash file, 7 ACs).
package cycle78

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC78_001_RetrospectiveColdMoveStage9 ports cycle-78/001.
// Retrospective cold-move + scout ceiling calibration (7 ACs).
func TestC78_001_RetrospectiveColdMoveStage9(t *testing.T) {
	root := acsassert.RepoRoot(t)
	retro := filepath.Join(root, "agents", "evolve-retrospective.md")
	ref := filepath.Join(root, "agents", "evolve-retrospective-reference.md")
	adr := filepath.Join(root, "docs", "architecture", "adr", "0016-retrospective-cold-move-stage9.md")
	scoutProfile := filepath.Join(root, ".evolve", "profiles", "scout.json")

	if !fixtures.FilePresent(retro) {
		t.Skip("evolve-retrospective.md missing — skip cycle-78-001")
	}

	// AC1: ≤ 253 lines (10% reduction from 281)
	if lines := countLines(t, retro); lines > 253 {
		t.Errorf("AC1: %s has %d lines (expected ≤ 253)", retro, lines)
	}

	// AC2 + AC3: reference sections
	if acsassert.FileExists(t, ref) {
		for _, section := range []string{
			"## Section: digest-format-template",
			"## Section: handoff-schema",
		} {
			if !acsassert.FileContains(t, ref, section) {
				return
			}
		}
	}

	// AC4 + AC5: pointers in hot persona
	for _, pat := range []string{
		`evolve-retrospective-reference\.md.*digest-format-template`,
		`evolve-retrospective-reference\.md.*handoff-schema`,
	} {
		if !acsassert.FileMatchesRegex(t, retro, pat) {
			return
		}
	}

	// AC6: ADR-0016 exists
	if !fixtures.FilePresent(adr) {
		t.Errorf("AC6: ADR-0016 missing: %s", adr)
		return
	}

	// AC7: scout max_turns == 30
	if !fixtures.FilePresent(scoutProfile) {
		t.Skipf("scout profile missing: %s — skip AC7", scoutProfile)
	}
	turns, err := readMaxTurns(scoutProfile)
	if err != nil {
		t.Errorf("AC7: parse %s: %v", scoutProfile, err)
		return
	}
	// Cycle-78 AC7 calibrated scout to 30. Cycle-102 raised the floor to 42
	// (carryover abnormal-turn-overrun-c99). Accept either the historical
	// value or a calibrated higher ceiling — but not a regression below 30.
	if turns < 30 {
		t.Errorf("AC7: scout max_turns=%d (regression below cycle-78 floor of 30)", turns)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<22)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}

func readMaxTurns(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0, err
	}
	v, ok := doc["max_turns"]
	if !ok {
		return 0, fmt.Errorf("max_turns missing")
	}
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	}
	return 0, fmt.Errorf("max_turns wrong type: %T", v)
}
