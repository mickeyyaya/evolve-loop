package policy

import (
	"cmp"
	"fmt"
)

const (
	RetryLadderShadow              = "shadow"
	RetryLadderEnforce             = "enforce"
	DefaultRetryLadderDigestTokens = 2000
	MinRetryLadderDigestTokens     = 64
	maxRetryLadderDigestTokens     = 16000
)

type RetryLadderPolicy struct {
	Stage        string          `json:"stage,omitempty"`
	DigestTokens int             `json:"digest_tokens,omitempty"`
	AuditFix     *AuditFixPolicy `json:"audit_fix,omitempty"`
}

type RetryLadderConfig struct {
	Stage        string
	DigestTokens int
	AuditFix     AuditFixConfig
}

func (c RetryLadderConfig) Enforced() bool { return c.Stage == RetryLadderEnforce }

func (p Policy) retryLadderConfig() RetryLadderConfig {
	c := RetryLadderConfig{Stage: RetryLadderShadow, DigestTokens: DefaultRetryLadderDigestTokens, AuditFix: defaultAuditFix}
	if p.Workflow == nil || p.Workflow.RetryLadder == nil {
		return c
	}
	block := p.Workflow.RetryLadder
	return RetryLadderConfig{
		Stage: cmp.Or(block.Stage, c.Stage), DigestTokens: cmp.Or(block.DigestTokens, c.DigestTokens), AuditFix: auditFixConfigOf(block.AuditFix),
	}
}

func (w *WorkflowPolicy) validateRetryLadder() error {
	if w == nil || w.RetryLadder == nil {
		return nil
	}
	switch block := w.RetryLadder; {
	case block.Stage != "" && block.Stage != RetryLadderShadow && block.Stage != RetryLadderEnforce:
		return fmt.Errorf("workflow.retry_ladder.stage must be %s or %s, got %q", RetryLadderShadow, RetryLadderEnforce, block.Stage)
	case block.DigestTokens != 0 && (block.DigestTokens < MinRetryLadderDigestTokens || block.DigestTokens > maxRetryLadderDigestTokens):
		return fmt.Errorf("workflow.retry_ladder.digest_tokens %d is outside %d to %d (0 means the default)", block.DigestTokens, MinRetryLadderDigestTokens, maxRetryLadderDigestTokens)
	}
	return w.RetryLadder.AuditFix.validate()
}
