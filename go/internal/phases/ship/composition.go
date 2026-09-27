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

// Review verdicts follow the patch-id (the change); native gates must still
// follow the tree. Every rejected condition falls back to a full re-audit,
// so this path can only narrow what ships, never widen it.
// See ADR-0069.
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

// findCompositionVerdict mirrors findLatestAudit's tolerance: an unreadable
// ledger or an alien line simply yields no match.
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
		if ce.Kind != ledger.CompositionVerdictKind || ce.Method != ledger.TrivialRebaseMethod {
			continue
		}
		if ce.LaneAuditRef != auditRef || ce.GitHead != currentHEAD {
			continue
		}
		return &ce
	}
	return nil
}
