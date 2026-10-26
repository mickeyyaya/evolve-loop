package policy

import (
	"errors"
	"fmt"
	"regexp"
)

type IsolationPolicy struct {
	I2 IsolationInvariantPolicy `json:"i2,omitempty"`
	I3 IsolationInvariantPolicy `json:"i3,omitempty"`
	I5 IsolationInvariantPolicy `json:"i5,omitempty"`
	I7 IsolationInvariantPolicy `json:"i7,omitempty"`
	I8 IsolationInvariantPolicy `json:"i8,omitempty"`

	LineageMax int `json:"lineage_max,omitempty"`

	Context *IsolationContextPolicy `json:"context,omitempty"`
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
