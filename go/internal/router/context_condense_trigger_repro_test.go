package router

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestDigest_RunDirArtifactBytes_ReportsUnderThresholdWithLargePromptsDoesNotTrigger(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFile(t, ws, "scout-report.md", strings.Repeat("a", 10000))
	writeFile(t, ws, "handoff-scout.json", strings.Repeat("b", 5000))
	writeFile(t, ws, "advisor-prompt-plan.txt", strings.Repeat("c", 45000))
	writeFile(t, ws, "router-prompt.txt", strings.Repeat("d", 50000))
	writeFile(t, ws, "signals.ndjson", strings.Repeat("e", 60000))
	writeFile(t, ws, "router-stdout.log", strings.Repeat("f", 10000))

	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}

	if sig.RunDir.ArtifactBytes != 15000 {
		t.Fatalf("RunDir.ArtifactBytes = %d, want 15000 (only report artifacts)", sig.RunDir.ArtifactBytes)
	}

	cond := config.Condition{Field: "run_dir.artifact_bytes", Op: "gt", Value: float64(102400)}
	if evalCondition(sig, cond) {
		t.Fatalf("run_dir.artifact_bytes > 102400 evaluated to true for 15KB reports and 165KB prompts/logs; want false")
	}
}

func TestDigest_RunDirArtifactBytes_PreservesTriggerWhenReportArtifactsExceedThreshold(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFile(t, ws, "scout-report.md", strings.Repeat("a", 60000))
	writeFile(t, ws, "audit-report.md", strings.Repeat("b", 50000))

	sig, err := Digest(ws, []string{"scout", "audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}

	if sig.RunDir.ArtifactBytes != 110000 {
		t.Fatalf("RunDir.ArtifactBytes = %d, want 110000", sig.RunDir.ArtifactBytes)
	}

	cond := config.Condition{Field: "run_dir.artifact_bytes", Op: "gt", Value: float64(102400)}
	if !evalCondition(sig, cond) {
		t.Fatalf("run_dir.artifact_bytes > 102400 evaluated to false for 110KB reports; want true")
	}
}
