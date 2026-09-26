package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type disposition struct {
	Cycle       int    `json:"cycle"`
	Fingerprint string `json:"fingerprint"`
	Recurrence  int    `json:"recurrence"`
	Legitimacy  string `json:"legitimacy"`
	RootCause   struct {
		Layer   string `json:"layer"`
		Summary string `json:"summary"`
	} `json:"root_cause"`
	Salvage struct {
		WorktreeHasValue bool   `json:"worktree_has_value"`
		Pointer          string `json:"pointer"`
	} `json:"salvage"`
	Urgency       string `json:"urgency"`
	Justification string `json:"justification"`
	Routing       string `json:"routing"`
	ProposedItem  string `json:"proposed_item"`
}

func readDispositionLegitimacy(workspace string, cycle int) string {
	b, err := os.ReadFile(filepath.Join(workspace, "disposition.json"))
	if err != nil {
		return ""
	}
	var d disposition
	if err := json.Unmarshal(b, &d); err != nil {
		return ""
	}
	if err := crossCheckAgainstDigest(workspace, d); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN disposition identity unverified (%v) — legitimacy not trusted for repair\n", err)
		return ""
	}
	if cycle <= 0 || d.Cycle != cycle {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN disposition names cycle %d, not %d — legitimacy not trusted for repair\n", d.Cycle, cycle)
		return ""
	}
	return d.Legitimacy
}

// Disposition enum vocabularies. Out-of-vocabulary values are rejected with the
// offending field named, so a JSON-parses-cleanly document still fails the gate.
var (
	validLegitimacy = map[string]bool{"legit-rejection": true, "false-rejection": true, "infra-failure": true, "indeterminate": true}
	validLayer      = map[string]bool{"task-code": true, "pipeline-code": true, "harness": true, "infra": true, "eval-contract": true}
	validUrgency    = map[string]bool{"P0": true, "P1": true, "P2": true, "P3": true}
	// "console" (not "escalate") is the operator-owned routing term — one
	// vocabulary with ADR-0074's route field and the plan-time gate.
	validRouting = map[string]bool{"inbox": true, "carryover": true, "console": true, "drop": true}
)

const dispositionSchemaExample = `{
  "cycle": 1398,
  "fingerprint": "ship|gate-block|cd49274beab2",
  "recurrence": 2,
  "legitimacy": "false-rejection",
  "root_cause": {"layer": "pipeline-code", "summary": "ship repo-contract gate bound an untracked runtime-minted profile stub, redding every lane"},
  "salvage": {"worktree_has_value": true, "pointer": ".evolve/worktrees/cycle-42824668-1403 (snapshot e0638346)"},
  "urgency": "P0",
  "justification": "audit-report.md PASS and acs-verdict.json green while ship-error.json records REPO_CONTRACT_GATE; the scanner output names TestRepoPersonaProfilePairing",
  "routing": "console",
  "proposed_item": ""
}`

// VerifyDisposition enforces the disposition contract for a retro workspace.
// nil means the disposition is valid and its failure identity agrees with the
// assembler's digest.
func VerifyDisposition(workspace string) error {
	raw, err := os.ReadFile(filepath.Join(workspace, "disposition.json"))
	if err != nil {
		return fmt.Errorf("disposition.json is a required retro deliverable but is absent: %w", err)
	}
	var d disposition
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("disposition.json is malformed: %w", err)
	}

	if !validLegitimacy[d.Legitimacy] {
		return fmt.Errorf("disposition legitimacy %q is out of vocabulary", d.Legitimacy)
	}
	if !validLayer[d.RootCause.Layer] {
		return fmt.Errorf("disposition root_cause.layer %q is out of vocabulary", d.RootCause.Layer)
	}
	if !validUrgency[d.Urgency] {
		return fmt.Errorf("disposition urgency %q is out of vocabulary", d.Urgency)
	}
	if !validRouting[d.Routing] {
		return fmt.Errorf("disposition routing %q is out of vocabulary", d.Routing)
	}

	if err := crossCheckAgainstDigest(workspace, d); err != nil {
		return err
	}

	if d.Salvage.WorktreeHasValue && d.Salvage.Pointer == "" {
		return fmt.Errorf("salvage floor: worktree_has_value=true requires a non-empty pointer")
	}
	return nil
}

func crossCheckAgainstDigest(workspace string, d disposition) error {
	raw, err := os.ReadFile(filepath.Join(workspace, "failure-digest.json"))
	if err != nil {
		return fmt.Errorf("cannot cross-check disposition: failure-digest.json unreadable: %w", err)
	}
	var dg FailureDigest
	if err := json.Unmarshal(raw, &dg); err != nil {
		return fmt.Errorf("cannot cross-check disposition: failure-digest.json malformed: %w", err)
	}
	if d.Fingerprint != dg.Fingerprint {
		return fmt.Errorf("disposition fingerprint %q disagrees with the digest %q (invented identity)", d.Fingerprint, dg.Fingerprint)
	}
	if d.Recurrence != dg.Recurrence {
		return fmt.Errorf("disposition recurrence %d disagrees with the digest's ledger-derived %d", d.Recurrence, dg.Recurrence)
	}
	return nil
}

func (o *Orchestrator) finalizeRetroCompletion(workspace string) error {
	if err := VerifyDisposition(workspace); err != nil {
		return fmt.Errorf("disposition-gate: %w", err)
	}
	return nil
}
