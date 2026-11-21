package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const AuditRoundDispositionsFile = "audit-round-dispositions.json"

func readAuditScopeFile(path string) *AuditRoundScope {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var scope AuditRoundScope
	if json.Unmarshal(raw, &scope) != nil {
		return nil
	}
	return &scope
}

func roundDispositions(workspace string, round int, live bool, warn io.Writer) map[string]defectledger.Entry {
	name := AuditRoundDispositionsFile
	if !live {
		name = phasecontract.RoundArchiveFilename(name, round)
	}
	raw, err := os.ReadFile(filepath.Join(workspace, name))
	if err != nil {
		return nil
	}
	claims, err := defectledger.ParseClaims(raw)
	if err != nil {
		fmt.Fprintf(warn, "audit re-review: %s is unreadable, so every previous finding stays open: %v\n", name, err)
		return nil
	}
	return claims
}

var RoundEvidenceResolver defectledger.Resolver

func claimsFix(claim defectledger.Entry, worktree string) bool {
	if claim.Status != defectledger.StatusFixed || RoundEvidenceResolver == nil {
		return false
	}
	ok, _ := RoundEvidenceResolver(claim.Evidence, defectledger.Request{ProjectRoot: worktree, Worktree: worktree})
	return ok
}
