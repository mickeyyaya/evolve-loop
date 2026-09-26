package deliverable

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// verifySecondaries adds one violation per agent-owed file that is absent, blank or unparseable. A read fault
// other than absence is infra ambiguity and returns an error, so the caller fails open.
func verifySecondaries(res *Result, c phasecontract.Contract, roots phasecontract.Roots) error {
	for _, name := range c.AgentOwedFiles {
		// OwedPath is the one join the prompt tail also renders, and it keeps the read inside the workspace.
		path := phasecontract.OwedPath(roots.Workspace, name)
		base := filepath.Base(path)
		res.Owed = append(res.Owed, base)
		content, exists, err := readDeliverableWithGrace(path)
		if err != nil {
			return fmt.Errorf("deliverable: read %s: %w", path, err)
		}
		if !exists {
			res.add(CodeMissingSecondary, fmt.Sprintf("declared deliverable %s not found — write it to exactly: %s", base, path))
			continue
		}
		if strings.TrimSpace(content) == "" {
			res.add(CodeEmptySecondary, fmt.Sprintf("declared deliverable %s at %s is empty", base, path))
			continue
		}
		if msg := secondaryParseError(base, content); msg != "" {
			res.add(CodeMalformedSecondary, fmt.Sprintf("declared deliverable %s at %s is malformed: %s", base, path, msg))
		}
	}
	return nil
}

// secondaryParseError says why a .json or .ndjson secondary does not parse, or "" when it does or has no parse rule.
func secondaryParseError(base, content string) string {
	switch strings.ToLower(filepath.Ext(base)) {
	case ".json":
		if !json.Valid([]byte(content)) {
			return "not valid JSON"
		}
	case ".ndjson":
		for i, line := range strings.Split(content, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !json.Valid([]byte(line)) {
				return fmt.Sprintf("line %d is not valid JSON", i+1)
			}
		}
	}
	return ""
}

// VerifiesDeclaredDeliverables marks the gate for core's declared-deliverables wiring proof; true unless the gate is off.
func (r *Reviewer) VerifiesDeclaredDeliverables() bool { return r.stage != config.StageOff }
