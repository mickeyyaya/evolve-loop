package policy

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditcascade"
)

const (
	AuditCascadeOff    = "off"
	AuditCascadeShadow = "shadow"
)

type AuditCascadePolicy struct {
	Stage            string   `json:"stage,omitempty"`
	RiskPaths        []string `json:"risk_paths,omitempty"`
	MaxChangedFiles  int      `json:"max_changed_files,omitempty"`
	EscalateVerdicts []string `json:"escalate_verdicts,omitempty"`
}

type AuditCascadeConfig struct {
	Stage string
	auditcascade.Rule
}

func (p Policy) AuditCascadeConfig() AuditCascadeConfig {
	c := p.AuditCascade
	if c == nil || c.Stage != AuditCascadeShadow {
		return AuditCascadeConfig{Stage: AuditCascadeOff}
	}
	return AuditCascadeConfig{Stage: AuditCascadeShadow, Rule: auditcascade.Rule{RiskPaths: c.RiskPaths, MaxChangedFiles: c.MaxChangedFiles, EscalateVerdicts: c.EscalateVerdicts}}
}

func AuditCascadeConfigFor(projectRoot string) AuditCascadeConfig {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return AuditCascadeConfig{Stage: AuditCascadeOff}
	}
	return pol.AuditCascadeConfig()
}
