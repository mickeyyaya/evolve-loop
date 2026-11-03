package policy

import (
	"errors"
	"fmt"
	"path/filepath"
)

const (
	credentialSourceStageOff    = "off"
	CredentialSourceStageShadow = "shadow"
)

type CredentialSourcePolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (c *CredentialSourcePolicy) validate() error {
	switch {
	case c == nil, c.Stage == "", c.Stage == credentialSourceStageOff, c.Stage == CredentialSourceStageShadow:
		return nil
	case c.Stage == "enforce":
		return errors.New(`stage "enforce" lands after the operator answers D1-OP1 (plan AD1-7); use "shadow"`)
	}
	return fmt.Errorf("stage %q: the stages are off and shadow", c.Stage)
}

func (p Policy) IsolationCredentialSourceStage() string {
	if p.Isolation == nil || p.Isolation.CredentialSource == nil || p.Isolation.CredentialSource.Stage == "" {
		return credentialSourceStageOff
	}
	return p.Isolation.CredentialSource.Stage
}

func CredentialSourceStageFor(projectRoot string) (string, error) {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return credentialSourceStageOff, err
	}
	return pol.IsolationCredentialSourceStage(), nil
}
