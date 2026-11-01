package policy

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

const (
	DispatchRequestStageOff     = "off"
	DispatchRequestStageShadow  = "shadow"
	DispatchRequestStageEnforce = "enforce"

	dispatchRequestsPerCycle = 3
	dispatchRequestsMaxDepth = 1
)

var (
	dispatchRequestStages = []string{DispatchRequestStageOff, DispatchRequestStageShadow, DispatchRequestStageEnforce}
	dispatchRequestRoles  = []string{"fault-localization", "premise-challenge", "dependency-map"}
	profileName           = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

type IsolationPolicy struct {
	I2  IsolationInvariantPolicy `json:"i2,omitempty"`
	I3  IsolationInvariantPolicy `json:"i3,omitempty"`
	I5  IsolationInvariantPolicy `json:"i5,omitempty"`
	I7  IsolationInvariantPolicy `json:"i7,omitempty"`
	I8  IsolationInvariantPolicy `json:"i8,omitempty"`
	I15 IsolationInvariantPolicy `json:"i15,omitempty"`

	LineageMax int `json:"lineage_max,omitempty"`

	Context *IsolationContextPolicy `json:"context,omitempty"`

	DispatchRequests *DispatchRequestsPolicy `json:"dispatch_requests,omitempty"`
}

type DispatchRequestsPolicy struct {
	Stage    string   `json:"stage,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	PerCycle int      `json:"per_cycle,omitempty"`
	MaxDepth int      `json:"max_depth,omitempty"`
}

type IsolationContextPolicy struct {
	Stage    string                 `json:"stage,omitempty"`
	Budget   IsolationContextBudget `json:"budget,omitempty"`
	Required map[string][]string    `json:"required,omitempty"`
}

type IsolationContextBudget struct {
	L1Tokens int `json:"l1_tokens,omitempty"`
	L2Tokens int `json:"l2_tokens,omitempty"`
}

type IsolationInvariantPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) IsolationEnvStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I2.Stage
}

func (p Policy) IsolationExecStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I3.Stage
}

func (p Policy) IsolationFlowStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I8.Stage
}

func (p Policy) IsolationReadStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I5.Stage
}

func (p Policy) IsolationQueueStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I15.Stage
}

func (p Policy) IsolationLineageMax() int {
	if p.Isolation == nil {
		return 0
	}
	return p.Isolation.LineageMax
}

func (p Policy) IsolationGuaranteeStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I7.Stage
}

func (p Policy) IsolationContext() IsolationContextPolicy {
	if p.Isolation == nil || p.Isolation.Context == nil {
		return IsolationContextPolicy{}
	}
	return *p.Isolation.Context
}

var contextKind = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (c *IsolationContextPolicy) validate() error {
	switch {
	case c == nil:
		return nil
	case c.Stage == "enforce":
		return errors.New(`stage "enforce" lands with the inbox item context-index-l0-prompt-enforce; use "shadow"`)
	case c.Stage != "" && c.Stage != "shadow":
		return fmt.Errorf(`stage %q: the only stage is "shadow"`, c.Stage)
	case c.Budget.L1Tokens < 0:
		return fmt.Errorf("budget.l1_tokens %d: a budget cannot be negative", c.Budget.L1Tokens)
	case c.Budget.L2Tokens < 0:
		return fmt.Errorf("budget.l2_tokens %d: a budget cannot be negative", c.Budget.L2Tokens)
	}
	for phase, kinds := range c.Required {
		for _, k := range kinds {
			if !contextKind.MatchString(k) {
				return fmt.Errorf("required.%s: kind %q is not an artifact kind such as scout-report", phase, k)
			}
		}
	}
	return nil
}

func (p Policy) IsolationDispatchRequests() DispatchRequestsPolicy {
	out := DispatchRequestsPolicy{}
	if p.Isolation != nil && p.Isolation.DispatchRequests != nil {
		out = *p.Isolation.DispatchRequests
	}
	if out.Stage == "" {
		out.Stage = DispatchRequestStageShadow
	}
	if len(out.Roles) == 0 {
		out.Roles = dispatchRequestRoles
	}
	if out.PerCycle == 0 {
		out.PerCycle = dispatchRequestsPerCycle
	}
	if out.MaxDepth == 0 {
		out.MaxDepth = dispatchRequestsMaxDepth
	}
	out.Roles = slices.Clone(out.Roles)
	return out
}

func (d *DispatchRequestsPolicy) validate() error {
	switch {
	case d == nil:
		return nil
	case d.Stage != "" && !slices.Contains(dispatchRequestStages, d.Stage):
		return fmt.Errorf("stage %q: the stages are %v", d.Stage, dispatchRequestStages)
	case d.PerCycle < 0:
		return fmt.Errorf("per_cycle %d: a cap cannot be negative", d.PerCycle)
	case d.MaxDepth < 0:
		return fmt.Errorf("max_depth %d: a cap cannot be negative", d.MaxDepth)
	}
	for _, role := range d.Roles {
		if !profileName.MatchString(role) {
			return fmt.Errorf("role %q is not a profile name such as fault-localization", role)
		}
	}
	return nil
}
