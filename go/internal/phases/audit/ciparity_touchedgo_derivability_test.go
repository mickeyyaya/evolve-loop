package audit

// The touched∧derivable decision for the three whole-repo gates has one
// owner; no gate re-derives the change-set independently. Underivable ⇒
// (nil, error) so applyCIGate surfaces a WARN diagnostic — WARN not FAIL,
// since a transient index lock must not hard-block a shippable cycle. A
// genuinely Go-untouched but derivable cycle still no-ops silently, and a
// worktree with no Go module at all stays silent regardless of git state.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// wholeRepoGates are the three gates that share the touched∧derivable decision.
var wholeRepoGates = []struct {
	name  string
	check func(core.PhaseRequest) ([]string, error)
}{
	{"go vet", goVetCheckDefault},
	{"acs-durable", acsDurableCheckDefault},
	{"integration-tier", integrationTierCheckDefault},
}

// TestWholeRepoGates_UnderivableChangeSet_WarnsNotSilentSkip — the defect.
// enforceFixtureNonGit is reused for its shape (a real go module in a directory
// that is deliberately NOT a git repo, and no build handoff), so every git
// invocation FromGitChecked makes fails and the change-set is underivable by
// construction. Each gate must surface the WARN-carrying error instead of the
// silent (nil,nil).
func TestWholeRepoGates_UnderivableChangeSet_WarnsNotSilentSkip(t *testing.T) {
	root := enforceFixtureNonGit(t)
	// No fake runner installed: the fix must decide BEFORE forking any command,
	// so a gate that still tries to run `go vet ./...` here is also caught.
	for _, g := range wholeRepoGates {
		off, err := g.check(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()})
		if err == nil {
			t.Errorf("%s gate on an underivable change-set = (%v, nil), want a WARN-carrying error (silent skip is the fail-open defect)", g.name, off)
			continue
		}
		if len(off) > 0 {
			t.Errorf("%s gate must WARN, not FAIL, on a transient underivable change-set; got offenders %v", g.name, off)
		}
		if !strings.Contains(err.Error(), "underivable") {
			t.Errorf("%s gate WARN must name the underivable change-set so the cause is greppable; got %q", g.name, err.Error())
		}
	}
}

// TestWholeRepoGates_CleanDerivableTree_StaySilent — the paired negative. A real,
// clean, committed git repo yields a DERIVABLE empty change-set: the cycle
// genuinely touched no Go, so every gate must no-op silently. Without this, an
// "always WARN when the change-set is empty" implementation would pass the test
// above and put a spurious WARN on every docs-only cycle.
func TestWholeRepoGates_CleanDerivableTree_StaySilent(t *testing.T) {
	root := enforceFixtureCleanGit(t)
	for _, g := range wholeRepoGates {
		off, err := g.check(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()})
		if off != nil || err != nil {
			t.Errorf("%s gate on a clean DERIVABLE tree = (%v, %v), want (nil, nil) — no spurious WARN", g.name, off, err)
		}
	}
}

// TestWholeRepoGates_NoGoModule_StaySilent — a worktree with no go module has
// nothing to check whatever git says (a synthetic unit-test fixture, or a repo
// shape the gate cannot run in). It must stay silent, never WARN: the
// no-module guard has to be checked BEFORE derivability.
func TestWholeRepoGates_NoGoModule_StaySilent(t *testing.T) {
	root := t.TempDir() // no go/go.mod, and not a git repo either
	for _, g := range wholeRepoGates {
		off, err := g.check(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()})
		if off != nil || err != nil {
			t.Errorf("%s gate with no go module = (%v, %v), want (nil, nil)", g.name, off, err)
		}
	}
}

// TestWholeRepoGates_DerivableAndTouched_StillRun — the fix must not smother the
// happy path: a derivable, Go-touching cycle still reaches the command seam and
// maps its exit code as before.
func TestWholeRepoGates_DerivableAndTouched_StillRun(t *testing.T) {
	req := tierFixture(t)
	withFakeRunner(t, fakeRunFunc(1, "", "bad.go:5:2: declared and not used: x", nil))
	off, err := goVetCheckDefault(req)
	if err != nil {
		t.Fatalf("go vet gate on a touched+derivable cycle: unexpected error %v", err)
	}
	if len(off) == 0 {
		t.Fatal("go vet gate must still report offenders on a real vet failure")
	}
}
