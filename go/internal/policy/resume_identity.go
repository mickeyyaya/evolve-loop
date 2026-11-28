package policy

import "fmt"

type ResumeIdentityStage string

const (
	ResumeIdentityOff     ResumeIdentityStage = "off"
	ResumeIdentityShadow  ResumeIdentityStage = "shadow"
	ResumeIdentityEnforce ResumeIdentityStage = "enforce"
)

var resumeIdentityStages = []ResumeIdentityStage{ResumeIdentityOff, ResumeIdentityShadow, ResumeIdentityEnforce}

type ResumeIdentityPolicy struct {
	Stage ResumeIdentityStage `json:"stage,omitempty"`
}

func (r *ResumeIdentityPolicy) validate() error {
	if r == nil || r.Stage == "" {
		return nil
	}
	for _, stage := range resumeIdentityStages {
		if r.Stage == stage {
			return nil
		}
	}
	return fmt.Errorf("resume.identity.stage: %q is not one of %v", r.Stage, resumeIdentityStages)
}

func (p Policy) ResumeIdentityStage() ResumeIdentityStage {
	if p.Resume == nil || p.Resume.Identity == nil || p.Resume.Identity.Stage == "" {
		return ResumeIdentityShadow
	}
	return p.Resume.Identity.Stage
}
