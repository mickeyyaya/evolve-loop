//go:build integration

package ship

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const passReportUnderHostWarn = "<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: PASS\n\nAll criteria met (test fixture).\n"

func TestVerifyAuditBinding_AHostWarnOutranksAPassInTheReportUnderStrictAudit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedCustomAudit(t, repo, passReportUnderHostWarn, 1)
	writeStrictAuditPolicy(t, repo)

	err := verifyAuditBinding(context.Background(), auditOpts(t, repo), &RunResult{}) //nolint:staticcheck

	wantShipErr(t, err, core.CodeAuditBindingVerdictWarn, core.ShipClassPrecondition, "WARN")
}

func TestVerifyAuditBinding_AHostWarnShipsAsWarnWithoutStrictAudit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedCustomAudit(t, repo, passReportUnderHostWarn, 1)
	res := &RunResult{}

	if err := verifyAuditBinding(context.Background(), auditOpts(t, repo), res); err != nil { //nolint:staticcheck
		t.Fatalf("a host WARN without strict audit ships: %v", err)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "audit verdict: WARN") {
		t.Errorf("logs %q: the ship records the host WARN, not the PASS of the report", res.Logs)
	}
}

func TestVerifyAuditBinding_AHostWarnDoesNotRepairAReportWithNoVerdict(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedCustomAudit(t, repo, "<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nNo verdict line.\n", 1)

	err := verifyAuditBinding(context.Background(), auditOpts(t, repo), &RunResult{}) //nolint:staticcheck

	wantShipErr(t, err, core.CodeAuditBindingMalformed, core.ShipClassPrecondition, "no recognizable verdict")
}
