package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func Parse(raw []byte) (Policy, error) {
	if err := refuseNullCLIRouting(raw); err != nil {
		return Policy{}, err
	}
	if err := refuseUnknownKeys(raw); err != nil {
		return Policy{}, err
	}
	var p Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		return Policy{}, err
	}
	if err := p.Fleet.validateWindowStart(); err != nil {
		return Policy{}, fmt.Errorf("fleet.orchestrator: %w", err)
	}
	if err := p.Workflow.validateRetryLadder(); err != nil {
		return Policy{}, err
	}
	if err := p.Workflow.validateRepair(); err != nil {
		return Policy{}, err
	}
	if p.Isolation != nil {
		if err := p.Isolation.Context.validate(); err != nil {
			return Policy{}, fmt.Errorf("isolation.context: %w", err)
		}
	}
	return p, nil
}

var patchBlocksMarshalIndent = json.MarshalIndent

func PatchBlocks(existing []byte, patch map[string]any) ([]byte, error) {
	blocks := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &blocks); err != nil {
			return nil, fmt.Errorf("policy: the existing policy.json is malformed (%w); refusing to clobber it", err)
		}
	}
	for key, value := range patch {
		if value == nil {
			delete(blocks, key)
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("policy: encoding %s: %w", key, err)
		}
		blocks[key] = raw
	}
	out, err := patchBlocksMarshalIndent(blocks, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("policy: encoding policy.json: %w", err)
	}
	return append(out, '\n'), nil
}
