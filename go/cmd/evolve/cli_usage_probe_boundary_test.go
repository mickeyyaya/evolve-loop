package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func installPreWaveUsagePane(t *testing.T, probe func(context.Context, string) (string, error)) {
	t.Helper()
	prev := preWaveUsagePaneFn
	preWaveUsagePaneFn = func(*bridge.ControllerFactory) func(context.Context, string) (string, error) { return probe }
	t.Cleanup(func() { preWaveUsagePaneFn = prev })
}

func preWaveProbeRoot(t *testing.T, policy string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func countWarns(log string) int {
	n := 0
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "WARN") {
			n++
		}
	}
	return n
}

func TestRunPreWaveProbes_AFakeProbeThatReadsAgyExhaustedBenchesAgyBeforeTheWaveAdmitsALane(t *testing.T) {
	usageProbeFakePATH(t, "agy")
	root, evolveDir := preWaveProbeRoot(t, `{"cli_health":{"proactive_probe":true}}`)
	pane := usageFixture(t, "agy_usage_gemini_drained.txt")
	installPreWaveUsagePane(t, func(context.Context, string) (string, error) { return pane, nil })
	var log bytes.Buffer

	if err := runPreWaveProbes(context.Background(), root, evolveDir, &log); err != nil {
		t.Fatalf("runPreWaveProbes: %v\n%s", err, log.String())
	}

	bench, ok := clihealth.NewStore(root, nil).Active()["agy"]
	if !ok || bench.Reason != clihealth.UsageProbePattern {
		t.Fatalf("the agy bench = %+v (present %v); the pre-wave step must write it before any lane boots\n%s", bench, ok, log.String())
	}
}

func TestRunPreWaveProbes_AFailedProbeBenchesNothingAndLogsOneWarn(t *testing.T) {
	usageProbeFakePATH(t, "agy")
	root, evolveDir := preWaveProbeRoot(t, `{"cli_health":{"proactive_probe":true}}`)
	installPreWaveUsagePane(t, func(context.Context, string) (string, error) { return "", errors.New("boot timed out") })
	var log bytes.Buffer

	if err := runPreWaveProbes(context.Background(), root, evolveDir, &log); err != nil {
		t.Fatalf("runPreWaveProbes: %v", err)
	}

	if active := clihealth.NewStore(root, nil).Active(); len(active) != 0 {
		t.Errorf("a failed probe benched %v", active)
	}
	if n := countWarns(log.String()); n != 1 || !strings.Contains(log.String(), "WARN agy: usage probe failed: boot timed out") {
		t.Errorf("want one WARN naming agy and its cause, got %d:\n%s", n, log.String())
	}
}

func TestRunPreWaveProbes_TheUsageEvidenceTimeoutBoundsAProbeThatNeverAnswers(t *testing.T) {
	usageProbeFakePATH(t, "agy")
	root, evolveDir := preWaveProbeRoot(t, `{"cli_health":{"proactive_probe":true,"usage_evidence_timeout_s":1}}`)
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	installPreWaveUsagePane(t, func(context.Context, string) (string, error) { <-block; return "", nil })
	var log bytes.Buffer
	start := time.Now()

	if err := runPreWaveProbes(context.Background(), root, evolveDir, &log); err != nil {
		t.Fatalf("runPreWaveProbes: %v", err)
	}

	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("the pre-wave step took %s; the 1 s policy bound must end it", took)
	}
	if n := countWarns(log.String()); n != 1 || !strings.Contains(log.String(), "WARN agy: no usage read within 1s; nothing benched") {
		t.Errorf("want one WARN naming the 1s bound, got %d:\n%s", n, log.String())
	}
	if active := clihealth.NewStore(root, nil).Active(); len(active) != 0 {
		t.Errorf("a probe that never answered benched %v", active)
	}
}
