package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func (cr *cycleRun) prepareAdoptedTree(wt, base string) error {
	archived, err := explanationdocs.ArchiveUnpublishedContinuationRecords(cr.ctx, wt, base)
	if err != nil {
		return fmt.Errorf("continuation unpublished explanation archive: %w", err)
	}
	if len(archived) > 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: archived %d unshipped ancestor explanation record(s) before Build\n", cr.cycle, len(archived))
	}
	packages, err := explanationdocs.ArchiveSupersededPredicatePackages(cr.ctx, wt, base, cr.cycle)
	if err != nil {
		return fmt.Errorf("continuation superseded predicate archive: %w", err)
	}
	if len(packages) > 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: archived the ancestor's own predicate package(s) before Build: %s\n", cr.cycle, strings.Join(packages, ", "))
	}
	return nil
}
