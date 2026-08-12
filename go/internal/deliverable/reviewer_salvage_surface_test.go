package deliverable

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// reviewer_salvage_surface_test.go — caller proof for SalvageSummaryLine.
//
// cycle-1392 audit LOW dd17d798e155571ecd91be63e14050ab6: the renderer was
// exported, documented in README §8 as "surfaced", and called from nothing but
// tests. A seam whose only caller is a test is dead code, and a doc that
// promises it is an overclaim. This test pins the OPPOSITE: the line reaches an
// operator through the real production entry point, `Reviewer.Review` — not by
// calling the renderer directly (that is TestSalvageSummaryLine_Surfaces...'s
// job) but by driving a salvageable deliverable through the gate and asserting
// the gate's own log stream carried the summary.
func TestReviewerReview_SalvageSurfacesSummaryLine(t *testing.T) {
	// Sole bad_verdict, fenced-json shape, one candidate, repairs to a payload
	// that re-verifies clean — the one case salvage is allowed to act on.
	const soleFencedPass = "## Verdict\n" +
		"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", soleFencedPass)
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}

	var logged []string
	// PhaseIO=enforce is what makes the sentinel comment strictly required, so
	// the displayable fenced payload lands as a bad_verdict instead of parsing
	// clean — the same stage pairing the acs wiring predicates drive.
	r := newTestReviewerPhaseIO(config.StageEnforce, config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	r.logf = func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	}

	got := r.Review(context.Background(), reviewInput("audit", ws, projectRoot))
	if !got.Approve {
		t.Fatalf("precondition: a sole recoverable bad_verdict must salvage to Approve=true; got block (%s)", got.Reason)
	}

	var hit string
	for _, line := range logged {
		if strings.Contains(line, "Salvaged verdicts:") {
			hit = line
		}
	}
	if hit == "" {
		t.Fatalf("dead export: Review salvaged the deliverable but emitted no salvage summary to the operator log — README §8 promises every coercion is \"logged + surfaced\" (cycle-1392 LOW dd17d798e155571ecd91be63e14050ab6). Logged lines: %q", logged)
	}
	// Single-sourced from the sidecar recordSalvageApplied just wrote, so the
	// count includes THIS salvage and no second counter exists to drift.
	if want := "Salvaged verdicts: 1 (fenced-json=1)"; !strings.Contains(hit, want) {
		t.Errorf("summary must render the sidecar's real count and pattern breakdown\n want substring: %q\n got line:       %q", want, hit)
	}
}
