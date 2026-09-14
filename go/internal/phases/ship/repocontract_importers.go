package ship

// repocontract_importers.go — the importer backstop (2026-09-14).
//
// The fixed scanner pack catches repo-wide guard suites and the added-test
// backstop catches red-first reproducers, but neither covers the shape that
// redded main from 4db205a8 until #590 (cycles 1657/1659): a lane MODIFIES a
// package, its own package tests stay green, and an UNTOUCHED test in a
// package that imports it still asserts the old contract. Per-package scope
// cannot see that edge; only the import graph can.
//
// The seed is the tree the ship will land, measured against its base — the
// one derivation in internal/changedpkgs (working tree, never the index: a
// lane's build output is unstaged until the ship itself stages it). The
// closure is changedpkgs.ImporterClosureChecked — build deps and the direct
// imports of each package's tests — projected to what `go test` can run under
// the default build context, minus what the fixed pack already ran, through
// the same classified pack runner as the other two layers.

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const (
	// importerBackstopTimeout bounds one importer-backstop run. A core change
	// closes over most of the module — what main's CI runs, moved before the
	// push — and that run must still end inside the ship phase; past the bound
	// the pack exits with no test-level failure and is classed infra.
	importerBackstopTimeout = 20 * time.Minute
	// importerBackstopRetryMaxTargets caps the pack size the ambiguous-exit
	// retry (cycle-1402/1403/1405 class) is still worth: above it the second
	// full run is more expensive than a re-dispatch, so an ambiguous exit is
	// classed infra straight away.
	importerBackstopRetryMaxTargets = 25
	// discoveryRetryPause gives the one named transient — a concurrent lane's
	// index.lock — time to clear before the single discovery retry.
	discoveryRetryPause = time.Second
)

// changedFilesTwice is the gate's seed: changedpkgs.ChangedFilesChecked with
// one retry (a concurrent fleet lane's index.lock is the known transient),
// classed CodeRepoContractInfra when git still cannot answer — a discovery
// failure is re-dispatchable, never a silent green.
func changedFilesTwice(out io.Writer, root, baseRef string) ([]changedpkgs.ChangedFile, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if files, ok := changedpkgs.ChangedFilesChecked(root, baseRef); ok {
			return files, nil
		}
		time.Sleep(discoveryRetryPause)
	}
	fmt.Fprintf(out, "[ship] repo-contract added-test and importer backstops: change discovery unavailable — git could not derive %s vs %s (twice); classed INFRA, the backstops did NOT run\n", root, baseRef)
	return nil, shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
		fmt.Sprintf("repo-contract backstop discovery: git could not derive the changes of %s vs %s (twice) — INFRA fault, not a contract violation; safe to re-dispatch", root, baseRef))
}

// runImporterBackstop is the third gate layer. files is the gate's seed;
// alreadyRun are the patterns an earlier layer paid for in the default build
// context (the fixed pack, the untagged added-test groups).
func runImporterBackstop(ctx context.Context, out io.Writer, root, moduleDir, workspace string, files []changedpkgs.ChangedFile, alreadyRun []string) error {
	changed := changedpkgs.PackagesOf(files)
	if len(changed) == 0 {
		fmt.Fprintf(out, "[ship] repo-contract importer backstop: no Go change in the tree — skipped\n")
		return nil
	}
	closure, ok := changedpkgs.ImporterClosureChecked(root, changed)
	if !ok {
		time.Sleep(discoveryRetryPause)
		closure, ok = changedpkgs.ImporterClosureChecked(root, changed)
	}
	if !ok {
		return shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
			fmt.Sprintf("repo-contract importer backstop discovery: `go list` could not walk the module's import graph in %s (twice) — INFRA fault, not a contract violation; safe to re-dispatch", moduleDir))
	}
	if skipped := without(closure.Patterns, closure.Testable); len(skipped) > 0 {
		fmt.Fprintf(out, "[ship] repo-contract importer backstop: %d pattern(s) with no default-context package (tag-only or removed) not run: %s\n", len(skipped), strings.Join(skipped, " "))
	}
	targets := without(closure.Testable, append(append([]string{}, repoContractPackages...), alreadyRun...))
	if len(targets) == 0 {
		fmt.Fprintf(out, "[ship] repo-contract importer backstop: closure of %s already run by an earlier layer or not testable — nothing more to run\n", strings.Join(changed, " "))
		return nil
	}
	fmt.Fprintf(out, "[ship] repo-contract importer backstop: go test -json -count=1 %s (%d changed → %d in closure, %d already run by an earlier layer)\n",
		strings.Join(targets, " "), len(changed), len(closure.Patterns), len(closure.Testable)-len(targets))
	bctx, cancel := context.WithTimeout(ctx, importerBackstopTimeout)
	defer cancel()
	start := time.Now()
	defer func() {
		fmt.Fprintf(out, "[ship] repo-contract importer backstop: %d target(s) in %s\n", len(targets), time.Since(start).Round(time.Second))
	}()
	return runClassifiedPackRetrying(bctx, out, workspace, "importer backstop", len(targets) <= importerBackstopRetryMaxTargets, func() packOutcome {
		return runRepoContractPackages(bctx, moduleDir, out, targets)
	})
}

// without returns xs minus every element of drop, order preserved.
func without(xs, drop []string) []string {
	skip := map[string]bool{}
	for _, d := range drop {
		skip[d] = true
	}
	var out []string
	for _, x := range xs {
		if !skip[x] {
			out = append(out, x)
		}
	}
	return out
}
