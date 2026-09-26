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

func TestReviewerReview_SalvageSurfacesSummaryLine(t *testing.T) {
	// A sole, fenced, single-candidate bad_verdict that re-verifies clean: the one shape salvage acts on.
	const soleFencedPass = "## Verdict\n" +
		"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", soleFencedPass)
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}

	var logged []string
	// PhaseIO=enforce makes the sentinel strictly required, so the fenced payload lands as a bad_verdict.
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
	if want := "Salvaged verdicts: 1 (fenced-json=1)"; !strings.Contains(hit, want) {
		t.Errorf("summary must render the sidecar's real count and pattern breakdown\n want substring: %q\n got line:       %q", want, hit)
	}
}
