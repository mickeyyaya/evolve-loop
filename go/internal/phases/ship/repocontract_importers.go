package ship

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
	importerBackstopTimeout         = 20 * time.Minute
	importerBackstopRetryMaxTargets = 25
	discoveryRetryPause             = time.Second
)

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

func runImporterBackstop(ctx context.Context, out packLog, root, moduleDir, workspace string, files []changedpkgs.ChangedFile, alreadyRun []string, cleared func([]string)) error {
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
	fmt.Fprintf(out, "[ship] repo-contract importer backstop: go test -json -count=1 -timeout %s %s (%d changed → %d in closure, %d already run by an earlier layer)\n",
		repoContractTestTimeout,
		strings.Join(targets, " "), len(changed), len(closure.Patterns), len(closure.Testable)-len(targets))
	bctx, cancel := context.WithTimeout(ctx, importerBackstopTimeout)
	defer cancel()
	start := time.Now()
	defer func() {
		fmt.Fprintf(out, "[ship] repo-contract importer backstop: %d target(s) in %s\n", len(targets), time.Since(start).Round(time.Second))
	}()
	return runClassifiedPackRetrying(bctx, out, workspace, "importer backstop", len(targets) <= importerBackstopRetryMaxTargets, func() packOutcome {
		return runRepoContractPackages(bctx, moduleDir, out, targets)
	}, &aloneRerun{
		run: func(failures []packFailure) packOutcome {
			return runRepoContractPackagesAlone(bctx, moduleDir, out, failures)
		},
		cleared: cleared,
	})
}

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
