// cmd_composition_wiring.go — the composition root's binding of the RUNG 0
// trivial-rebase composition-verdict fast path (merge-concurrency-2026,
// cycle-786/801 built the pieces, cycle-804 wires them).
//
// core cannot import internal/adapters/ledger (ledger already imports core —
// an import cycle), so cmd/evolve — the only package that legally depends on
// both — binds the three Option-injected closures to the real adapters, and
// core stays adapter-agnostic. Every closure fails closed: a missing bound
// audit, an unreadable ledger, a red gate, or a writer error makes
// compositionCarryForward fall back to the pre-existing full re-audit path.
// A bad binding can only keep the fast path dark (status quo), never widen
// what ships.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// compositionOptions binds all three composition closures. Appending these
// in wireOrchestratorDeps is what makes Orchestrator.CompositionFastPathWired
// report true in production (it AND's the three — a partial binding stays
// dark).
func compositionOptions() []core.Option {
	return []core.Option{
		core.WithCompositionVerdictWriter(writeCompositionVerdict),
		core.WithCompositionGateRunner(runComposedGates),
		core.WithCompositionSnapshot(readCompositionSnapshot),
	}
}

// writeCompositionVerdict is a 1:1 pass-through into the real ledger writer —
// core.CompositionVerdictInput mirrors ledger.CompositionVerdictInput
// field-for-field so no validation is duplicated here; the ledger writer
// stays the single source of fail-closed patch-id/gate checks.
func writeCompositionVerdict(ledgerPath string, in core.CompositionVerdictInput) error {
	return ledger.WriteCompositionVerdict(ledgerPath, ledger.CompositionVerdictInput{
		Cycle:        in.Cycle,
		Method:       in.Method,
		LaneAuditRef: in.LaneAuditRef,
		PatchID:      in.PatchID,
		AuditedBase:  in.AuditedBase,
		GitHead:      in.GitHead,
		TreeStateSHA: in.TreeStateSHA,
		GateResults:  in.GateResults,
		AuditedDiff:  in.AuditedDiff,
		ComposedDiff: in.ComposedDiff,
		ArtifactDir:  in.ArtifactDir,
	})
}

// composedGateTargets maps each required composed-tree gate to the Makefile
// target that runs it, so the fast path executes the SAME commands CI and the
// cycle audit run (ADR-0069) — single-sourced through the Makefile, never a
// second gate implementation that could drift. Keys stay in lock-step with
// ciparity.RequiredComposedGates; an unmapped gate is left absent, which
// MissingComposedGates counts as not-green (fail-closed).
var composedGateTargets = map[string]string{
	"compile":  "build",
	"test":     "test",
	"acs":      "test-acs-durable",
	"apicover": "apicover",
}

// runComposedGates re-runs the full native gate set against the composed
// (rebased) worktree and records "pass"/"fail" per gate. Gates follow the
// TREE: even when the audit verdict follows the change across a clean rebase,
// every required gate must be green on the composed tree before the verdict
// carries forward. Any non-zero exit → "fail" → MissingComposedGates trips →
// full re-audit.
func runComposedGates(ctx context.Context, worktree string) map[string]string {
	results := make(map[string]string, len(ciparity.RequiredComposedGates))
	for _, gate := range ciparity.RequiredComposedGates {
		target, ok := composedGateTargets[gate]
		if !ok {
			continue // unmapped ⇒ absent ⇒ fail-closed via MissingComposedGates
		}
		cmd := exec.CommandContext(ctx, "make", "-C", "go", target)
		cmd.Dir = worktree
		if err := cmd.Run(); err != nil {
			results[gate] = "fail"
			continue
		}
		results[gate] = "pass"
	}
	return results
}

// auditLedgerEntry is the composition-verdict-scoped subset of an auditor
// ledger line the snapshot needs. Ship's audit reader (findLatestAudit) is
// package-private; a 4-field scan avoids exporting ship internals for this
// single call site.
// TODO(merge-concurrency-2026): fold into a shared ledger read-side helper if
// a third consumer appears.
type auditLedgerEntry struct {
	Role           string `json:"role"`
	Kind           string `json:"kind"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	GitHEAD        string `json:"git_head"`
}

// readCompositionSnapshot captures what the bound audit reviewed BEFORE a
// peer moved main: the auditor entry's artifact SHA (LaneAuditRef), the git
// HEAD it bound (AuditedBase), and the audited change as a diff + its
// patch-id. For a clean rebase, git patch-id is offset-insensitive, so this
// pre-rebase diff recomputes to the same patch-id as the post-rebase composed
// diff — the RUNG 0 identity check compositionCarryForward enforces.
func readCompositionSnapshot(ctx context.Context, worktree string) (core.CompositionAuditSnapshot, error) {
	ledgerPath := filepath.Join(worktree, ".evolve", "ledger.jsonl")
	entry, err := latestAuditEntry(ledgerPath)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	// Three-dot: the lane's change against the merge-base with main — i.e.
	// exactly what the audit reviewed at the base it bound.
	diff, err := gitDiffCapture(ctx, worktree, "main..."+entry.GitHEAD)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	patchID, err := ledger.PatchID(diff)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	return core.CompositionAuditSnapshot{
		LaneAuditRef: entry.ArtifactSHA256,
		AuditedBase:  entry.GitHEAD,
		Diff:         diff,
		PatchID:      patchID,
	}, nil
}

// latestAuditEntry walks ledger.jsonl backwards for the most recent bound
// auditor entry. Mirrors ship.findLatestAudit's tolerance: alien/unparseable
// lines are skipped; an absent or auditor-less ledger is an error the
// snapshot surfaces so compositionCarryForward fails closed to full re-audit.
func latestAuditEntry(ledgerPath string) (auditLedgerEntry, error) {
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return auditLedgerEntry{}, fmt.Errorf("composition snapshot: read ledger %s: %w", ledgerPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var e auditLedgerEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Kind == "agent_subprocess" && e.Role == "auditor" && e.GitHEAD != "" {
			return e, nil
		}
	}
	return auditLedgerEntry{}, fmt.Errorf("composition snapshot: no bound auditor entry in %s", ledgerPath)
}

// gitDiffCapture runs `git diff <spec>` in worktree and returns its stdout.
func gitDiffCapture(ctx context.Context, worktree, spec string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", spec)
	cmd.Dir = worktree
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("composition snapshot: git diff %s: %w", spec, err)
	}
	return out.Bytes(), nil
}
