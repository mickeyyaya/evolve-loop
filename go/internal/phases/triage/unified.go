package triage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type unifiedProjection struct {
	Size        string `json:"size"`
	MemberCount int    `json:"member_count"`
}

// processUnifiedCommitment validates the optional LLM-authored claim and adds
// only its deterministic projection to the authoritative decision. Invalid
// claims remain visible for forensics but cannot influence routing.
func processUnifiedCommitment(req core.PhaseRequest) ([]core.Diagnostic, error) {
	decisionPath := filepath.Join(req.Workspace, "triage-decision.json")
	raw, err := os.ReadFile(decisionPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read triage decision: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, nil // existing decision digest reports malformed JSON loudly
	}
	claimRaw, declared := fields["unified_commitment"]
	if !declared || bytes.Equal(bytes.TrimSpace(claimRaw), []byte("null")) {
		if _, spoofed := fields["unified_projection"]; spoofed {
			return rejectUnified(fields, decisionPath, "projection declared without unified_commitment")
		}
		return nil, nil
	}

	var claim inboxbatch.UnifiedCommitment
	if err := json.Unmarshal(claimRaw, &claim); err != nil {
		return rejectUnified(fields, decisionPath, "invalid JSON: "+err.Error())
	}
	items, _, err := inboxbatch.LoadDir(filepath.Join(req.ProjectRoot, ".evolve", "inbox"))
	if err != nil {
		return nil, fmt.Errorf("load inbox for unified_commitment: %w", err)
	}
	claimed, _, err := inboxbatch.LoadDir(filepath.Join(req.ProjectRoot, ".evolve", "inbox", "processing", fmt.Sprintf("cycle-%d", req.Cycle)))
	if err != nil {
		return nil, fmt.Errorf("load claimed inbox for unified_commitment: %w", err)
	}
	items = append(items, claimed...)
	if err := claim.Validate(items); err != nil {
		return rejectUnified(fields, decisionPath, err.Error())
	}

	var decision struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil {
		return rejectUnified(fields, decisionPath, "top_n is unreadable: "+err.Error())
	}
	committed := make(map[string]bool, len(decision.TopN))
	for _, item := range decision.TopN {
		committed[strings.TrimSpace(item.ID)] = true
	}
	for _, member := range claim.Members {
		if !committed[member.ID] {
			return rejectUnified(fields, decisionPath, fmt.Sprintf("member %q is outside top_n", member.ID))
		}
	}

	if claim.Size() == "large" {
		plan, err := campaign.PlanFromUnifiedCommitment(claim, items)
		if err != nil {
			return rejectUnified(fields, decisionPath, "campaign projection is invalid: "+err.Error())
		}
		if err := atomicwrite.JSON(filepath.Join(req.Workspace, "campaign-plan.json"), plan); err != nil {
			return nil, fmt.Errorf("write unified campaign plan: %w", err)
		}
	} else if err := removeIfPresent(filepath.Join(req.Workspace, "campaign-plan.json")); err != nil {
		return nil, err
	}
	projection, _ := json.Marshal(unifiedProjection{Size: claim.Size(), MemberCount: len(claim.Members)})
	fields["unified_projection"] = projection
	if err := atomicwrite.JSON(decisionPath, fields); err != nil {
		return nil, fmt.Errorf("write validated unified projection: %w", err)
	}
	return nil, nil
}

func rejectUnified(fields map[string]json.RawMessage, decisionPath, reason string) ([]core.Diagnostic, error) {
	if _, present := fields["unified_projection"]; present {
		delete(fields, "unified_projection")
		if err := atomicwrite.JSON(decisionPath, fields); err != nil {
			return nil, fmt.Errorf("clear invalid unified projection: %w", err)
		}
	}
	if err := removeIfPresent(filepath.Join(filepath.Dir(decisionPath), "campaign-plan.json")); err != nil {
		return nil, err
	}
	return []core.Diagnostic{{
		Severity: "warning",
		Message:  "unified_commitment rejected; preserving independent top_n: " + reason,
	}}, nil
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale unified campaign plan: %w", err)
	}
	return nil
}
