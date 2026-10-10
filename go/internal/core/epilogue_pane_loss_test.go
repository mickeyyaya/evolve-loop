package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func artifactTimeoutErr(cause, reason string) error {
	return fmt.Errorf("build: bridge: launch exit=81: artifact-timeout: cause=%s reason=%q phase=build cycle=1853 driver=claude-tmux artifact=\"build-report.md\" waited=2400s interval=1200s extends_used=1 max_extends=6 last_review=pause liveness=idle progressed=false busy=false transient=false detector_error=\"\": %w", cause, reason, ErrArtifactTimeout)
}

func TestEpilogueCauseSuffix_APaneLossNeverMergesWithAReviewPauseTimeout(t *testing.T) {
	lost := epilogueCauseSuffix(artifactTimeoutErr("pane_lost", "tmux session s is gone"))
	paused := epilogueCauseSuffix(artifactTimeoutErr("review_pause", "no output during the last 1200s interval"))

	if !strings.Contains(lost, "pane_lost") {
		t.Errorf("sealed reason suffix %q, want it to name pane_lost: the seal and the fingerprint must say the pane was lost", lost)
	}
	phase := "build"
	if a, b := fingerprint(phase, "infra-error", []string{abnormalEpilogueReason(phase) + lost}), fingerprint(phase, "infra-error", []string{abnormalEpilogueReason(phase) + paused}); a == b {
		t.Errorf("a pane loss and a review_pause timeout share fingerprint %s: a lost pane must never merge with an ordinary stall", a)
	}
}

func TestAbnormalEpilogue_ADossierOrStateWriteFaultStillLeavesTheDigestAndTheAbortedPhase(t *testing.T) {
	cr, root := epilogueRun(t, false)
	if err := os.WriteFile(filepath.Join(root, "knowledge-base"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cr.o.storage = &fakeStorage{failOnWriteCS: true}

	cr.abnormalEpilogue(artifactTimeoutErr("pane_lost", "tmux session s is gone"))

	if _, err := os.Stat(filepath.Join(cr.cs.WorkspacePath, "failure-digest.json")); err != nil {
		t.Fatalf("a dossier or state fault must not cost the failure digest: %v", err)
	}
	if cr.cs.Phase != "aborted" {
		t.Errorf("phase=%q, want aborted in memory even when the state write fails", cr.cs.Phase)
	}
}

func TestRunCycle_ALedgerFaultDoesNotLoseTheQuotaPause(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { return nil }
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(85), failUntil: 99}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{failOnAppend: true}, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the quota pause: a ledger fault only warns", err)
	}
}
