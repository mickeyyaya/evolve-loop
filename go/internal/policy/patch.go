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
	if err := p.validate(); err != nil {
		return Policy{}, err
	}
	return p, nil
}

func (p Policy) validate() error {
	if err := p.Fleet.validateWindowStart(); err != nil {
		return fmt.Errorf("fleet.orchestrator: %w", err)
	}
	if err := p.Workflow.validate(); err != nil {
		return err
	}
	if err := p.Isolation.validate(); err != nil {
		return err
	}
	if err := p.Boundary.validate(); err != nil {
		return err
	}
	if err := p.Catalog.validate(); err != nil {
		return err
	}
	if err := p.Overlays.validatePaths(); err != nil {
		return err
	}
	if err := p.Healing.validate(); err != nil {
		return fmt.Errorf("healing.%w", err)
	}
	if err := p.ExpectedSignal.validate(); err != nil {
		return fmt.Errorf("expected_signal: %w", err)
	}
	if err := p.Preflight.validate(); err != nil {
		return fmt.Errorf("preflight.%w", err)
	}
	if err := p.validateDiskFloor(); err != nil {
		return err
	}
	return p.Telemetry.validate()
}

func (i *IsolationPolicy) validate() error {
	if i == nil {
		return nil
	}
	if err := i.Context.validate(); err != nil {
		return fmt.Errorf("isolation.context: %w", err)
	}
	if err := i.DispatchRequests.validate(); err != nil {
		return fmt.Errorf("isolation.dispatch_requests: %w", err)
	}
	if err := i.InboxRequests.validate(); err != nil {
		return fmt.Errorf("isolation.inbox_requests: %w", err)
	}
	return nil
}

func (w *WorkflowPolicy) validate() error {
	for _, check := range []func() error{w.validateRetryLadder, w.validateRepair, w.validateCycleSpend, w.validateBlockerAttribution, w.validateBuilderSelfCheck, w.validateSupersede} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
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
