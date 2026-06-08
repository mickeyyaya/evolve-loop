//go:build acs

// Package redteam holds the standing red-team EGPS predicates — the anti-gaming
// invariants that fire every cycle (the Go lane runs `./acs/redteam`). Each is a
// thin wrapper over internal/redteamcheck (where the detection logic lives and
// is adversarially unit-tested in normal CI), run against the REAL .evolve/
// ledger + state. A predicate SKIPs when its evidence is absent (fresh clone)
// and FAILs (t.Errorf) on a detected gaming signature. Ported from
// acs/red-team/rt-*.sh (EGPS Go-native migration; ADR-0025).
package redteam

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/redteamcheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// evolveDir resolves the .evolve/ directory the predicate inspects: the suite
// exports EVOLVE_PROJECT_ROOT (MAIN, even from a worktree — issue #12), else the
// repo root.
func evolveDir(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	return filepath.Join(root, ".evolve")
}

// TestRT001_LedgerRoleCompleteness ports red-team-001: the last completed cycle
// must have scout + builder + auditor agent_subprocess entries (cycle-102-111).
func TestRT001_LedgerRoleCompleteness(t *testing.T) {
	skip, err := redteamcheck.LedgerRoleCompleteness(filepath.Join(evolveDir(t), "ledger.jsonl"))
	if skip {
		t.Skip("ledger / completed cycle absent — red-team-001 not applicable")
	}
	if err != nil {
		t.Errorf("RED red-team-001: %v", err)
	}
}

// TestRT002_NoBatchCycleJump ports red-team-002: state.json:lastCycleNumber must
// not run >1 ahead of the highest cycle with ledger evidence (cycle-132-141).
func TestRT002_NoBatchCycleJump(t *testing.T) {
	ev := evolveDir(t)
	skip, err := redteamcheck.NoBatchCycleJump(filepath.Join(ev, "ledger.jsonl"), filepath.Join(ev, "state.json"))
	if skip {
		t.Skip("ledger / state.json absent — red-team-002 not applicable")
	}
	if err != nil {
		t.Errorf("RED red-team-002: %v", err)
	}
}

// TestRT003_ChallengeTokenIntegrity ports red-team-003: every agent_subprocess
// entry for the last completed cycle carries a non-empty challenge_token.
func TestRT003_ChallengeTokenIntegrity(t *testing.T) {
	skip, err := redteamcheck.ChallengeTokenIntegrity(filepath.Join(evolveDir(t), "ledger.jsonl"))
	if skip {
		t.Skip("ledger / completed cycle / entries absent — red-team-003 not applicable")
	}
	if err != nil {
		t.Errorf("RED red-team-003: %v", err)
	}
}
