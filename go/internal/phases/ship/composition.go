// composition.go — trivial-rebase audit carry-forward (merge ladder RUNG 0,
// cycle-786; knowledge-base/research/merge-concurrency-2026).
//
// Review verdicts follow the CHANGE (git patch-id), gates follow the TREE.
// When git HEAD moved after the audit only because the lane was rebased
// conflict-free onto a moved main (patch-id unchanged) and the full native
// gate set re-ran green on the composed tree, the audit verdict carries
// forward via a composition-verdict ledger entry instead of hard-failing
// CodeAuditBindingHeadMoved into a full re-audit — the Gerrit
// `copyCondition: TRIVIAL_REBASE` precedent. Every rejected condition falls
// back to the pre-existing full re-audit path; the fast path can only
// narrow, never widen, what ships.
package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

// compositionEntry is the subset of a composition-verdict ledger line the
// fast path reads.
type compositionEntry struct {
	Kind         string            `json:"kind"`
	Method       string            `json:"method"`
	LaneAuditRef string            `json:"lane_audit_ref"`
	PatchID      string            `json:"patch_id"`
	AuditedBase  string            `json:"audited_base"`
	GitHead      string            `json:"git_head"`
	TreeStateSHA string            `json:"tree_state_sha"`
	GateResults  map[string]string `json:"gate_results"`
}

// tryTrivialRebaseCarryForward reports whether a valid composition-verdict
// entry lets the bound audit carry forward to currentHEAD:
//
//  1. entry chains the bound auditor entry (lane_audit_ref) to currentHEAD
//     and records the audited base the audit actually bound;
//  2. the full native gate set re-ran green on the composed tree
//     (ciparity.RequiredComposedGates — gates follow the tree, ADR-0069);
//  3. the composed tree ship sees NOW is the one the entry's gates ran on;
//  4. live kernel recompute: `git diff HEAD | git patch-id --stable` still
//     equals the audited patch_id — semantic drift, however textually
//     clean, falls back to full re-audit.
//
// false = fall back to the pre-existing CodeAuditBindingHeadMoved error.
// A non-nil error is reserved for I/O failures while recomputing the
// composed tree, which must surface as themselves rather than as HeadMoved.
func tryTrivialRebaseCarryForward(ctx context.Context, opts *Options, res *RunResult, ledgerPath string, audit *auditEntry, currentHEAD string) (bool, error) {
	ce := findCompositionVerdict(ledgerPath, audit.ArtifactSHA256, currentHEAD)
	if ce == nil {
		return false, nil
	}
	if ce.AuditedBase != audit.GitHEAD {
		return false, nil
	}
	if missing := ciparity.MissingComposedGates(ce.GateResults); missing != nil {
		res.Logs = append(res.Logs,
			"[ship] trivial-rebase carry-forward rejected: composed-tree gates not green: "+strings.Join(missing, ","))
		return false, nil
	}
	currentTree, err := computeTreeStateSHA(ctx, opts)
	if err != nil {
		return false, err
	}
	if currentTree != ce.TreeStateSHA {
		return false, nil
	}
	diff, err := captureGitOutput(ctx, opts, "diff", "HEAD")
	if err != nil {
		return false, err
	}
	got, err := ledger.PatchID([]byte(diff))
	if err != nil || got != ce.PatchID {
		res.Logs = append(res.Logs,
			"[ship] trivial-rebase carry-forward rejected: composed tree's patch-id does not recompute to the audited patch_id — falling back to full re-audit")
		return false, nil
	}
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] OK: trivial-rebase carry-forward — audit verdict follows patch-id %s (base %.12s → %.12s), composed-tree gates green",
		ce.PatchID, ce.AuditedBase, ce.GitHead))
	return true, nil
}

// findCompositionVerdict walks ledger.jsonl backwards for the most recent
// trivial-rebase composition-verdict entry chaining auditRef (the bound
// auditor entry's artifact_sha256) to currentHEAD. Mirrors findLatestAudit's
// tolerance: unreadable ledger or alien lines simply yield no match — the
// caller then takes the pre-existing HeadMoved path.
func findCompositionVerdict(ledgerPath, auditRef, currentHEAD string) *compositionEntry {
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var ce compositionEntry
		if err := json.Unmarshal([]byte(line), &ce); err != nil {
			continue
		}
		if ce.Kind != ledger.CompositionVerdictKind || ce.Method != "trivial-rebase" {
			continue
		}
		if ce.LaneAuditRef != auditRef || ce.GitHead != currentHEAD {
			continue
		}
		return &ce
	}
	return nil
}
