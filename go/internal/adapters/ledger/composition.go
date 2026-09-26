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

// CompositionVerdictKind is the kind of an audit carry-forward entry that ledger verify kernel-recomputes.
const CompositionVerdictKind = "composition-verdict"

// TrivialRebaseMethod is the RUNG 0 composition method; the writer and the ship-side reader share it.
const TrivialRebaseMethod = "trivial-rebase"

// ScopedReviewMethod is the RUNG 2 method, recorded when a scoped merge review resolved an overlapping change.
const ScopedReviewMethod = "scoped-review"

// compositionFields is the kernel-recomputable subset of a composition-verdict line.
type compositionFields struct {
	PatchID          string `json:"patch_id"`
	AuditedDiffPath  string `json:"audited_diff_path"`
	ComposedDiffPath string `json:"composed_diff_path"`
}

// PatchID returns the `git patch-id --stable` content identity of diff; it needs no repository.
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

// CompositionVerdictInput is what a caller supplies to persist a composition verdict; the writer trusts none of it.
type CompositionVerdictInput struct {
	Cycle        int
	Method       string            // composition method; blank defaults to TrivialRebaseMethod (RUNG 0)
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

// compositionRecord is the on-disk line: the union of ship's compositionEntry and compositionFields,
// plus the chain fields appendChained fills in.
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

// WriteCompositionVerdict validates in, persists both diffs and appends one chained line; validation writes nothing.
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

	method := in.Method
	if method == "" {
		method = TrivialRebaseMethod
	}
	rec := compositionRecord{
		TS:               time.Now().UTC().Format(time.RFC3339),
		Cycle:            in.Cycle,
		Kind:             CompositionVerdictKind,
		Method:           method,
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

// verifyCompositionLine checks both persisted diffs recompute the recorded patch_id. Failures wrap
// core.ErrLedgerChainBroken so `evolve ledger verify` exits 2 as for a hash break.
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
