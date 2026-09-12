package deliverable

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// secondaries.go — ADR-0100: every AGENT-OWED declared output is verified,
// not only outputs.files[0].
//
// The registry has always declared a phase's full output list; FromSpec
// projected only the first entry into the contract, so handoff-build.json,
// handoff-scout.json, triage-decision.json and carryover-todos.json were
// waited for by the bridge, read by the router and by committedset, and
// judged by nobody. A phase could omit them and the cycle continued — the
// shape behind batch cycle 1623 (2026-09-11) and the empty triage decisions
// of 1630/1631 (2026-09-12).
//
// Deliberately narrow: existence, non-emptiness, and parseability for JSON /
// NDJSON. Markdown SHAPE stays the primary contract's business (Sections,
// Verdicts) — restating it here would be the duplicated belief the campaign
// forbids. Harness-produced secondaries (acs-verdict.json, written by
// acsrunner) are declared in the registry under harness_produced and never
// reach this function: re-dispatching an agent cannot make a harness write.

// verifySecondaries appends one violation per agent-owed secondary that is
// absent, blank, or unparseable. Each message names the file — that message
// is the correction directive the agent is re-dispatched with. A read fault
// that is not absence (EISDIR, permissions) is infra ambiguity and returns
// an error, exactly as the primary does, so the caller fails OPEN rather
// than re-dispatching an agent for a file nobody could read.
func verifySecondaries(res *Result, c phasecontract.Contract, roots phasecontract.Roots) error {
	for _, name := range c.AgentOwedFiles {
		// The registry declares basenames; the gate never trusts a declared
		// separator to steer a read outside the workspace.
		base := filepath.Base(name)
		path := filepath.Join(roots.Workspace, base)
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

// secondaryParseError reports why a declared JSON/NDJSON secondary does not
// parse, or "" when it does (or when its extension carries no parse rule).
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

// VerifiesDeclaredDeliverables marks the production contract gate as the
// ADR-0100 declared-deliverables gate for core's composition-root wiring
// proof (core.DeclaredDeliverablesGateWired). True whenever the gate is not
// switched off: at shadow/advisory it still verifies and logs would-block.
func (r *Reviewer) VerifiesDeclaredDeliverables() bool { return r.stage != config.StageOff }
