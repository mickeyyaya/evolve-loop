package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type CLIRouting struct {
	CLIs       []string             `json:"clis,omitempty"`
	Default    []string             `json:"default,omitempty"`
	Work       map[string][]string  `json:"work,omitempty"`
	Agents     map[string]AgentRule `json:"agents,omitempty"`
	Tiers      map[string][]string  `json:"tiers,omitempty"`
	AfterChain string               `json:"after_chain,omitempty"`
}

type AgentRule struct {
	CLI   []string `json:"cli"`
	Model string   `json:"model,omitempty"`
}

func (c *CLIRouting) UnmarshalJSON(raw []byte) error {
	type block CLIRouting
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("cli_routing: %w", err)
	}
	*c = CLIRouting(decoded)
	return nil
}

func (r *AgentRule) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if isJSONNull(trimmed) {
		return agentRuleError(errNullRoutingValue)
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var chain []string
		if err := json.Unmarshal(trimmed, &chain); err != nil {
			return agentRuleError(err)
		}
		*r = AgentRule{CLI: chain}
		return nil
	}
	type rule AgentRule
	var decoded rule
	if err := decodeStrict(trimmed, &decoded); err != nil {
		return agentRuleError(err)
	}
	*r = AgentRule(decoded)
	return nil
}

func agentRuleError(err error) error {
	return fmt.Errorf(`agent rule: want an array of CLIs or {"cli": [...], "model": "<tier>"}: %w`, err)
}

func decodeStrict(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

var errNullRoutingValue = errors.New("null is not a routing value; remove the key instead")

func isJSONNull(raw []byte) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func refuseNullCLIRouting(raw []byte) error {
	var probe struct {
		Block json.RawMessage `json:"cli_routing"`
	}
	if json.Unmarshal(raw, &probe) != nil || !isJSONNull(probe.Block) {
		return nil
	}
	return fmt.Errorf("cli_routing: %w", errNullRoutingValue)
}
