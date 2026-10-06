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
)

func claimedInboxItem(t *testing.T, root string, cycle int, id string) (claim, pending string) {
	t.Helper()
	name := id + ".json"
	claim = filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+itoa(cycle), name)
	if err := os.MkdirAll(filepath.Dir(claim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claim, []byte(`{"id":"`+id+`","weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return claim, filepath.Join(root, ".evolve", "inbox", name)
}

func TestCycleRunErrorExit_ALaneDeferralReleasesItsClaimsUnbumpedAndExitsDeferred(t *testing.T) {
	root := t.TempDir()
	claim, pending := claimedInboxItem(t, root, 1806, "inboxbatch-utf8-and-resolution")
	var stderr bytes.Buffer
	deferral := &core.LaneDeferral{Cycle: 1806, Cause: errors.New("git fetch origin main: cannot lock ref")}

	code := cycleRunErrorExit(deferral, 0, root, filepath.Join(root, ".evolve"), &stderr, nil, nil)

	if code != fleet.ExitDeferred {
		t.Errorf("exit = %d, want fleet.ExitDeferred=%d so the wave counts the lane deferred, not failed", code, fleet.ExitDeferred)
	}
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the claim is still held: %v", err)
	}
	body, err := os.ReadFile(pending)
	if err != nil {
		t.Fatalf("the item is not back in the queue: %v", err)
	}
	if strings.Contains(string(body), "failure_count") {
		t.Errorf("an infrastructure deferral charged the task: %s", body)
	}
	if !strings.Contains(stderr.String(), "fleet lane deferred (cycle 1806)") {
		t.Errorf("stderr does not name the deferral: %q", stderr.String())
	}
}

func TestCycleRunErrorExit_AnyOtherErrorStillExitsOne(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	if code := cycleRunErrorExit(errors.New("allocate cycle: boom"), 0, root, filepath.Join(root, ".evolve"), &stderr, nil, nil); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}
