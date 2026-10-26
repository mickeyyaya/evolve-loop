package policy

import (
	"cmp"
	"fmt"
)

type AuditFixPolicy struct {
	MaxFiles             int   `json:"max_files,omitempty"`
	MaxLines             int   `json:"max_lines,omitempty"`
	MaxCodeLines         int   `json:"max_code_lines,omitempty"`
	MaxTokens            int   `json:"max_tokens,omitempty"`
	NoNewProductionLogic *bool `json:"no_new_production_logic,omitempty"`
}

type AuditFixConfig struct {
	MaxFiles             int
	MaxLines             int
	MaxCodeLines         int
	MaxTokens            int
	NoNewProductionLogic bool
}

var defaultAuditFix = AuditFixConfig{MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, MaxTokens: 6000, NoNewProductionLogic: true}

var auditFixCaps = AuditFixConfig{MaxFiles: 50, MaxLines: 2000, MaxCodeLines: 2000, MaxTokens: 64000}

func auditFixConfigOf(block *AuditFixPolicy) AuditFixConfig {
	if block == nil {
		return defaultAuditFix
	}
	c := AuditFixConfig{
		MaxFiles:             cmp.Or(block.MaxFiles, defaultAuditFix.MaxFiles),
		MaxLines:             cmp.Or(block.MaxLines, defaultAuditFix.MaxLines),
		MaxCodeLines:         cmp.Or(block.MaxCodeLines, defaultAuditFix.MaxCodeLines),
		MaxTokens:            cmp.Or(block.MaxTokens, defaultAuditFix.MaxTokens),
		NoNewProductionLogic: defaultAuditFix.NoNewProductionLogic,
	}
	if block.NoNewProductionLogic != nil {
		c.NoNewProductionLogic = *block.NoNewProductionLogic
	}
	return c
}

func (b *AuditFixPolicy) validate() error {
	if b == nil {
		return nil
	}
	for _, f := range []struct {
		key        string
		value, cap int
	}{
		{"max_files", b.MaxFiles, auditFixCaps.MaxFiles},
		{"max_lines", b.MaxLines, auditFixCaps.MaxLines},
		{"max_code_lines", b.MaxCodeLines, auditFixCaps.MaxCodeLines},
		{"max_tokens", b.MaxTokens, auditFixCaps.MaxTokens},
	} {
		if f.value < 0 || f.value > f.cap {
			return fmt.Errorf("workflow.retry_ladder.audit_fix.%s %d is outside 0 to %d", f.key, f.value, f.cap)
		}
	}
	return nil
}
