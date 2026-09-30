package ciparitygate

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

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

var errChangeSetUnderivable = errors.New(
	"changed-package set is underivable this cycle (git diff failed — e.g. a concurrent-fleet .git/index.lock race), " +
		"so this whole-repo gate cannot tell an untouched cycle from an unreadable one: gate skipped, CI backstops it")

func (g *Gates) scope(at gateOrigin, req Request) (pkgs []string, run bool, err error) {
	if moduleDir(req.root()) == "" {
		return nil, false, nil
	}
	pkgs, derivable := g.changedSet(req.root(), req.Cycle)
	if !derivable {
		g.warn(at, req, CodeChangeSetUnderivable, errChangeSetUnderivable.Error(), "root", req.root())
		return nil, false, errChangeSetUnderivable
	}
	return pkgs, len(pkgs) > 0, nil
}
