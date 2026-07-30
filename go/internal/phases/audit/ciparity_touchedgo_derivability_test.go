package audit

// ciparity_touchedgo_derivability_test.go — RED contract for the inbox defect
// `cycletouchedgo-derivability-silent-skip`.
//
// TODAY cycleTouchedGo computed `pkgs, _ := changedPackagesForAudit(...); return
// len(pkgs) > 0` — it DISCARDED the derivable bool. changedpkgs.FromGitChecked
// only ever returns (nil,false) on git failure, so !derivable implies
// len(pkgs)==0 implies cycleTouchedGo==false. When git diff fails (the concrete
// trigger the changedpkgs doc names: a concurrent-fleet .git/index.lock race)
// all three whole-repo gates — go vet, acs-durable, integration-tier — short-
// circuited to (nil,nil): a SILENT skip, neither a WARN nor a whole-repo run.
//
// That is the cycle-581 D1/D2 fail-open conflation (git-clean vs underivable)
// reintroduced one call-frame UP from where apicover was hardened against it.
// A gate that cannot determine its input must not resolve to "nothing to check".
//
// FIX CONTRACT:
//   - the touched∧derivable decision has ONE owner (changedScopeForGate); no
//     gate re-derives the change-set independently;
//   - underivable ⇒ (nil, error) so applyCIGate surfaces a WARN diagnostic
//     ("gate skipped, CI backstops") — WARN not FAIL, since a transient index
//     lock must not hard-block a shippable cycle;
//   - a genuinely Go-untouched but DERIVABLE cycle still no-ops silently — the
//     paired negative that stops a naive "always WARN" implementation;
//   - a worktree with no Go module at all stays silent (nothing to check), so a
//     docs-only / synthetic-fixture cycle gains no spurious WARN.

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

// TestChangedScopeForGate_SingleOwnerOfTouchedAndDerivable pins the replacement
// for cycleTouchedGo as the SINGLE source of the decision the three gates
// consult, across all three input classes. It returns the change-set itself so
// the integration tier scopes from it rather than re-deriving (acceptance bullet
// 2: "no gate re-derives independently").
func TestChangedScopeForGate_SingleOwnerOfTouchedAndDerivable(t *testing.T) {
	t.Run("underivable: git failed → run=false with a WARN error", func(t *testing.T) {
		root := enforceFixtureNonGit(t)
		pkgs, run, err := changedScopeForGate(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
		if run || err == nil {
			t.Fatalf("= (%v, run=%v, %v), want (nil, false, error)", pkgs, run, err)
		}
		if pkgs != nil {
			t.Errorf("an underivable set must yield no packages, got %v", pkgs)
		}
	})

	t.Run("derivable and empty: clean tree → run=false, no error", func(t *testing.T) {
		root := enforceFixtureCleanGit(t)
		pkgs, run, err := changedScopeForGate(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
		if run || err != nil || pkgs != nil {
			t.Fatalf("= (%v, run=%v, %v), want (nil, false, nil) — a silent no-op", pkgs, run, err)
		}
	})

	t.Run("derivable and touched: handoff → run=true with the change-set", func(t *testing.T) {
		req := tierFixture(t) // go module + handoff naming go/internal/widget/w.go
		pkgs, run, err := changedScopeForGate(req)
		if !run || err != nil {
			t.Fatalf("= (%v, run=%v, %v), want (pkgs, true, nil)", pkgs, run, err)
		}
		if len(pkgs) != 1 || pkgs[0] != "./internal/widget/..." {
			t.Errorf("pkgs = %v, want the handoff's package so the tier need not re-derive", pkgs)
		}
	})
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
