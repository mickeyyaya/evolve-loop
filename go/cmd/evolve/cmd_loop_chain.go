// cmd_loop_chain.go — the outer batch-chaining loop (cycle 1075, inbox item
// loop-batch-chaining; standing operator directive of 2026-07-11: lanes keep
// running until the inbox is empty without an operator relaunching
// `evolve loop` at every batch boundary).
//
// Design: runLoopBatch stays the single-batch dispatcher it has always been.
// This file adds a thin loop AROUND it that, at each boundary, decides whether
// another batch may start. It deliberately owns no cycle-level logic — the
// quota wall in particular is NOT re-derived here: the batch already maps
// core.ErrAllFamiliesExhausted (core.allFamiliesQuotaExhausted, the all-85
// attempt sequence) onto the resumable rc=5 QUOTA-PAUSE contract, and the
// chain simply refuses to relaunch into it and defers with the checkpoint's
// reset-time hint.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// chainBrakeFile is the operator brake: `touch .evolve/loop-stop` and the
// chain stops at the next boundary (the in-flight batch is never interrupted —
// use SIGINT for that, which the batch already checkpoints).
const chainBrakeFile = "loop-stop"

// runLoopBatchFn is the test seam for the chain loop: tests substitute a
// scripted batch so the boundary decisions can be exercised without running
// real cycles. nil-free by construction — production is the real batch.
var runLoopBatchFn = runLoopBatch

// chainBatchRecord is one chained batch's boundary observation. FleetCount is
// recorded per batch because fleet width is a hard operator commitment
// (ten_lane_concurrency_standing): a chain that silently narrows lanes from
// batch N to N+1 looks healthy in every other signal.
type chainBatchRecord struct {
	Batch        int `json:"batch"`
	RC           int `json:"rc"`
	FleetCount   int `json:"fleet_count"`
	InboxPending int `json:"inbox_pending"`
}

// chainResult is the machine-readable chain summary, emitted to stdout after
// the per-batch loopResult documents.
type chainResult struct {
	ChainMode  bool               `json:"chain_mode"`
	MaxBatches int                `json:"max_batches"`
	Batches    []chainBatchRecord `json:"batches"`
	StopReason string             `json:"chain_stop_reason"`
}

// loadChainConfig loads .evolve/policy.json and returns the resolved chain
// configuration. Absent or malformed policy falls back to built-in defaults
// (chaining off, positive compiled cap), mirroring loadWorkflowConfig.
func loadChainConfig(evolveDir string) policy.ChainConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.Policy{}.ChainConfig()
	}
	return pol.ChainConfig()
}

// inboxPendingCount counts unclaimed inbox items — the `*.json` files directly
// under .evolve/inbox. Lifecycle subdirectories (processing/, processed/,
// consumed/, quarantine/, …) are not pending work and are skipped. A MISSING
// inbox is legitimately zero pending; any other read error is returned so the
// caller stops loudly rather than chaining on a guess.
func inboxPendingCount(evolveDir string) (int, error) {
	ents, err := os.ReadDir(filepath.Join(evolveDir, "inbox"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read inbox: %w", err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n++
	}
	return n, nil
}

// chainBrakeEngaged reports whether the operator dropped the `.evolve/loop-stop`
// brake file.
func chainBrakeEngaged(evolveDir string) bool {
	_, err := os.Stat(filepath.Join(evolveDir, chainBrakeFile))
	return err == nil
}

// chainStartDecision decides whether batch n (0-based) may start, and names
// the reason when it may not. Pure, so the precedence between the three
// pre-batch stop conditions is testable without running a batch: the operator
// brake outranks everything (it is an explicit instruction), then a drained
// inbox (the success exit), then the runaway cap.
func chainStartDecision(n, maxBatches, inboxPending int, brake bool) (reason string, stop bool) {
	switch {
	case brake:
		return "chain_operator_brake", true
	case inboxPending == 0:
		return "chain_inbox_empty", true
	case n >= maxBatches:
		return "chain_max_batches", true
	}
	return "", false
}

// chainContinueDecision maps a finished batch's exit code onto the chain's
// next move. rc 0 (clean) and rc 3 (batch completed with absorbed
// recoverable/verdict failures) are both "the batch ran to completion" — the
// queue is never halted for them (never_stop_queue_inject_inbox). rc 5 is the
// QUOTA-PAUSE contract the batch derives from core.allFamiliesQuotaExhausted:
// relaunching would only burn the next batch into the same drained families,
// so the chain defers with the checkpoint intact. Every other code is a fatal
// batch outcome (preflight, unfinished cycle, ADR-0072 system-failure halt,
// signal) and propagates unchanged.
func chainContinueDecision(rc int) (reason string, exit int, stop bool) {
	switch rc {
	case 0, 3:
		return "", rc, false
	case 5:
		return "chain_quota_defer", 5, true
	default:
		return "chain_batch_error", rc, true
	}
}

// runLoopChain drives runLoopBatch until a boundary condition stops it. The
// same loopConfig value is handed to EVERY batch — no per-batch re-derivation
// — so fleet width and every other resolved setting are preserved across the
// chain by construction.
func runLoopChain(cfg loopConfig, cc policy.ChainConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	res := chainResult{ChainMode: true, MaxBatches: cc.MaxBatches}
	exit := 0

	for n := 0; ; n++ {
		pending, err := inboxPendingCount(cfg.EvolveDir)
		if err != nil {
			fmt.Fprintf(stderr, "[chain] cannot read the inbox (%v) — stopping the chain rather than looping blind\n", err)
			res.StopReason = "chain_inbox_unreadable"
			exit = 2
			break
		}
		if reason, stop := chainStartDecision(n, cc.MaxBatches, pending, chainBrakeEngaged(cfg.EvolveDir)); stop {
			res.StopReason = reason
			fmt.Fprintf(stderr, "[chain] stopping after %d batch(es): %s (inbox pending=%d, cap=%d)\n",
				len(res.Batches), reason, pending, cc.MaxBatches)
			break
		}

		// Width is read (not re-resolved into cfg) purely to record it: the
		// batch resolves its own fleet block, so an operator widening mid-chain
		// still takes effect — what must never happen is the CHAIN narrowing it.
		width := loadFleetConfig(cfg.EvolveDir).Count
		fmt.Fprintf(stderr, "[chain] batch %d/%d starting — inbox pending=%d, fleet lanes=%d\n",
			n+1, cc.MaxBatches, pending, width)

		rc := runLoopBatchFn(cfg, stdin, stdout, stderr)
		res.Batches = append(res.Batches, chainBatchRecord{Batch: n + 1, RC: rc, FleetCount: width, InboxPending: pending})

		reason, code, stop := chainContinueDecision(rc)
		if !stop {
			continue
		}
		res.StopReason, exit = reason, code
		if reason == "chain_quota_defer" {
			emitChainQuotaDefer(cfg, n+1, stderr)
		} else {
			fmt.Fprintf(stderr, "[chain] batch %d exited rc=%d — stopping the chain (%s)\n", n+1, rc, reason)
		}
		break
	}

	buf, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	return exit
}

// emitChainQuotaDefer prints the deferral notice for a quota-walled batch,
// including the checkpoint's reset-time hint when one was written. The point
// of the chain reading it here is the negative behaviour: it does NOT start
// another batch into families that are already drained.
func emitChainQuotaDefer(cfg loopConfig, batch int, stderr io.Writer) {
	if qp, ok := detectQuotaPause(cfg.EvolveDir); ok {
		fmt.Fprintf(stderr, "[chain] batch %d hit the quota wall (cycle=%d wake-at=%s source=%s) — DEFERRING, not relaunching\n",
			batch, qp.Cycle, qp.WakeAt, qp.Source)
	} else {
		fmt.Fprintf(stderr, "[chain] batch %d hit the quota wall (no checkpoint block on disk) — DEFERRING, not relaunching\n", batch)
	}
	fmt.Fprintln(stderr, "[chain]   the checkpoint is intact; resume when quota resets: evolve loop --resume")
}
