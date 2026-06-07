// Package deliverable is the shared verifier for phase-agent deliverables. The
// `evolve phase verify` self-check (cmd_phase_verify.go) and the host-side
// contract gate (reviewer.go) both call Verify so the agent's pre-finish check
// and the harness's post-phase gate run BYTE-IDENTICAL logic — they can never
// drift. Design: ADR-0034.
//
// Scope: WELL-FORMEDNESS ONLY (does the deliverable exist at the contracted
// path, in the right shape, with the required sections/keys and a parseable
// verdict). Semantic correctness — "is the report's content right" — is the
// auditor's LLM-judged job. A Verify PASS must never be read as a semantic PASS
// (the validation-vs-guardrail split; anti-Goodhart).
//
// Fail-open / fail-closed contract, encoded in the return signature:
//
//	err != nil       → ambiguity / infrastructure fault (unknown phase) → caller fails OPEN
//	err == nil, !OK  → confirmed agent violation                        → caller fails CLOSED
//	err == nil, OK   → well-formed
package deliverable

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Violation is one confirmed well-formedness failure with an actionable message.
type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result is the verifier verdict for one deliverable.
type Result struct {
	OK           bool        `json:"ok"`
	Phase        string      `json:"phase"`
	ArtifactPath string      `json:"artifact_path"`
	Violations   []Violation `json:"violations,omitempty"`
}

// Violation codes (stable; consumed by tests, the CLI, and the gate).
const (
	CodeMissingArtifact = "missing_artifact"
	CodeEmptyArtifact   = "empty_artifact"
	CodeMissingSection  = "missing_section"
	CodeBadVerdict      = "bad_verdict"
	CodeStrayInWorktree = "stray_in_worktree"
	CodeInvalidJSON     = "invalid_json"
	CodeMissingKey      = "missing_key"
	// CodeFailureContextMissing: a sentinel-declared FAIL/WARN lacks the
	// ADR-0039 structured failure block. (snake_case to match this closed
	// vocabulary; ADR prose spells it with hyphens.)
	CodeFailureContextMissing = "failure_context_missing"
)

// Verify runs the deterministic well-formedness checks for a phase's deliverable
// against the built-in phasecontract registry. See the package doc for the
// return contract. It is VerifyWith with the BuiltinResolver default —
// preserved so existing callers (and any path that only deals in built-in
// phases) are unchanged.
func Verify(phase string, roots phasecontract.Roots) (Result, error) {
	return VerifyWith(phase, roots, phasecontract.BuiltinResolver{})
}

// VerifyWith runs the well-formedness checks resolving the phase's contract
// through the given Resolver. A CatalogResolver lets user/minted phases be
// verified against a spec-derived contract (FromSpec) with no Go change, while
// built-ins stay authoritative. See the package doc for the return contract.
func VerifyWith(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver) (Result, error) {
	c, ok := resolver.Resolve(phase)
	if !ok {
		// Ambiguity: we cannot determine what "well-formed" means. Fail OPEN.
		return Result{}, fmt.Errorf("deliverable: no contract registered for phase %q", phase)
	}
	path := c.ArtifactPath(roots)
	res := Result{Phase: phase, ArtifactPath: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			res.add(CodeMissingArtifact, fmt.Sprintf("deliverable not found — write it to exactly: %s", path))
			// If the agent wrote it into the worktree instead, say so — that is
			// the actionable correction (the recoverBuildLeak failure class).
			checkStray(&res, c, roots)
			res.finish()
			return res, nil
		}
		// Unreadable for a reason other than absence (permissions, IO) is infra.
		return Result{}, fmt.Errorf("deliverable: read %s: %w", path, err)
	}
	content := string(data)
	if strings.TrimSpace(content) == "" {
		res.add(CodeEmptyArtifact, fmt.Sprintf("deliverable at %s is empty", path))
		res.finish()
		return res, nil
	}

	switch c.Kind {
	case phasecontract.KindJSON:
		verifyJSON(&res, c, content)
	default:
		verifyMarkdown(&res, c, content, roots)
	}
	res.finish()
	return res, nil
}

func verifyMarkdown(res *Result, c phasecontract.Contract, content string, roots phasecontract.Roots) {
	for _, s := range c.Sections {
		if !s.Present(content) {
			res.add(CodeMissingSection, fmt.Sprintf("required section %q is missing", s.Canonical))
		}
	}
	if len(c.Verdicts) > 0 && !verdictPresent(content, c.Verdicts) {
		res.add(CodeBadVerdict, fmt.Sprintf("no parseable verdict; expected one of %v", c.Verdicts))
	}
	// ADR-0039 §7: a sentinel-declared FAIL/WARN on a RequireFailureContext
	// contract must carry the structured failure block. Applies ONLY to
	// sentinel verdicts — legacy prose-only artifacts stay legal forever.
	// The message is the correction directive (re-dispatched verbatim).
	if c.RequireFailureContext {
		if s, ok := phasecontract.ParseVerdictSentinelFull(content); ok &&
			(s.Verdict == "FAIL" || s.Verdict == "WARN") &&
			(s.Failure == nil || s.Failure.Class == "") {
			res.add(CodeFailureContextMissing, fmt.Sprintf(
				"verdict %s declares no structured failure context — re-emit the evolve-verdict sentinel as schema_version 2 with a failure block: {\"class\":\"<failure class>\",\"defects\":[\"<one line per defect>\"],\"evidence_paths\":[\"<artifact>\"]}", s.Verdict))
		}
	}
	checkStray(res, c, roots)
}

// checkStray flags a deliverable the agent wrote into the worktree root instead
// of the workspace — the exact failure the recoverBuildLeak fixes
// (cb604d6/f96537c) chased reactively. Only meaningful for workspace-target
// contracts with a distinct worktree.
func checkStray(res *Result, c phasecontract.Contract, roots phasecontract.Roots) {
	if c.WriteTarget != phasecontract.TargetWorkspace {
		return
	}
	if roots.Worktree == "" || roots.Worktree == roots.Workspace {
		return
	}
	strayPath := joinWorktree(roots.Worktree, c.ArtifactName)
	if fileExists(strayPath) {
		res.add(CodeStrayInWorktree, fmt.Sprintf("a stray %s exists in the worktree (%s); the deliverable must live in the workspace", c.ArtifactName, strayPath))
	}
}

func verifyJSON(res *Result, c phasecontract.Contract, content string) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &top); err != nil {
		res.add(CodeInvalidJSON, fmt.Sprintf("not valid JSON object: %v", err))
		return
	}
	// Tolerant reader: only the minimal required keys are checked; unknown/future
	// keys are ignored (Postel's law + forward-compat).
	for _, k := range c.RequiredKeys {
		if _, ok := top[k]; !ok {
			res.add(CodeMissingKey, fmt.Sprintf("required key %q is missing", k))
		}
	}
}

// verdictPresent reports whether the deliverable declares an allowed verdict.
// Layer-5 strangler: the machine-readable sentinel is checked first; the prose
// scan is the fallback for reports written against older templates.
func verdictPresent(content string, verdicts []string) bool {
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		for _, allowed := range verdicts {
			if v == allowed {
				return true
			}
		}
		// A sentinel with an out-of-vocabulary verdict is not a valid declaration;
		// fall through to the prose scan rather than trusting it.
	}
	for _, v := range verdicts {
		if strings.Contains(content, v) {
			return true
		}
	}
	return false
}

func (r *Result) add(code, msg string) {
	r.Violations = append(r.Violations, Violation{Code: code, Message: msg})
}

func (r *Result) finish() { r.OK = len(r.Violations) == 0 }

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func joinWorktree(worktree, name string) string {
	return worktree + string(os.PathSeparator) + name
}
