package audit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestProductionPath_TheCascadeShadowSignalReachesTheCenter(t *testing.T) {
	req := productionRunFixture(t, 9)
	withFakeRunner(t, func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, nil
	})
	policyJSON := `{"audit_cascade":{"stage":"shadow","risk_paths":["go/internal/core/"],"max_changed_files":5,"escalate_verdicts":["FAIL","WARN"]}}`
	if err := os.WriteFile(filepath.Join(req.ProjectRoot, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	rec, events := recordingCenter()
	phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: passReport}, fakePromptsFS("body"),
		config.StageOff, false, nil, WithSignals(func() *signalcenter.Center { return rec }),
		func(c *Config) { c.BeginPredicateEvidence = nil })

	if _, err := phase.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	rec.Flush()

	counts := make(map[signalcenter.Code]int)
	for _, e := range *events {
		counts[e.Code]++
	}
	if counts[signalcenter.CodeAuditCascadeShadow] != 1 {
		t.Errorf("AUDIT_CASCADE_SHADOW events = %d, want 1: the emitter is lost between WithSignals and recordCascadeShadow", counts[signalcenter.CodeAuditCascadeShadow])
	}
}
