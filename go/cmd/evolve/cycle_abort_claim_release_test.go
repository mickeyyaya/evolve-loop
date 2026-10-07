package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

var errPostReviewAbort = errors.New("refresh Build explanation after tdd: revalidate builder explanation handoff before sealing: unreadable host snapshot")

func seedClaimedCycle(t *testing.T, root string) string {
	t.Helper()
	evolveDir, _ := seedFailedCycleInbox(t, root, "worked", 7)
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "worked", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	return evolveDir
}

func assertClaimsReleased(t *testing.T, evolveDir string, stderr *bytes.Buffer) {
	t.Helper()
	if entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox", "processing", "cycle-7")); len(entries) != 0 {
		t.Errorf("a batch-fatal abort strands the cycle's claim: %d file(s) left in processing/cycle-7; stderr=%s", len(entries), stderr.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox")); !hasInboxItem(entries) {
		t.Errorf("the released claim is back at the inbox root; stderr=%s", stderr.String())
	}
}

func hasInboxItem(entries []os.DirEntry) bool {
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			return true
		}
	}
	return false
}

func TestCycleRunErrorExit_ABatchFatalAbortReleasesTheCyclesClaims(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedClaimedCycle(t, root)
	var stderr bytes.Buffer

	rc := cycleRunErrorExit(errPostReviewAbort, 7, root, evolveDir, &stderr, newFakeLedger(), nil)

	if rc != 1 {
		t.Errorf("a batch-fatal abort still exits 1: rc=%d", rc)
	}
	assertClaimsReleased(t, evolveDir, &stderr)
}

func TestHandleCycleError_ABatchFatalAbortReleasesClaimsAndStillStopsTheBatch(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedClaimedCycle(t, root)
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: newFakeLedger()}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}

	d := b.handleCycleError(core.CycleResult{Cycle: 7}, errPostReviewAbort)

	if d.flow != batchStopIterations || b.result.StopReason != "error" {
		t.Errorf("a batch-fatal abort still stops the batch: flow=%v stop=%q", d.flow, b.result.StopReason)
	}
	assertClaimsReleased(t, evolveDir, &stderr)
}

func TestHandleCycleError_ABatchFatalAbortWalksTheLifecycleThroughTheRootLedger(t *testing.T) {
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
	fake := newFakeLedger()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: fake}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}

	b.handleCycleError(core.CycleResult{Cycle: 7}, errPostReviewAbort)

	assertLifecycleWentThroughTheRootLedger(t, evolveDir, fake)
}

func TestHandleCycleError_AnUnwrappedQuotaWallKeepsItsClaimsAndPauses(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedClaimedCycle(t, root)
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: newFakeLedger()}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}

	d := b.handleCycleError(core.CycleResult{Cycle: 7}, fmt.Errorf("phase build: %w", core.ErrAllFamiliesExhausted))

	if d.flow != batchReturn || d.exitCode != 5 {
		t.Errorf("a quota wall pauses the batch resumably: flow=%v exit=%d", d.flow, d.exitCode)
	}
	if entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox", "processing", "cycle-7")); len(entries) != 1 {
		t.Errorf("a resumable quota wall keeps its claim: %d file(s) in processing/cycle-7; stderr=%s", len(entries), stderr.String())
	}
}

func TestCycleErrorRoots_AnErrorBeforeAllocationWalksNoCycle(t *testing.T) {
	early := errors.New("acquire lock: another orchestrator holds it")
	for name, exit := range map[string]func(root, evolveDir string, fake *fakeLedgerNoAppend, stderr *bytes.Buffer){
		"the cycle-run root": func(root, evolveDir string, fake *fakeLedgerNoAppend, stderr *bytes.Buffer) {
			cycleRunErrorExit(early, 0, root, evolveDir, stderr, fake, nil)
		},
		"the sequential root": func(root, evolveDir string, fake *fakeLedgerNoAppend, stderr *bytes.Buffer) {
			var stdout bytes.Buffer
			b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: fake}, result: &loopResult{}, stdout: &stdout, stderr: stderr}
			b.handleCycleError(core.CycleResult{}, early)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			evolveDir := seedClaimedCycle(t, root)
			fake := newFakeLedger()
			var stderr bytes.Buffer

			exit(root, evolveDir, fake, &stderr)

			if strings.Contains(stderr.String(), "cycle-0") || len(fake.lifecycle) != 0 {
				t.Errorf("an error before a cycle exists walks nothing: lifecycle lines=%d stderr=%q", len(fake.lifecycle), stderr.String())
			}
			if entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox", "processing", "cycle-7")); len(entries) != 1 {
				t.Errorf("a peer cycle's claim is untouched: %d file(s)", len(entries))
			}
		})
	}
}
