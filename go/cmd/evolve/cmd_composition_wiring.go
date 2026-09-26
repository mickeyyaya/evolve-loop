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
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

// compositionOptions binds the three composition closures.
// Orchestrator.CompositionFastPathWired ANDs them, so a partial binding stays dark.
func compositionOptions() []core.Option {
	return []core.Option{
		core.WithCompositionVerdictWriter(writeCompositionVerdict),
		core.WithCompositionGateRunner(runComposedGates),
		core.WithCompositionSnapshot(readCompositionSnapshot),
	}
}

// writeCompositionVerdict passes straight through to the ledger writer, the
// single home of the fail-closed patch-id and gate checks.
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

// composedGateTargets maps each required composed-tree gate to its Makefile
// target, so the fast path runs the commands CI runs. Keys track
// ciparity.RequiredComposedGates; an unmapped gate stays absent, so not green.
var composedGateTargets = map[string]string{
	"compile":  "build",
	"test":     "test",
	"acs":      "test-acs-durable",
	"apicover": "apicover-enforce",
}

// runComposedGates re-runs every required gate on the composed worktree: gates
// follow the tree even when the audit verdict follows the change.
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

// auditLedgerEntry is the subset of an auditor ledger line the snapshot needs;
// ship's reader is package-private.
// TODO(merge-concurrency-2026): fold into a shared ledger read helper if a third consumer appears.
type auditLedgerEntry struct {
	Role           string `json:"role"`
	Kind           string `json:"kind"`
	RunID          string `json:"run_id"`
	ArtifactPath   string `json:"artifact_path"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	GitHEAD        string `json:"git_head"`
}

// readCompositionSnapshot captures what the bound audit reviewed before a peer
// moved main. git patch-id is offset-insensitive, so after a clean rebase the
// composed diff recomputes the same patch-id.
func readCompositionSnapshot(ctx context.Context, worktree, runID string) (core.CompositionAuditSnapshot, error) {
	ledgerPath := filepath.Join(worktree, ".evolve", "ledger.jsonl")
	entry, err := latestAuditEntry(ledgerPath, runID)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	if err := requireReusableAudit(entry); err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	// Three-dot: the change against the merge-base, exactly what the audit reviewed.
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

// latestAuditEntry returns this run's newest bound auditor entry; runID "" keeps
// latest-any. The ledger is host-global and records FAIL audits too, so an
// unscoped "latest" can be a sibling lane's or a failed one.
func latestAuditEntry(ledgerPath, runID string) (auditLedgerEntry, error) {
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
		if e.Kind != "agent_subprocess" || e.Role != "auditor" || e.GitHEAD == "" {
			continue
		}
		if runID == "" || e.RunID == runID {
			return e, nil
		}
	}
	return auditLedgerEntry{}, fmt.Errorf("composition snapshot: no bound auditor entry for run %q in %s (foreign-run entries refused)", runID, ledgerPath)
}

// requireReusableAudit refuses a snapshot from an audit that did not pass. The
// verdict comes from the bound artifact, since the ledger's exit_code is 1 for
// WARN and FAIL alike, and there is no fallback to an older PASS.
func requireReusableAudit(entry auditLedgerEntry) error {
	if entry.ArtifactPath == "" {
		return fmt.Errorf("composition snapshot: bound auditor entry has no artifact_path — cannot confirm its verdict")
	}
	body, err := os.ReadFile(entry.ArtifactPath)
	if err != nil {
		return fmt.Errorf("composition snapshot: read audit artifact %s: %w", entry.ArtifactPath, err)
	}
	sentinel, ok := phasecontract.ParseVerdictSentinelFull(string(body))
	if !ok {
		return fmt.Errorf("composition snapshot: audit artifact %s declares no parseable verdict sentinel", entry.ArtifactPath)
	}
	// Only the audit phase's own sentinel counts: the parser is tail-anchored, so
	// a quoted foreign-phase sentinel could otherwise satisfy it.
	if sentinel.Phase != string(core.PhaseAudit) {
		return fmt.Errorf("composition snapshot: audit artifact %s carries a %q-phase verdict sentinel, not audit",
			entry.ArtifactPath, sentinel.Phase)
	}
	if !verdictcache.Reusable(sentinel.Verdict) {
		return fmt.Errorf("composition snapshot: refusing to carry forward a %s audit (%s) — only PASS/WARN are reusable",
			sentinel.Verdict, entry.ArtifactPath)
	}
	return nil
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
