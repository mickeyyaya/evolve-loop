package ciparitygate

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

// moduleDir resolves the cycle's go/ module dir (where the builder's code
// lives) under root. Empty → the no-op signal (""). It requires a real go
// module (go.mod present): ModuleDir falls back to root itself when there is
// no go/ subdir, so an IsDir check alone would run a gate in a non-module
// directory — go vet then fails "go.mod not found", a false offender. A
// synthetic/incomplete test worktree has no go.mod.
func moduleDir(root string) string {
	if root == "" {
		return ""
	}
	dir := codequality.ModuleDir(root)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return ""
	}
	return dir
}

// errChangeSetUnderivable is the WARN-carrying error every whole-repo gate
// returns when it cannot determine its own input. The host maps a non-nil
// error to a WARN diagnostic (verdict unchanged), which is the deliberate
// severity: the concrete trigger is a transient concurrent-fleet
// `.git/index.lock` race, and a hard FAIL there would discard a shippable
// cycle. Loud-but-soft — never the silent no-op it replaced.
var errChangeSetUnderivable = errors.New(
	"changed-package set is underivable this cycle (git diff failed — e.g. a concurrent-fleet .git/index.lock race), " +
		"so this whole-repo gate cannot tell an untouched cycle from an unreadable one: gate skipped, CI backstops it")

// scope is the SINGLE owner of the touched∧derivable decision the three
// whole-repo gates (go vet, acs-durable, integration tier) consult — no gate
// derives the change set independently. Three outcomes:
//
//	(pkgs, true,  nil) — a real cycle build touching >= 1 Go package: run, scoped to pkgs.
//	(nil,  false, err) — UNDERIVABLE: git failed, so "touched nothing" is unknowable.
//	                     Fail-open LOUD (WARN + CHANGESET_UNDERIVABLE), never silently.
//	(nil,  false, nil) — derivable and genuinely empty (docs-only cycle, or no Go
//	                     module at all): nothing to check, stay silent.
//
// The no-module guard is checked FIRST: a worktree with no go.mod has nothing
// to check whatever git says, so a synthetic fixture gains no spurious WARN.
// Each gate resolves its own scope, so an underivable cycle records three
// identical WARN diagnostics and three events — accurate, if repetitive
// (preserved quirk Q1; hoisting the derivation into Classify is follow-up 14-1).
func (g *Gates) scope(at gateOrigin, req Request) (pkgs []string, run bool, err error) {
	if moduleDir(req.root()) == "" {
		return nil, false, nil // no go module in the worktree → nothing to check
	}
	pkgs, derivable := g.changedSet(req.root(), req.Cycle)
	if !derivable {
		g.warn(at, req, CodeChangeSetUnderivable, errChangeSetUnderivable.Error(), "root", req.root())
		return nil, false, errChangeSetUnderivable
	}
	return pkgs, len(pkgs) > 0, nil
}
