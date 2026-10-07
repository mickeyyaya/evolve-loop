package core

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const gateDerivedClass = failurelog.CodeAuditFail

const HarnessRedClauseMarker = " red(s) are the harness's own: predicates that could not run"

type auditGateRemedy struct {
	gate            string
	reasonPrefix    string
	remediation     string
	isStaticReading bool
}

var auditGateRemedies = []auditGateRemedy{
	{gate: "gofmt", reasonPrefix: "gofmt: ", isStaticReading: true,
		remediation: "run `gofmt -w -s .` in go/"},
	{gate: "solution-contract", reasonPrefix: "solution contract: ", isStaticReading: true,
		remediation: "fix each listed violation, then self-check with `evolve solution check`"},
	{gate: "skills-drift", reasonPrefix: "skill projection drift: ", isStaticReading: true,
		remediation: "regenerate with the worktree's own generator: `EVOLVE_WORKTREE_ROOT=<worktree> go run ./cmd/evolve skills generate` in <worktree>/go (an installed evolve binary renders its own build's templates)"},
	{gate: "go vet gate", reasonPrefix: "go vet ./... reported ",
		remediation: "fix each offender `go vet ./...` reports in go/"},
	{gate: "acs-durable gate", reasonPrefix: "acs-durable (-tags acs) FAILED ",
		remediation: "make each offender pass under `make -C go test-acs-durable`"},
	{gate: "integration-tier gate", reasonPrefix: "the integration tier (`go test -tags integration`) reported ",
		remediation: "make each offender pass under `go test -tags integration` in go/"},
	{gate: "apicover-enforce gate", reasonPrefix: "apicover -enforce flagged ",
		remediation: "name each flagged export in a test of its package (apicover_named_test.go)"},
	{gate: "apicover new-package graduation gate", reasonPrefix: "apicover new-package graduation: ", isStaticReading: true,
		remediation: "add each new package to go/.apicover-enforce with an apicover_named_test.go"},
	{gate: "EGPS red_count>0", reasonPrefix: "EGPS: red_count=",
		remediation: "turn every red ACS predicate green"},
}

func auditGateRemedyFor(reason string) (auditGateRemedy, bool) {
	if strings.Contains(reason, HarnessRedClauseMarker) {
		return auditGateRemedy{}, false
	}
	for _, row := range auditGateRemedies {
		if strings.HasPrefix(reason, row.reasonPrefix) {
			return row, true
		}
	}
	return auditGateRemedy{}, false
}

func AuditGateOf(reason string) string {
	row, _ := auditGateRemedyFor(reason)
	return row.gate
}

func AuditGateRemedy(gate string) string {
	for _, row := range auditGateRemedies {
		if row.gate == gate {
			return row.remediation
		}
	}
	return ""
}

type gateDiagnosis struct {
	row    auditGateRemedy
	reason string
}

func gateForcedDiagnoses(reasons []string) ([]gateDiagnosis, bool) {
	conflict := false
	var diagnoses []gateDiagnosis
	for _, reason := range reasons {
		if BookkeepingConflictAuditReason(reason) {
			conflict = true
			continue
		}
		row, ok := auditGateRemedyFor(reason)
		if !ok {
			return nil, false
		}
		diagnoses = append(diagnoses, gateDiagnosis{row: row, reason: reason})
	}
	return diagnoses, conflict && len(diagnoses) > 0
}

func gateFailureBlock(reasons []string) (*phasecontract.FailureBlock, bool) {
	diagnoses, ok := gateForcedDiagnoses(reasons)
	if !ok {
		return nil, false
	}
	defects := make([]string, len(diagnoses))
	for i, d := range diagnoses {
		defects[i] = d.row.remediation + " — " + d.reason
	}
	return &phasecontract.FailureBlock{Class: string(gateDerivedClass), Defects: defects}, true
}

func staticGateReadingsForcedTheFail(reasons []string) bool {
	diagnoses, ok := gateForcedDiagnoses(reasons)
	if !ok {
		return false
	}
	for _, d := range diagnoses {
		if !d.row.isStaticReading {
			return false
		}
	}
	return true
}

func gateRemediationEnvelope(env retryEnvelope) retryEnvelope {
	return narrowRetryEnvelope(env, retryActionRetryBuild,
		"only deterministic gates forced the FAIL over a PASS or WARN narrative (a verdict-conflict record exists): Build applies their named remediation, TDD is skipped")
}
