package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func cloneRouting(block policy.CLIRouting) policy.CLIRouting {
	block.CLIs = slices.Clone(block.CLIs)
	block.Default = slices.Clone(block.Default)
	block.Work = maps.Clone(block.Work)
	block.Tiers = maps.Clone(block.Tiers)
	block.Agents = maps.Clone(block.Agents)
	return block
}

type routingSet struct {
	key    string
	values []string
	model  string
}

func setRoutingKey(block policy.CLIRouting, set routingSet) (policy.CLIRouting, error) {
	key, values, model := set.key, set.values, set.model
	group, name, scoped := strings.Cut(key, ".")
	switch {
	case len(values) == 0:
		return block, fmt.Errorf("%s needs a value", key)
	case model != "" && group != "agents":
		return block, fmt.Errorf("--model applies only to agents.<agent>, not %s", key)
	case key == "after_chain" && len(values) > 1:
		return block, fmt.Errorf("after_chain takes one value: other_clis or stop")
	}
	switch {
	case key == "clis":
		block.CLIs = values
	case key == "default":
		block.Default = values
	case key == "after_chain":
		block.AfterChain = values[0]
	case scoped && group == "tiers":
		block.Tiers = withEntry(block.Tiers, name, values)
	case scoped && group == "work":
		block.Work = withEntry(block.Work, name, values)
	case scoped && group == "agents":
		block.Agents = withEntry(block.Agents, name, policy.AgentRule{CLI: values, Model: model})
	default:
		return block, fmt.Errorf("unknown key %q (clis, default, after_chain, tiers.<tier>, work.<role>, agents.<agent>)", key)
	}
	return block, nil
}

func unsetRoutingKey(block policy.CLIRouting, key string) (policy.CLIRouting, error) {
	group, name, scoped := strings.Cut(key, ".")
	switch {
	case key == "clis":
		block.CLIs = nil
	case key == "default":
		block.Default = nil
	case key == "after_chain":
		block.AfterChain = ""
	case scoped && group == "tiers":
		delete(block.Tiers, name)
	case scoped && group == "work":
		delete(block.Work, name)
	case scoped && group == "agents":
		delete(block.Agents, name)
	default:
		return block, fmt.Errorf("unknown key %q", key)
	}
	return block, nil
}

func withEntry[V any](m map[string]V, key string, value V) map[string]V {
	if m == nil {
		m = map[string]V{}
	}
	m[key] = value
	return m
}

type migration struct {
	existing []byte
	pol      policy.Policy
	block    policy.CLIRouting
	patch    map[string]any
}

func migrateLegacyKeys(existing []byte, pol policy.Policy, block policy.CLIRouting) (map[string]any, error) {
	m := &migration{existing: existing, pol: pol, block: cloneRouting(block), patch: map[string]any{}}
	for _, step := range []func() error{m.movePins, m.moveTail, m.moveRouterKeys} {
		if err := step(); err != nil {
			return nil, err
		}
	}
	m.patch["cli_routing"] = m.block
	return m.patch, nil
}

func (m *migration) movePins() error {
	pins := m.pol.Pins
	if len(pins) == 0 {
		return nil
	}
	for _, phase := range slices.Sorted(maps.Keys(pins)) {
		pin := pins[phase]
		if pin.CLI == "" {
			return fmt.Errorf("pins.%s sets only a model; a table rule needs a CLI: set agents.%s yourself", phase, phase)
		}
		if _, taken := m.block.Agents[phase]; taken {
			return fmt.Errorf("pins.%s and agents.%s both route %s: keep one", phase, phase, phase)
		}
		m.block.Agents = withEntry(m.block.Agents, phase, policy.AgentRule{CLI: []string{pin.CLI}, Model: pin.Model})
	}
	m.patch["pins"] = nil
	return nil
}

func (m *migration) moveTail() error {
	wf := m.pol.Workflow
	if wf == nil || (wf.UniversalFallback == nil && wf.UniversalFallbackExclude == nil) {
		return nil
	}
	if len(wf.UniversalFallbackExclude) > 0 {
		return fmt.Errorf("workflow.universal_fallback_exclude %v has no table form: drop those families from clis, then remove the key", wf.UniversalFallbackExclude)
	}
	if wf.UniversalFallback != nil && !*wf.UniversalFallback {
		m.block.AfterChain = "stop"
	}
	rest, err := blockWithout(m.existing, "workflow", "universal_fallback", "universal_fallback_exclude")
	m.patch["workflow"] = rest
	return err
}

func (m *migration) moveRouterKeys() error {
	rt := m.pol.Router
	if rt == nil || (rt.CLI == "" && rt.Model == "" && rt.PlanModel == "" && rt.ProposeModel == "") {
		return nil
	}
	tier, err := oneRouterTier(rt)
	if err != nil {
		return err
	}
	if rt.CLI == "" {
		return fmt.Errorf("router.model sets a tier without router.cli: set agents.router yourself")
	}
	if _, taken := m.block.Agents["router"]; taken {
		return fmt.Errorf("router.cli and agents.router both route the router: keep one")
	}
	m.block.Agents = withEntry(m.block.Agents, "router", policy.AgentRule{CLI: []string{rt.CLI}, Model: tier})
	rest, err := blockWithout(m.existing, "router", "cli", "model", "plan_model", "propose_model")
	m.patch["router"] = rest
	return err
}

func oneRouterTier(rt *policy.RouterPolicy) (string, error) {
	planTier := cmp.Or(rt.PlanModel, rt.Model)
	proposeTier := cmp.Or(rt.ProposeModel, rt.Model)
	if planTier != proposeTier {
		return "", fmt.Errorf("the router's plan and re-plan decisions run at %q but its propose and judge decisions at %q (router.model, router.plan_model, router.propose_model name different tiers): the table holds one tier per agent, so choose one and set agents.router", planTier, proposeTier)
	}
	return planTier, nil
}

func blockWithout(existing []byte, name string, keys ...string) (any, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(existing, &top); err != nil {
		return nil, err
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(top[name], &inner); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for _, k := range keys {
		delete(inner, k)
	}
	if len(inner) == 0 {
		return nil, nil
	}
	return inner, nil
}
