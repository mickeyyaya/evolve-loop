//go:build acs

package redteam

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/redteamcheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func evolveDir(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	return filepath.Join(root, ".evolve")
}

func TestRT001_LedgerRoleCompleteness(t *testing.T) {
	skip, err := redteamcheck.LedgerRoleCompleteness(filepath.Join(evolveDir(t), "ledger.jsonl"))
	if skip {
		t.Skip("ledger / completed cycle absent — red-team-001 not applicable")
	}
	if err != nil {
		t.Errorf("RED red-team-001: %v", err)
	}
}

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

func TestRT003_ChallengeTokenIntegrity(t *testing.T) {
	skip, err := redteamcheck.ChallengeTokenIntegrity(filepath.Join(evolveDir(t), "ledger.jsonl"))
	if skip {
		t.Skip("ledger / completed cycle / entries absent — red-team-003 not applicable")
	}
	if err != nil {
		t.Errorf("RED red-team-003: %v", err)
	}
}
