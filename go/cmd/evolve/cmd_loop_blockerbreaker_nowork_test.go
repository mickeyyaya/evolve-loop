package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const noWorkHandoff = `[{"phase":"triage","verdict":"SKIPPED","abort_reason":"triage-empty-commitment"}]`

func TestBlockerBreakerHalt_NoWorkHandoffsPastThePolicyCeilingHaltWithAnIncident(t *testing.T) {
	t.Parallel()
	c, got, _ := recordingLoopSignals()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"failure_policy":{"thresholds":{"no_work_handoff_halt_ceiling":3}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, timing := range []string{noWorkHandoff, shipped, noWorkHandoff, shipped, noWorkHandoff} {
		writeCycleTiming(t, evolveDir, 11+i, timing)
	}

	rc, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard, c)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("three no-work hand-offs at a policy ceiling of three must halt, ships between them or not: halted=%v rc=%d", halted, rc)
	}
	halts := loopHalts(*got)
	if len(halts) != 1 {
		t.Fatalf("one halt is ONE INCIDENT: %+v", halts)
	}
	if e := halts[0]; e.Code != CodeLoopPipelineBlockerHalt || e.Cycle != 15 || e.Fields["rule"] != "no-work-handoffs" || e.Fields["category"] != "no-work-handoff" {
		t.Errorf("the INCIDENT names the rule, the category and the last hand-off cycle: %+v", e)
	}
	entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox"))
	found := false
	for _, e := range entries {
		found = found || strings.Contains(e.Name(), "pipeline-defect-no-work-handoff")
	}
	if !found {
		t.Error("the halt files its P0 pipeline-defect item")
	}
}

func TestBlockerBreakerHalt_NoWorkHandoffsUnderTheCompiledCeilingDoNotHalt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for i, timing := range []string{noWorkHandoff, shipped, noWorkHandoff, shipped, noWorkHandoff, shipped, noWorkHandoff, noWorkHandoff} {
		writeCycleTiming(t, evolveDir, 11+i, timing)
	}
	if _, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard, nil); halted {
		t.Fatal("five hand-offs under the compiled ceiling of six halted")
	}
	writeCycleTiming(t, evolveDir, 19, noWorkHandoff)
	if _, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard, nil); !halted {
		t.Fatal("the sixth hand-off in the batch must halt at the compiled ceiling")
	}
}
