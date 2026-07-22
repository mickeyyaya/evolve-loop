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
