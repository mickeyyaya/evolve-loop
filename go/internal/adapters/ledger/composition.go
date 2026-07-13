// composition.go — kernel verification of composition-verdict entries
// (merge ladder RUNG 0, cycle-786; knowledge-base/research/
// merge-concurrency-2026: review verdicts follow the CHANGE via git
// patch-id, gates follow the TREE).
//
// A composition-verdict entry records that an audit verdict carried
// forward across a conflict-free trivial rebase. It is deterministic and
// kernel-recomputable: anyone can re-derive the patch-id of its two
// persisted diff artifacts and compare against the recorded patch_id —
// zero LLM tokens. Verify/VerifyDeep do exactly that for every such
// entry; a mismatch (drifted composed diff, forged patch_id, missing
// artifact) is tampering and breaks the chain like any hash break.
package ledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// CompositionVerdictKind is the ledger entry kind recording a
// trivial-rebase audit carry-forward (ship's fast path reads it, ledger
// verify kernel-recomputes it).
const CompositionVerdictKind = "composition-verdict"

// TrivialRebaseMethod is the only composition method RUNG 0 defines; the
// ship-side reader filters on it, so writer and reader share one constant.
const TrivialRebaseMethod = "trivial-rebase"

// compositionFields is the kernel-recomputable subset of a
// composition-verdict line.
type compositionFields struct {
	PatchID          string `json:"patch_id"`
	AuditedDiffPath  string `json:"audited_diff_path"`
	ComposedDiffPath string `json:"composed_diff_path"`
}

// PatchID pipes a unified diff through `git patch-id --stable` and returns
// the patch-id — the offset-insensitive content identity of a change
// (bors-style patch identity; merge-concurrency-2026). patch-id is a pure
// stdin filter: no repo access, works in any directory.
func PatchID(diff []byte) (string, error) {
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Stdin = bytes.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git patch-id --stable: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("git patch-id --stable: empty output (empty diff?)")
	}
	return fields[0], nil
}

// CompositionVerdictInput carries everything a caller must supply to persist
// a trivial-rebase composition verdict. The writer trusts none of it: the
// claimed PatchID must recompute from BOTH diffs and GateResults must be
// green on the full ciparity.RequiredComposedGates set, or nothing is
// written — fail-closed at write time so no line can land that
// verifyCompositionLine would immediately flag as tampered.
type CompositionVerdictInput struct {
	Cycle        int
	LaneAuditRef string            // artifact_sha256 of the bound auditor entry
	PatchID      string            // caller-claimed patch-id of the change
	AuditedBase  string            // git HEAD the audit originally bound
	GitHead      string            // composed HEAD the verdict carries forward to
	TreeStateSHA string            // composed tree state the gates ran on
	GateResults  map[string]string // must record "pass" for every required composed gate
	AuditedDiff  []byte            // unified diff the audit reviewed
	ComposedDiff []byte            // unified diff of the composed (rebased) tree
	ArtifactDir  string            // directory the two diff artifacts persist under
}

// compositionRecord is the full on-disk composition-verdict line — the
// union of ship's compositionEntry (read side) and this package's
// compositionFields (kernel-verify side), plus the hash-chain fields
// appendChained fills in.
type compositionRecord struct {
	TS               string            `json:"ts"`
	Cycle            int               `json:"cycle"`
	Kind             string            `json:"kind"`
	Method           string            `json:"method"`
	LaneAuditRef     string            `json:"lane_audit_ref"`
	PatchID          string            `json:"patch_id"`
	AuditedBase      string            `json:"audited_base"`
	GitHead          string            `json:"git_head"`
	TreeStateSHA     string            `json:"tree_state_sha"`
	GateResults      map[string]string `json:"gate_results"`
	AuditedDiffPath  string            `json:"audited_diff_path"`
	ComposedDiffPath string            `json:"composed_diff_path"`
	EntrySeq         int               `json:"entry_seq"`
	PrevHash         string            `json:"prev_hash"`
}

// WriteCompositionVerdict validates in, persists both diff artifacts under
// in.ArtifactDir, and appends one hash-chained composition-verdict line to
// ledgerPath. Every validation failure returns before any byte is written
// to the ledger. This is the RUNG 0 producer for the fast path
// ship.tryTrivialRebaseCarryForward consumes (merge-concurrency-2026).
func WriteCompositionVerdict(ledgerPath string, in CompositionVerdictInput) error {
	if filepath.Base(ledgerPath) != "ledger.jsonl" {
		return fmt.Errorf("composition-verdict: ledger path must be a ledger.jsonl (chained append), got %q", ledgerPath)
	}
	diffs := []struct {
		label string
		diff  []byte
	}{
		{"audited diff", in.AuditedDiff},
		{"composed diff", in.ComposedDiff},
	}
	for _, d := range diffs {
		if len(bytes.TrimSpace(d.diff)) == 0 {
			return fmt.Errorf("composition-verdict: %s is empty or whitespace-only — nothing to attest", d.label)
		}
	}
	if missing := ciparity.MissingComposedGates(in.GateResults); missing != nil {
		return fmt.Errorf("composition-verdict: composed-tree gates not green: %s", strings.Join(missing, ","))
	}
	for _, d := range diffs {
		got, err := PatchID(d.diff)
		if err != nil {
			return fmt.Errorf("composition-verdict: %s: %w", d.label, err)
		}
		if got != in.PatchID {
			return fmt.Errorf("composition-verdict: %s recomputes patch-id %s but caller claims %s — refusing to write a line verify would flag as tampered", d.label, got, in.PatchID)
		}
	}

	if err := os.MkdirAll(in.ArtifactDir, 0o755); err != nil {
		return fmt.Errorf("composition-verdict: artifact dir: %w", err)
	}
	auditedPath := filepath.Join(in.ArtifactDir, fmt.Sprintf("composition-%d-audited.diff", in.Cycle))
	composedPath := filepath.Join(in.ArtifactDir, fmt.Sprintf("composition-%d-composed.diff", in.Cycle))
	for _, a := range []struct {
		path string
		diff []byte
	}{{auditedPath, in.AuditedDiff}, {composedPath, in.ComposedDiff}} {
		if err := os.WriteFile(a.path, a.diff, 0o644); err != nil {
			return fmt.Errorf("composition-verdict: persist artifact %s: %w", a.path, err)
		}
	}

	rec := compositionRecord{
		TS:               time.Now().UTC().Format(time.RFC3339),
		Cycle:            in.Cycle,
		Kind:             CompositionVerdictKind,
		Method:           TrivialRebaseMethod,
		LaneAuditRef:     in.LaneAuditRef,
		PatchID:          in.PatchID,
		AuditedBase:      in.AuditedBase,
		GitHead:          in.GitHead,
		TreeStateSHA:     in.TreeStateSHA,
		GateResults:      in.GateResults,
		AuditedDiffPath:  auditedPath,
		ComposedDiffPath: composedPath,
	}
	return New(filepath.Dir(ledgerPath)).appendChained(func(seq int, prevHash string) any {
		rec.EntrySeq = seq
		rec.PrevHash = prevHash
		return rec
	})
}

// verifyCompositionLine kernel-recomputes one composition-verdict line:
// both persisted diff artifacts must recompute to the recorded patch_id.
// Any failure wraps core.ErrLedgerChainBroken so `evolve ledger verify`
// exits 2 exactly as it does for a hash-chain break.
func verifyCompositionLine(i int, line []byte) error {
	var f compositionFields
	if err := json.Unmarshal(line, &f); err != nil {
		return fmt.Errorf("%w: line %d composition-verdict unmarshal: %v", core.ErrLedgerChainBroken, i, err)
	}
	if f.PatchID == "" {
		return fmt.Errorf("%w: line %d composition-verdict has no patch_id — not kernel-recomputable", core.ErrLedgerChainBroken, i)
	}
	for _, p := range []struct{ label, path string }{
		{"audited_diff_path", f.AuditedDiffPath},
		{"composed_diff_path", f.ComposedDiffPath},
	} {
		diff, err := os.ReadFile(p.path)
		if err != nil {
			return fmt.Errorf("%w: line %d composition-verdict %s unreadable: %v", core.ErrLedgerChainBroken, i, p.label, err)
		}
		got, err := PatchID(diff)
		if err != nil {
			return fmt.Errorf("%w: line %d composition-verdict %s: %v", core.ErrLedgerChainBroken, i, p.label, err)
		}
		if got != f.PatchID {
			return fmt.Errorf("%w: line %d composition-verdict tampered: %s recomputes patch-id %s, entry records %s",
				core.ErrLedgerChainBroken, i, p.label, got, f.PatchID)
		}
	}
	return nil
}
