package heldaudit

import (
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

var writerPhases = []string{"build", "debugger"}

func BuilderFamilies(workspace string, cycle int, modelFamily func(cli string) string) ([]string, error) {
	var families []string
	addFromCLI := func(source, cli string) error {
		family := modelFamily(cli)
		if family == "" {
			return fmt.Errorf("%w: %s names cli %q, which has no model family", heldproposal.ErrAuditUnattributed, source, cli)
		}
		if !slices.Contains(families, family) {
			families = append(families, family)
		}
		return nil
	}
	calls, err := llmcalls.ReadWorkspace(workspace)
	if err != nil {
		return nil, fmt.Errorf("%w: read the dispatch records: %v", heldproposal.ErrAuditUnattributed, err)
	}
	for _, rec := range calls.Records {
		if rec.Cycle == cycle && slices.Contains(writerPhases, rec.Phase) {
			if err := addFromCLI("the "+rec.Phase+" dispatch", rec.CLI); err != nil {
				return nil, err
			}
		}
	}
	fix, found, err := auditfix.ReadRecord(workspace)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", heldproposal.ErrAuditUnattributed, err)
	}
	if found && fix.Auditor.Family != "" && (len(fix.Patch.Changes) > 0 || fix.Patch.Diff != "") {
		cliFamily := fix.Auditor.Family
		if err := addFromCLI("the audit-fix patch", cliFamily); err != nil {
			return nil, err
		}
	}
	if len(families) == 0 {
		return nil, fmt.Errorf("%w: no dispatch record of cycle %d names a writer", heldproposal.ErrAuditUnattributed, cycle)
	}
	slices.Sort(families)
	return families, nil
}
