package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestCycleRunQuotaExit_AFleetLaneReleasesItsClaimWithTheQuotaDeferralNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	claim, pending := claimedInboxItem(t, root, 2104, "deferred-fleet-lane-releases-its-claim")
	lifecycle := newFakeLedger()
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(2104, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, lifecycle, nil)

	if code != fleet.ExitDeferred {
		t.Errorf("exit = %d, want fleet.ExitDeferred=%d", code, fleet.ExitDeferred)
	}
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the deferred lane still holds its claim, so the planner never offers the item again: %v", err)
	}
	body, err := os.ReadFile(pending)
	if err != nil {
		t.Fatalf("the item is not back in the queue: %v", err)
	}
	if strings.Contains(string(body), "failure_count") {
		t.Errorf("a quota deferral charged the task: %s", body)
	}
	if !lifecycleNotes(lifecycle, laneQuotaDeferredReason) {
		t.Errorf("no lifecycle record names the quota deferral %q: %+v", laneQuotaDeferredReason, lifecycle.lifecycle)
	}
	if !strings.Contains(stderr.String(), "claims are pending again") {
		t.Errorf("stderr does not tell the operator the lane needs no resume: %q", stderr.String())
	}
}

func TestCycleRunQuotaExit_ANonFleetCycleKeepsItsClaimForResume(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	claim, pending := claimedInboxItem(t, root, 2105, "sequential-resume-keeps-claim")
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(2105, root, filepath.Join(root, ".evolve"), false, &stdout, &stderr, newFakeLedger(), nil)

	if code != fleet.ExitDeferred {
		t.Errorf("exit = %d, want fleet.ExitDeferred=%d", code, fleet.ExitDeferred)
	}
	if _, err := os.Stat(claim); err != nil {
		t.Errorf("a resumable pause lost its claim: %v", err)
	}
	if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a resumable pause put its item back in the queue: %v", err)
	}
	if !strings.Contains(stderr.String(), "evolve loop --resume") {
		t.Errorf("stderr does not name the resume path: %q", stderr.String())
	}
}

func TestCycleRunQuotaExit_AReleaseErrorIsReportedAndTheLaneStillDefers(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory, so the release cannot fail")
	}
	root := t.TempDir()
	processing := filepath.Join(root, ".evolve", "inbox", "processing")
	if err := os.MkdirAll(processing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(processing, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(processing, 0o755) })
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(2106, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, newFakeLedger(), nil)

	if code != fleet.ExitDeferred {
		t.Errorf("exit = %d, want fleet.ExitDeferred=%d", code, fleet.ExitDeferred)
	}
	if !strings.Contains(stderr.String(), "could not release cycle 2106 claims") {
		t.Errorf("the release error was swallowed: %q", stderr.String())
	}
}

func TestCycleRunRoot_AFleetLaneOnAQuotaWallReleasesItsClaim(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	var claim, pending string
	rc, fake, _, stderr := runCycleRunWith(t, cycleRunQuotaRunner{}, false, func(root string) {
		claim, pending = claimedInboxItem(t, root, 1, "walled-lane-item")
	})
	if rc != fleet.ExitDeferred {
		t.Errorf("rc = %d, want fleet.ExitDeferred; stderr=%s", rc, stderr)
	}
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the walled fleet lane still holds its claim: %v; stderr=%s", err, stderr)
	}
	if _, err := os.Stat(pending); err != nil {
		t.Errorf("the walled fleet lane's item is not pending again: %v", err)
	}
	if !lifecycleNotes(fake, laneQuotaDeferredReason) {
		t.Errorf("no lifecycle record names the quota deferral: %+v", fake.lifecycle)
	}
}

func lifecycleNotes(l *fakeLedgerNoAppend, reason string) bool {
	for _, r := range l.lifecycle {
		if strings.Contains(r.Message, reason) {
			return true
		}
	}
	return false
}
