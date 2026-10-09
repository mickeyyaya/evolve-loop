package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const sizeMarker = "audit-report.md size"

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

func sizeDiags(diags []core.Diagnostic) []core.Diagnostic {
	var out []core.Diagnostic
	for _, d := range diags {
		if strings.Contains(d.Message, sizeMarker) {
			out = append(out, d)
		}
	}
	return out
}

func TestAuditReportLength(t *testing.T) {
	capBytes := auditReportMaxBytes

	t.Run("cap_value_sane", func(t *testing.T) {
		if capBytes < 8*1024 || capBytes > 1<<20 {
			t.Errorf("auditReportMaxBytes=%d out of sane range [8KiB, 1MiB] — a cap outside it "+
				"either fires on every report or never fires at all", capBytes)
		}
	})

	t.Run("under_cap_silent", func(t *testing.T) {
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
				"report through cyclestate.ErrorMessages into AuditFailReasons, converting a verbose "+
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
		verdict, _, _ := classifySized(t, reportOfSize(t, capBytes*2))
		if verdict != core.VerdictPASS {
			t.Errorf("verdict=%q, want PASS — the size check is diagnostic-only and must never "+
				"block a ship on verbosity alone", verdict)
		}
	})

	t.Run("over_cap_does_not_mutate_artifact", func(t *testing.T) {
		want := reportOfSize(t, capBytes*2)
		_, _, after := classifySized(t, want)
		if after != want {
			t.Errorf("Classify mutated audit-report.md on disk (%d bytes before, %d after) — "+
				"ship SHA-binds this artifact (ship/audit.go:83), so the cap must warn, never truncate",
				len(want), len(after))
		}
	})
}
