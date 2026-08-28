package core

// disposition_gate.go — S2 disposition-contract-gate (cycle-1034, item
// failure-disposition-router). The retro phase gains a MANDATORY disposition.json
// deliverable. VerifyDisposition is the fail-HARD counterpart to
// readFailureDecision's fail-SOFT boundary: a required deliverable, so an
// absent/malformed/invalid disposition is a LOUD error (retro cannot complete),
// never a silent (nil,nil) fallback. It also cross-checks the disposition's
// fingerprint+recurrence against the S1 failure-digest.json so the agent cannot
// INVENT a failure identity in retro.
//
// disposition.json schema: {cycle, fingerprint, recurrence, legitimacy,
// root_cause:{layer,summary}, salvage:{worktree_has_value,pointer}, urgency,
// justification, routing, proposed_item}.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// disposition is the retro agent's authored classification of a failed cycle.
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

// readDispositionLegitimacy returns disposition.json's legitimacy field, or ""
// when the file is absent, unreadable, or malformed. It is the fail-SOFT
// counterpart to VerifyDisposition's fail-HARD gate, and exists for readers that
// treat the disposition as EVIDENCE rather than as a contract — the audit-repair
// rule, which must never grant a retry on the strength of a file it could not
// read. Returning "" there means "not eligible", so soft-failing is the safe
// direction; VerifyDisposition remains the only enforcement path.
func readDispositionLegitimacy(workspace string, cycle int) string {
	b, err := os.ReadFile(filepath.Join(workspace, "disposition.json"))
	if err != nil {
		return ""
	}
	var d disposition
	if err := json.Unmarshal(b, &d); err != nil {
		return ""
	}
	// IDENTITY FIRST (adversarial review, CRITICAL). A disposition is
	// agent-authored prose ABOUT a failure; the only part of it a machine
	// computed is the fingerprint/recurrence pair, and crossCheckAgainstDigest
	// exists precisely to stop an agent inventing a failure identity. Reading
	// legitimacy past that check let a fabricated, stale, or copied disposition
	// convert a genuine ADR-0072 system-failure HALT into a granted repair.
	//
	// Unverifiable is NOT verified-good: a missing or malformed digest returns
	// "" (no repair), because the safe direction here is to decline.
	if err := crossCheckAgainstDigest(workspace, d); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN disposition identity unverified (%v) — legitimacy not trusted for repair\n", err)
		return ""
	}
	// A disposition left in the workspace by a DIFFERENT cycle is stale evidence
	// about someone else's failure. cycle<=0 means the caller has no cycle to
	// check against, which is again not a licence to trust.
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

// dispositionSchemaExample is the single-sourced LEGAL example of the
// disposition contract. The retro persona (agents/evolve-retrospective.md,
// "Required deliverable: disposition.json") carries this exact document as its
// literal example; disposition_gate_singlesource_test.go parses both as JSON,
// asserts equality, and feeds this const through VerifyDisposition against a
// matching digest — so a drifted example fails CI instead of failing the
// agent (ADR-0084 invariant 2; the pre-2026-08-10 prose "example" was
// placeholder pseudo-JSON that would itself have failed this fail-HARD gate).
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

// VerifyDisposition enforces the disposition contract for a retro workspace. It is
// fail-HARD: absent/malformed disposition.json, out-of-vocabulary enums, a
// fingerprint/recurrence that disagrees with failure-digest.json, or a salvage
// floor violation all return a loud error. nil means the disposition is valid and
// its failure identity agrees with the assembler's digest.
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

	// Salvage floor: preserved worktree value must be pointed at, never silently
	// dropped (cycles 984/1000 salvage precedent).
	if d.Salvage.WorktreeHasValue && d.Salvage.Pointer == "" {
		return fmt.Errorf("salvage floor: worktree_has_value=true requires a non-empty pointer")
	}
	return nil
}

// crossCheckAgainstDigest rejects a disposition whose fingerprint or recurrence
// disagrees with the S1 failure-digest.json — this is what stops the agent from
// inventing a failure identity no assembler ever computed.
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

// finalizeRetroCompletion is the orchestrator seam that wires the disposition gate
// into the composed retro-completion path (unit-green != live-green). It wraps the
// gate error so the RetroDecision audit field surfaces a "disposition-gate" reason
// instead of silently recording a clean retro outcome.
func (o *Orchestrator) finalizeRetroCompletion(workspace string) error {
	if err := VerifyDisposition(workspace); err != nil {
		return fmt.Errorf("disposition-gate: %w", err)
	}
	return nil
}
