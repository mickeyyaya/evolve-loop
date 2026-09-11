package audit

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// executeRepositoryGates runs local formatting and contract gates before the
// commands that mirror repository CI. Each gate keeps its historical fail-open
// behavior for infrastructure errors and fail-closed behavior for offenders.
func (a *auditClassification) executeRepositoryGates() {
	a.applyRepositoryGate(a.hooks.gofmtCheck, "gofmt",
		"gofmt gate skipped (could not run): %s",
		"gofmt: %d file(s) are not gofmt -s clean — CI `vet + fmt` would FAIL. Run `gofmt -w -s .` in go/. Offenders: %s",
		", ",
	)
	a.applyRepositoryGate(a.hooks.solutionCheck, "solution-contract",
		"solution-contract gate skipped (could not run): %s",
		"solution contract: %d violation(s) in the document deliverable — fix these exactly (self-check: `evolve solution check`): %s",
		"; ",
	)
	a.applyRepositoryGate(a.hooks.skillsDriftCheck, "skills-drift",
		"skills-drift gate skipped (could not run): %s",
		"skill projection drift: %d artifact(s) stale vs their SSOTs (SKILL.md phase-facts and/or commands/ stubs) — CI TestSkills_NoDrift would FAIL. Run `evolve skills generate`. Drifted: %s",
		", ",
	)

	a.applyCIGate(a.hooks.goVetCheck, "go vet gate",
		"go vet ./... reported %d issue(s) — CI `vet + fmt` would FAIL (e.g. import cycle). Offenders: %s")
	a.applyCIGate(a.hooks.acsDurableCheck, "acs-durable gate",
		"acs-durable (-tags acs) FAILED %d check(s) — CI acs-durable gate would FAIL (flag-registry / flag-ceiling / skills-drift). Offenders: %s")
	a.applyCIGate(a.hooks.integrationTierCheck, "integration-tier gate",
		integrationTierTemplateWithCaveat(ciParityCaveatNow()))
	a.applyCIGate(a.hooks.apicoverEnforceCheck, "apicover-enforce gate",
		"apicover -enforce flagged %d line(s) in touched enforced packages — CI `api-coverage enforce` would FAIL (unnamed export). Offenders: %s")
	a.applyCIGate(a.hooks.apicoverNewPkgGraduationCheck, "apicover new-package graduation gate",
		"%d new go/internal/<pkg>(s) changed this cycle are absent from .apicover-enforce — the apicover -enforce gate silently skips them (new-package blind spot). Add each to go/.apicover-enforce + an apicover_named_test.go before ship. Offenders: %s")
}

func (a *auditClassification) applyRepositoryGate(
	check func(core.PhaseRequest) ([]string, error),
	label, skippedTemplate, failedTemplate, separator string,
) {
	if check == nil {
		return
	}
	offenders, err := check(a.req)
	switch {
	case err != nil:
		a.warn(fmt.Sprintf(skippedTemplate, err.Error()))
	case len(offenders) > 0:
		a.fail(label, fmt.Sprintf(failedTemplate, len(offenders), strings.Join(offenders, separator)))
	}
}

func (a *auditClassification) applyCIGate(
	check func(core.PhaseRequest) ([]string, error),
	name, failedTemplate string,
) {
	if check == nil {
		return
	}
	offenders, err := check(a.req)
	switch {
	case err != nil:
		a.warn(fmt.Sprintf("%s skipped (could not run): %s", name, err.Error()))
	case len(offenders) > 0:
		a.fail(name, fmt.Sprintf(failedTemplate, len(offenders), strings.Join(offenders, "; ")))
	}
}
