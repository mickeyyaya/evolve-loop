package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func signalStopCoordinator(t *testing.T, firstCycle int) (*loopBatchCoordinator, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for cycle, id := range map[string]string{"cycle-1739": "lane-a", "cycle-1740": "lane-b", "cycle-1600": "older-batch"} {
		dir := filepath.Join(evolveDir, "inbox", "processing", cycle)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:       ctx,
		cfg:       loopConfig{ProjectRoot: root, EvolveDir: evolveDir},
		result:    &loopResult{batchFirstCycle: firstCycle},
		stdout:    &stdout,
		stderr:    &stderr,
		loadGraph: noModuleGraph,
	}
	return b, &stderr
}

func TestInterruptReturn_AFleetStopNamesTheStoppedLanesAndTheirRecovery(t *testing.T) {
	t.Parallel()
	b, stderr := signalStopCoordinator(t, 1739)

	d := b.interruptReturn(2, "")

	out := stderr.String()
	if d.flow != batchReturn || d.exitCode != 130 || b.result.StopReason != "signal" {
		t.Fatalf("decision=%+v stop=%q, want a signal stop with exit 130", d, b.result.StopReason)
	}
	for _, want := range []string{"cycle 1739 holds lane-a", "cycle 1740 holds lane-b", "evolve inbox-mover recover-orphans", "cannot continue them"} {
		if !strings.Contains(out, want) {
			t.Errorf("the stop message lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "older-batch") || strings.Contains(out, "resume with: evolve loop --resume") {
		t.Errorf("the stop message must name only this batch's lanes and never a resume that cannot continue them:\n%s", out)
	}
}

func TestInterruptReturn_WithoutALaneOfThisBatchTheResumeHintStays(t *testing.T) {
	t.Parallel()
	b, stderr := signalStopCoordinator(t, 1800)

	b.interruptReturn(0, "")

	if !strings.Contains(stderr.String(), "resume with: evolve loop --resume") {
		t.Fatalf("a stop that holds no lane claim keeps the plain message:\n%s", stderr.String())
	}
}

func TestInterruptReturn_AnUnknownBatchStartKeepsThePlainMessage(t *testing.T) {
	t.Parallel()
	b, stderr := signalStopCoordinator(t, 0)

	b.interruptReturn(0, "")

	if !strings.Contains(stderr.String(), "resume with: evolve loop --resume") || strings.Contains(stderr.String(), "lane-a") {
		t.Fatalf("without a known batch start no claim can be attributed to the batch:\n%s", stderr.String())
	}
}

func TestInterruptReturn_AnUnreadableClaimSurveyWarnsAndKeepsThePlainMessage(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 directory, so the survey cannot fail here")
	}
	b, stderr := signalStopCoordinator(t, 1739)
	inbox := filepath.Join(b.cfg.EvolveDir, "inbox")
	if err := os.RemoveAll(inbox); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(inbox, "processing", "cycle-1739", "x.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(inbox, "processing", "cycle-1739"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(inbox, "processing", "cycle-1739"), 0o755) })

	b.interruptReturn(0, "")

	if !strings.Contains(stderr.String(), "WARN: cannot list the lane claims of this batch") || !strings.Contains(stderr.String(), "resume with: evolve loop --resume") {
		t.Fatalf("a survey error must warn and keep the plain message:\n%s", stderr.String())
	}
}

func TestRunResumeBatch_RefusesAStaleCheckpointAndListsEveryCheckpointItConsidered(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for name, body := range map[string]string{
		"cycle-1709": `{"cycle_id":1709,"phase":"build","checkpoint":{"enabled":true,"reason":"quota-likely","savedAt":"2026-09-27T10:00:00Z","resumeFromPhase":"build","gitHead":"unknown"}}`,
		"cycle-1738": `{"cycle_id":1738,"phase":"end","started_at":"2026-09-28T09:00:00Z","goal_hash":"wave-26"}`,
		"cycle-1739": `{"cycle_id":1739,"phase":"build","started_at":"2026-09-28T20:00:00Z"}`,
	} {
		dir := filepath.Join(evolveDir, "runs", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var lr loopResult
	var out, errOut bytes.Buffer

	rc := runResumeBatch(context.Background(), loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true}, nil, newFakeLedger(), nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)

	for _, want := range []string{"[resume] considered", "cycle 1709: stale", "evolve cycle reset", "cycle 1739: no resumable checkpoint"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut.String())
		}
	}
	if rc != 2 {
		t.Fatalf("rc=%d, want 2: a stale checkpoint must not be resumed", rc)
	}
}
