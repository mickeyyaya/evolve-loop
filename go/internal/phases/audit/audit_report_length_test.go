package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// audit_report_length_test.go — RED contract for the cycle-1522 fleet-scoped
// task `cap-audit-report-length` (scout-report.md ## Selected Tasks, Task 1).
//
// The defect: audit-report.md has no upper bound on total size. The ## Issues
// table grows one row per finding with no cap, the report is re-read in full at
// ship time (go/internal/phases/ship/audit.go:83, which also SHA-binds it), and
// the next cycle's handoff carries prior audit context — so an oversized report
// compounds token cost on every downstream read. defect_ledger.go:56-61 already
// bounds the *ledger* (defectLedgerMaxEntries / defectTextMaxRunes) with the
// exact idiom this task extends to the *report*: overflow is RECORDED, never
// silently dropped.
//
// The contract pinned here, in three load-bearing parts:
//
//  1. A package const `auditReportMaxBytes` exists and carries a sane budget
//     (this file references it directly, so RED is a compile failure until
//     Builder declares it).
//  2. Classify emits EXACTLY ONE warning-severity diagnostic when the artifact
//     exceeds the cap, naming both the actual size and the cap.
//  3. The check is DIAGNOSTIC-ONLY. It must never flip the verdict (an
//     oversized but green report still classifies exactly as its verdict says)
//     and must never mutate the on-disk artifact — ship SHA-binds those bytes
//     (ship/audit.go:83), so a truncating cap would break the ship-time
//     integrity check.
//
// Severity is WIRING, not taste: core.errorSeverityMessages keys off
// Severity=="error" to build AuditFailReasons, so an error-severity size
// diagnostic would convert a merely-verbose report into a dossier-visible
// failure. The size warning must be Severity=="warning".

// sizeMarker is the stable substring every size diagnostic must carry, so a
// downstream operator (and this contract) can find it without matching prose.
const sizeMarker = "audit-report.md size"

// reportOfSize renders a valid PASS audit-report.md padded to exactly n bytes.
// Padding rides in a body line, never inside the verdict block, so the verdict
// stays parseable at every size — the size axis is isolated from the verdict
// axis (a fixture whose padding broke verdict extraction would prove nothing).
func reportOfSize(t *testing.T, n int) string {
	t.Helper()
	base := "# Audit Report\n\n## Issues\n\n\n\n## Verdict\n**PASS**\n"
	if len(base) > n {
		t.Fatalf("cannot build a %d-byte report: the minimal valid shape is already %d bytes", n, len(base))
	}
	pad := strings.Repeat("x", n-len(base))
	out := strings.Replace(base, "## Issues\n\n\n", "## Issues\n\n"+pad+"\n", 1)
	if len(out) != n {
		t.Fatalf("fixture is %d bytes, want %d", len(out), n)
	}
	return out
}

// classifySized runs the REAL production Classify path (the seam
// runner.BaseRunner.Run calls at runner.go:1117) over a temp workspace holding
// a green acs-verdict.json plus the artifact on disk, and returns the verdict,
// the diagnostics, and the artifact's post-call on-disk bytes.
func classifySized(t *testing.T, artifact string) (string, []core.Diagnostic, string) {
	t.Helper()
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	path := filepath.Join(ws, "audit-report.md")
	if err := os.WriteFile(path, []byte(artifact), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read artifact: %v", err)
	}
	return verdict, diags, string(after)
}

// sizeDiags returns every diagnostic carrying the size marker.
func sizeDiags(diags []core.Diagnostic) []core.Diagnostic {
	var out []core.Diagnostic
	for _, d := range diags {
		if strings.Contains(d.Message, sizeMarker) {
			out = append(out, d)
		}
	}
	return out
}

// TestAuditReportLength is the table-driven size contract. Sub-test names are
// part of the contract: go/acs/cycle1522 asserts each one reports PASS by name,
// so a rename or a silent skip cannot green the ACS predicate.
func TestAuditReportLength(t *testing.T) {
	capBytes := auditReportMaxBytes

	t.Run("cap_value_sane", func(t *testing.T) {
		// A cap of 1 byte would "pass" every behavioral case below while making
		// every real report oversized; a cap of 1GB would never bite. Bound the
		// budget to the range a real audit-report.md lives in.
		if capBytes < 8*1024 || capBytes > 1<<20 {
			t.Errorf("auditReportMaxBytes=%d out of sane range [8KiB, 1MiB] — a cap outside it "+
				"either fires on every report or never fires at all", capBytes)
		}
	})

	t.Run("under_cap_silent", func(t *testing.T) {
		// Negative case (the anti-no-op signal): an implementation that warns
		// unconditionally passes the over-cap case and fails here.
		verdict, diags, _ := classifySized(t, reportOfSize(t, capBytes/2))
		if got := sizeDiags(diags); len(got) != 0 {
			t.Errorf("under-cap report (%d bytes, cap %d) emitted %d size diagnostic(s), want 0: %v",
				capBytes/2, capBytes, len(got), got)
		}
		if verdict != core.VerdictPASS {
			t.Errorf("verdict=%q, want PASS — an under-cap green report must classify normally", verdict)
		}
	})

	t.Run("exact_boundary_silent", func(t *testing.T) {
		// The boundary is EXPLICIT: len == cap is within budget; only len > cap
		// warns. Pinning it here stops an off-by-one from being re-litigated.
		_, diags, _ := classifySized(t, reportOfSize(t, capBytes))
		if got := sizeDiags(diags); len(got) != 0 {
			t.Errorf("exactly-at-cap report (%d bytes) emitted %d size diagnostic(s), want 0 — "+
				"the boundary is `> cap` warns, `== cap` is silent: %v", capBytes, len(got), got)
		}
	})

	t.Run("over_cap_warns_once", func(t *testing.T) {
		size := capBytes + 1
		_, diags, _ := classifySized(t, reportOfSize(t, size))
		got := sizeDiags(diags)
		if len(got) != 1 {
			t.Fatalf("over-cap report (%d bytes, cap %d) emitted %d size diagnostic(s), want exactly 1: %v",
				size, capBytes, len(got), diags)
		}
		if got[0].Severity != "warning" {
			t.Errorf("size diagnostic severity=%q, want \"warning\" — error severity would route the "+
				"report through errorSeverityMessages into AuditFailReasons, converting a verbose "+
				"report into a dossier-visible failure: %s", got[0].Severity, got[0].Message)
		}
		for _, want := range []string{strconv.Itoa(size), strconv.Itoa(capBytes)} {
			if !strings.Contains(got[0].Message, want) {
				t.Errorf("size diagnostic omits %q — an operator cannot tell how far over budget "+
					"the report is: %s", want, got[0].Message)
			}
		}
	})

	t.Run("over_cap_does_not_flip_verdict", func(t *testing.T) {
		// The cap is advisory. A green, oversized report still ships.
		verdict, _, _ := classifySized(t, reportOfSize(t, capBytes*2))
		if verdict != core.VerdictPASS {
			t.Errorf("verdict=%q, want PASS — the size check is diagnostic-only and must never "+
				"block a ship on verbosity alone", verdict)
		}
	})

	t.Run("over_cap_does_not_mutate_artifact", func(t *testing.T) {
		// ship/audit.go:83 re-reads and SHA-binds these exact bytes; a cap that
		// truncated on disk would break that integrity check.
		want := reportOfSize(t, capBytes*2)
		_, _, after := classifySized(t, want)
		if after != want {
			t.Errorf("Classify mutated audit-report.md on disk (%d bytes before, %d after) — "+
				"ship SHA-binds this artifact (ship/audit.go:83), so the cap must warn, never truncate",
				len(want), len(after))
		}
	})
}
