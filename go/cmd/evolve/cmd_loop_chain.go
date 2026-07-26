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
// under .evolve/inbox that actually PARSE as an inbox item. Lifecycle
// subdirectories (processing/, processed/, consumed/, quarantine/, …) and
// non-json files are not pending work and stay invisible. A root-level `*.json`
// that is not an item (truncated, 0-byte, a top-level array, no `id`) is
// returned by NAME in skipped rather than counted: counting it would pin
// pending>0 permanently and burn the chain to max_batches consuming nothing,
// and swallowing it would hide a real item lost to a typo. A MISSING inbox is
// legitimately zero pending; any other read error is returned so the caller
// stops loudly rather than chaining on a guess.
//
// The skip list is RETURNED, not printed, so this stays a pure function and the
// operator diagnostic lives at the call site (runLoopChain).
func inboxPendingCount(evolveDir string) (int, []string, error) {
	dir := filepath.Join(evolveDir, "inbox")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("read inbox: %w", err)
	}
	n := 0
	var skipped []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if isInboxItemFile(filepath.Join(dir, e.Name())) {
			n++
			continue
		}
		skipped = append(skipped, e.Name())
	}
	return n, skipped, nil
}

// isInboxItemFile reports whether a root-level `*.json` is a real inbox item: a
// JSON OBJECT carrying a non-empty `id` (the field every inbox consumer keys
// on). Deliberately shallow — the chain only needs "is this pending work", not
// full schema validation, and a stricter check here would silently drop items
// the real consumers accept.
func isInboxItemFile(path string) bool {
	buf, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		return false
	}
	id, _ := doc["id"].(string)
	return strings.TrimSpace(id) != ""
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
// inbox AFTER at least one batch (the success exit), then the runaway cap.
//
// The `n > 0` scope on the drained-inbox exit is the min-one-batch guarantee:
// a drained inbox is a CONTINUE condition ("we finished the work"), not a START
// condition ("there was never any work"). Without it, opting into chaining is
// silently WEAKER than the pre-chain contract, where `evolve loop` always ran
// one batch — a chain launched against an already-drained inbox returned rc=0
// having run zero cycles. The allowance is deliberately narrow: it is outranked
// by the brake (an explicit operator instruction is never overridden by a
// contract default) and it never widens the cap, which stays an exact ceiling
// (a non-positive cap still runs nothing — no cap+1).
func chainStartDecision(n, maxBatches, inboxPending int, brake bool) (reason string, stop bool) {
	switch {
	case brake:
		return "chain_operator_brake", true
	case inboxPending == 0 && n > 0:
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
		pending, skipped, err := inboxPendingCount(cfg.EvolveDir)
		if err != nil {
			fmt.Fprintf(stderr, "[chain] cannot read the inbox (%v) — stopping the chain rather than looping blind\n", err)
			res.StopReason = "chain_inbox_unreadable"
			exit = 2
			break
		}
		// Fail loudly: a root-level *.json that is not an item is usually a real
		// todo lost to a typo. Named BEFORE the stop decision so the operator
		// sees it even on the boundary that ends the chain.
		for _, name := range skipped {
			fmt.Fprintf(stderr, "[chain] skipping .evolve/inbox/%s — not a valid inbox item (no parseable object with an `id`); it is NOT counted as pending work\n", name)
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
