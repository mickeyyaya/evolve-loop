package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// WS3-S2 (ADR-0052): recordPhasePlan must hash-bind the WS3-S1 capture
// artifacts (advisor-prompt-plan.txt / advisor-response-plan.txt) into the
// ledger, so a post-hoc mutation of a persisted routing prompt/response is
// detectable. The binding reuses the existing ArtifactPath+ArtifactSHA256
// shape, one bound entry per artifact; the ledger's hash chain then carries
// the tamper-evidence.

func sha256Hex(t *testing.T, path string) string {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

func TestRecordPhasePlan_BindsPromptResponseSHAs(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	promptPath := filepath.Join(ws, "advisor-prompt-plan.txt")
	respPath := filepath.Join(ws, "advisor-response-plan.txt")
	if err := os.WriteFile(promptPath, []byte("PLAN PROMPT BODY — redacted copy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(respPath, []byte(`[{"phase":"scout","run":true,"justification":"x"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
	cs := CycleState{WorkspacePath: ws}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true, Justification: "x"}}}

	o.recordPhasePlan(context.Background(), 42, cs, plan, nil)

	// Collect the two capture-binding entries by kind.
	bound := map[string]LedgerEntry{}
	for _, e := range led.entries {
		switch e.Kind {
		case "advisor_prompt", "advisor_response":
			bound[e.Kind] = e
		}
	}
	if len(bound) != 2 {
		t.Fatalf("want one bound entry each for advisor_prompt + advisor_response, got %d (%+v)", len(bound), led.entries)
	}

	// 1. Bound SHA == an INDEPENDENT recompute of the unmodified file.
	if got, want := bound["advisor_prompt"].ArtifactSHA256, sha256Hex(t, promptPath); got != want {
		t.Errorf("prompt bound sha = %q, want independently-recomputed %q", got, want)
	}
	if got, want := bound["advisor_response"].ArtifactSHA256, sha256Hex(t, respPath); got != want {
		t.Errorf("response bound sha = %q, want independently-recomputed %q", got, want)
	}
	// The entry must point at the artifact it binds (forensics + WS3-S5 replay).
	if bound["advisor_response"].ArtifactPath != respPath {
		t.Errorf("response entry ArtifactPath = %q, want %q", bound["advisor_response"].ArtifactPath, respPath)
	}

	// 2. Tamper-evidence: mutate one byte of the prompt; its recomputed sha must
	// now DIVERGE from the bound sha (the binding still attests the original).
	if err := os.WriteFile(promptPath, []byte("PLAN PROMPT BODY — redacted copyX"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sha256Hex(t, promptPath) == bound["advisor_prompt"].ArtifactSHA256 {
		t.Error("a mutated prompt artifact must NOT match the bound sha — tamper-evidence failed")
	}
	// 3. The untouched response still matches its bound sha (no false positive).
	if sha256Hex(t, respPath) != bound["advisor_response"].ArtifactSHA256 {
		t.Error("an unmodified response artifact must still match its bound sha")
	}
}

// TestRecordPhasePlan_NoCaptureNoBinding proves the binding is fail-open: when
// the WS3-S1 capture is absent (capture write failed, or pre-WS3 cycle), no
// advisor_prompt/advisor_response entries are appended — never a binding to a
// missing file.
func TestRecordPhasePlan_NoCaptureNoBinding(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
	cs := CycleState{WorkspacePath: t.TempDir()} // empty — no capture artifacts
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}

	o.recordPhasePlan(context.Background(), 7, cs, plan, nil)

	for _, e := range led.entries {
		if e.Kind == "advisor_prompt" || e.Kind == "advisor_response" {
			t.Errorf("no capture present ⇒ must not append %s binding (got %+v)", e.Kind, e)
		}
	}
}
