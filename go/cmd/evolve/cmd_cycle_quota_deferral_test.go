package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestCycleRunQuotaExit_AFleetLaneReleasesItsClaimWithTheQuotaDeferralNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWallCheckpoint(t, filepath.Join(root, ".evolve"), 2104, quotaEnd)
	claim, pending := claimedInboxItem(t, root, 2104, "deferred-fleet-lane-releases-its-claim")
	lifecycle := newFakeLedger()
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(core.CycleResult{Cycle: 2104}, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, lifecycle, nil)

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
	if !lifecycleNotes(lifecycle, laneWallDeferredReason) {
		t.Errorf("no lifecycle record names the quota deferral %q: %+v", laneWallDeferredReason, lifecycle.lifecycle)
	}
	if !strings.Contains(stderr.String(), "claims are pending again") {
		t.Errorf("stderr does not tell the operator the lane needs no resume: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"quota-pause"`) {
		t.Errorf("the lane's stdout envelope does not carry the quota-pause stop reason: %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "evolve loop --resume") {
		t.Errorf("a released lane must not also steer the operator to a resume: %q", stderr.String())
	}
}

func TestCycleRunQuotaExit_ANonFleetCycleKeepsItsClaimForResume(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	claim, pending := claimedInboxItem(t, root, 2105, "sequential-resume-keeps-claim")
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(core.CycleResult{Cycle: 2105}, root, filepath.Join(root, ".evolve"), false, &stdout, &stderr, newFakeLedger(), nil)

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

	code := cycleRunQuotaExit(core.CycleResult{Cycle: 2106}, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, newFakeLedger(), nil)

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
	if !lifecycleNotes(fake, laneWallDeferredReason) {
		t.Errorf("no lifecycle record names the quota deferral: %+v", fake.lifecycle)
	}
}

func TestCycleRunRoot_ACycleRunOutsideAFleetKeepsItsClaimOnAQuotaWall(t *testing.T) {
	for _, fleetValue := range []string{"", "0"} {
		t.Run("EVOLVE_FLEET="+fleetValue, func(t *testing.T) {
			t.Setenv(ipcenv.FleetKey, fleetValue)
			var claim, pending string
			rc, fake, _, stderr := runCycleRunWith(t, cycleRunQuotaRunner{}, false, func(root string) {
				claim, pending = claimedInboxItem(t, root, 1, "manual-cycle-run-item")
			})
			if rc != fleet.ExitDeferred {
				t.Errorf("rc = %d, want fleet.ExitDeferred; stderr=%s", rc, stderr)
			}
			if _, err := os.Stat(claim); err != nil {
				t.Errorf("a manual cycle run lost the claim that evolve loop --resume needs: %v; stderr=%s", err, stderr)
			}
			if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a manual cycle run put its item back in the queue: %v", err)
			}
			if lifecycleNotes(fake, laneWallDeferredReason) {
				t.Errorf("a manual cycle run wrote a quota-deferral release: %+v", fake.lifecycle)
			}
		})
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

func writeCorruptWallCheckpoint(t *testing.T, evolveDir string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"cycle_id":9,"checkpoint":{"enabled":true,"reason":"quota-likely","quotaWall":"garbage"}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCycleRunQuotaExit_AFleetLaneNamesACorruptWallFieldAndNotNoRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeCorruptWallCheckpoint(t, evolveDir)
	var stdout, stderr bytes.Buffer

	cycleRunQuotaExit(core.CycleResult{Cycle: 9}, root, evolveDir, true, &stdout, &stderr, newFakeLedger(), nil)

	if strings.Contains(stderr.String(), "no cause was recorded") || !strings.Contains(stderr.String(), "the recorded cause could not be read") {
		t.Errorf("a corrupt wall field must read as unreadable, not as no record: %s", stderr.String())
	}
}

func TestChainQuotaPause_NamesACorruptWallFieldAndNotNoRecord(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeCorruptWallCheckpoint(t, evolveDir)

	got, _ := chainQuotaPause(evolveDir)

	if !strings.HasPrefix(got.Clauses, "the recorded cause could not be read") {
		t.Errorf("clauses = %q, want the decode error named", got.Clauses)
	}
}
