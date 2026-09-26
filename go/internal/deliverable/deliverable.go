// Package deliverable is the one well-formedness verifier behind both the agent's
// `evolve phase verify` self-check and the host contract gate, so the two cannot drift.
// See docs/architecture/packages/internal-deliverable.md.
package deliverable

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
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
	// Content is the exact bytes this verdict was computed from, so a consumer never
	// re-reads the path; empty when the artifact was absent or blank, or declared no file.
	Content string `json:"-"`

	// Owed and Effects name the owed files (basenames) and effects this verdict checked,
	// so the verified signal never re-resolves the declaration.
	Owed    []string `json:"-"`
	Effects []string `json:"-"`
}

// Violation codes (stable; consumed by tests, the CLI, and the gate).
const (
	CodeMissingArtifact = "missing_artifact"
	CodeEmptyArtifact   = "empty_artifact"
	CodeMissingSection  = "missing_section"
	// CodeMissingChallengeToken: the report does not echo the minted challenge token (proof of read).
	CodeMissingChallengeToken = "missing_challenge_token"
	CodeBadVerdict            = "bad_verdict"
	CodeStrayInWorktree       = "stray_in_worktree"
	CodeInvalidJSON           = "invalid_json"
	CodeMissingKey            = "missing_key"
	// CodeFailureContextMissing: a sentinel-declared FAIL/WARN lacks the structured failure block.
	CodeFailureContextMissing = "failure_context_missing"
	// CodeFailureClassUnknown: the failure block's class is outside the failurelog vocabulary.
	CodeFailureClassUnknown = "failure_class_unknown"
	// Agent-owed secondaries: one code per class so the ladder's same-defect identity sees a repeat.
	CodeMissingSecondary   = "missing_secondary"
	CodeEmptySecondary     = "empty_secondary"
	CodeMalformedSecondary = "malformed_secondary"
	// Declared effects: missing_effect is an unperformed effect, unbound_effect a registry name no check binds.
	CodeMissingEffect = "missing_effect"
	CodeUnboundEffect = "unbound_effect"
)

// Verify checks a phase's deliverable against the built-in registry: an error is ambiguity (fail open), !OK a confirmed violation (fail closed).
func Verify(phase string, roots phasecontract.Roots) (Result, error) {
	return VerifyWith(phase, roots, phasecontract.BuiltinResolver{})
}

// VerifyWith is Verify resolving the contract through resolver, so user and minted phases verify against their spec.
func VerifyWith(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver) (Result, error) {
	return VerifyWithStage(phase, roots, resolver, config.StageOff)
}

// VerifyWithStage is VerifyWith with the EVOLVE_PHASE_IO stage, which gates only the RequireFailureContextPhaseIO check.
func VerifyWithStage(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver, phaseIO config.Stage) (Result, error) {
	c, ok := resolver.Resolve(phase)
	if !ok {
		// Without a contract there is no definition of well-formed: fail open.
		return Result{}, fmt.Errorf("deliverable: no contract registered for phase %q", phase)
	}
	if c.NoArtifact {
		// ship's deliverable is the pushed commit, which the ship and commit gates verify.
		return Result{Phase: phase, OK: true}, nil
	}
	res, err := verifyPrimary(phase, c, roots, phaseIO)
	if err != nil {
		return Result{}, err
	}
	// Secondaries, then effects, after the primary: one correction names everything still owed.
	if err := verifySecondaries(&res, c, roots); err != nil {
		return Result{}, err
	}
	if err := verifyEffects(&res, c, roots); err != nil {
		return Result{}, err
	}
	res.finish()
	return res, nil
}

// verifyPrimary checks the primary artifact and returns the Result unfinished, for the secondary and effect checks.
func verifyPrimary(phase string, c phasecontract.Contract, roots phasecontract.Roots, phaseIO config.Stage) (Result, error) {
	path := c.ArtifactPath(roots)
	// The runner owns which file this run was asked to write; the contract still owns its shape.
	if roots.DispatchedArtifact != "" {
		path = roots.DispatchedArtifact
	}
	res := Result{Phase: phase, ArtifactPath: path}

	content, exists, err := readDeliverableWithGrace(path)
	if err != nil {
		// A read fault other than absence is infra ambiguity, not an agent violation.
		return Result{}, fmt.Errorf("deliverable: read %s: %w", path, err)
	}
	res.Content = content
	if !exists {
		res.add(CodeMissingArtifact, fmt.Sprintf("deliverable not found — write it to exactly: %s", path))
		checkStray(&res, c, roots)
		return res, nil
	}
	if strings.TrimSpace(content) == "" {
		res.add(CodeEmptyArtifact, fmt.Sprintf("deliverable at %s is empty", path))
		return res, nil
	}

	switch c.Kind {
	case phasecontract.KindJSON:
		verifyJSON(&res, c, content)
	default:
		verifyMarkdown(&res, c, content, roots, phaseIO)
	}
	return res, nil
}

// A final write is not atomic with the verify after it, so absence or emptiness is provisional for readGraceWindow.
// Partial content is closed upstream: completion.go waits artifactStableTicks consecutive polls of an unchanged (size, mtime) key.
const (
	readGraceWindow = 500 * time.Millisecond
	readGracePoll   = 20 * time.Millisecond
)

// graceSleep is a test seam, not a dial.
var graceSleep = time.Sleep

// readDeliverableWithGrace reads first and polls only while the file is absent or blank. The window delays
// a missing or empty verdict but never changes it, and a non-absence read fault returns at once as infra.
func readDeliverableWithGrace(path string) (content string, exists bool, err error) {
	deadline := time.Now().Add(readGraceWindow)
	for {
		data, rerr := os.ReadFile(path)
		switch {
		case rerr == nil && strings.TrimSpace(string(data)) != "":
			return string(data), true, nil
		case rerr != nil && !os.IsNotExist(rerr):
			return "", false, rerr
		}
		if !time.Now().Before(deadline) {
			return string(data), rerr == nil, nil
		}
		graceSleep(readGracePoll)
	}
}

func verifyMarkdown(res *Result, c phasecontract.Contract, content string, roots phasecontract.Roots, phaseIO config.Stage) {
	for _, s := range c.Sections {
		if !s.Present(content) {
			res.add(CodeMissingSection, fmt.Sprintf("required section %q is missing", s.Canonical))
		}
	}
	// reportdoc.HasSection is the audit gate's own predicate, so a report that gate refuses cannot pass here.
	if roots.ExplanationDocumentationVersion != 0 {
		for _, s := range c.ExplanationSections {
			if !reportdoc.HasSection(content, s.Title()) {
				res.add(CodeMissingSection, fmt.Sprintf("required section %q is missing (the explanation-documentation contract v%d is active for this cycle — review the Build explanation document and emit the section in the shape of the explanation-documentation-review reference's contract example)", s.Canonical, roots.ExplanationDocumentationVersion))
			}
		}
	}
	if len(c.Verdicts) > 0 && !verdictPresent(content, c.Verdicts, phaseIO) {
		res.add(CodeBadVerdict, fmt.Sprintf("no parseable verdict; expected one of %v", c.Verdicts))
	}
	// A sentinel FAIL/WARN owes the structured failure block; PhaseIO phases owe it only at enforce, prose-only verdicts never.
	if c.RequireFailureContext || (c.RequireFailureContextPhaseIO && phaseIO >= config.StageEnforce) {
		s, ok := phasecontract.ParseVerdictSentinelFull(content)
		isFailOrWarn := ok && (s.Verdict == "FAIL" || s.Verdict == "WARN")
		// Only the audit's class feeds a decision (retry envelope, failure record); NormalizeLegacy's aliases pass on purpose.
		if c.RequireFailureContext && isFailOrWarn && s.Failure != nil && s.Failure.Class != "" && failurelog.NormalizeLegacy(s.Failure.Class) == failurelog.UnknownClassification {
			res.add(CodeFailureClassUnknown, fmt.Sprintf(
				"verdict %s declares failure class %q, which is not in the failure vocabulary — re-emit the evolve-verdict sentinel with \"class\" set to one of [%s] (on FAIL the class drives the retry envelope — an unknown class forfeits the direct repair; on FAIL and WARN it is what the failure record and the carryover read). Keep your judgment in the defects and prescription entries.",
				s.Verdict, s.Failure.Class, failurelog.VocabularyList()))
		}
		if isFailOrWarn && (s.Failure == nil || s.Failure.Class == "") {
			res.add(CodeFailureContextMissing, fmt.Sprintf(
				"verdict %s declares no structured failure context — re-emit the evolve-verdict sentinel as schema_version 2 with a failure block: {\"class\":\"<failure class>\",\"defects\":[\"<one line per defect>\"],\"evidence_paths\":[\"<artifact>\"]}", s.Verdict))
		}
	}
	// Checked here so the correction ladder can fix a missing echo before audit; no minted token owes nothing.
	if c.RequireChallengeToken {
		if tok, err := os.ReadFile(filepath.Join(roots.Workspace, "challenge-token.txt")); err == nil {
			if t := strings.TrimSpace(string(tok)); t != "" && !strings.Contains(content, t) {
				res.add(CodeMissingChallengeToken, fmt.Sprintf(
					"report does not echo the challenge token — copy it verbatim into the report (e.g. as the comment <!-- challenge-token: %s -->) to prove the upstream report was read", t))
			}
		}
	}
	checkStray(res, c, roots)
}

// checkStray flags a deliverable written into the worktree root instead of the workspace.
func checkStray(res *Result, c phasecontract.Contract, roots phasecontract.Roots) {
	if c.WriteTarget != phasecontract.TargetWorkspace {
		return
	}
	if roots.Worktree == "" || roots.Worktree == roots.Workspace {
		return
	}
	// Name the dispatched file: delta-mode intent writes intent-delta.md, not intent.md.
	name := c.ArtifactName
	if roots.DispatchedArtifact != "" {
		name = filepath.Base(roots.DispatchedArtifact)
	}
	strayPath := joinWorktree(roots.Worktree, name)
	if fileExists(strayPath) {
		res.add(CodeStrayInWorktree, fmt.Sprintf("a stray %s exists in the worktree (%s); the deliverable must live in the workspace", name, strayPath))
	}
}

func verifyJSON(res *Result, c phasecontract.Contract, content string) {
	data := []byte(content)
	if !json.Valid(data) {
		res.add(CodeInvalidJSON, "not valid JSON")
		return
	}

	shape := c.TopLevelJSONShape()
	trimmed := strings.TrimSpace(content)
	if shape == phasecontract.JSONShapeObject && !strings.HasPrefix(trimmed, "{") {
		res.add(CodeInvalidJSON, "expected a JSON object at the top level")
		return
	}
	if shape == phasecontract.JSONShapeArray && !strings.HasPrefix(trimmed, "[") {
		res.add(CodeInvalidJSON, "expected a JSON array at the top level")
		return
	}
	if len(c.RequiredKeys) == 0 {
		return
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		res.add(CodeInvalidJSON, fmt.Sprintf("not valid JSON object: %v", err))
		return
	}
	// Tolerant reader: unknown keys are ignored for forward compatibility.
	for _, k := range c.RequiredKeys {
		if _, ok := top[k]; !ok {
			res.add(CodeMissingKey, fmt.Sprintf("required key %q is missing", k))
		}
	}
}

// verdictPresent reports whether content declares an allowed verdict: the sentinel first, then the legacy
// prose scan, which is off at enforce so an out-of-vocabulary sentinel gets no prose rescue.
// See ADR-0050.
func verdictPresent(content string, verdicts []string, phaseIO config.Stage) bool {
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		for _, allowed := range verdicts {
			if v == allowed {
				return true
			}
		}
	}
	if phaseIO < config.StageEnforce {
		for _, v := range verdicts {
			if strings.Contains(content, v) {
				return true
			}
		}
	}
	return false
}

func (r *Result) add(code, msg string) {
	r.Violations = append(r.Violations, Violation{Code: code, Message: msg})
}

func (r *Result) finish() { r.OK = len(r.Violations) == 0 }

// onlyViolation reports whether r fails solely on code; salvage and the warn-only size gate need sole, never membership.
func (r Result) onlyViolation(code string) bool {
	if len(r.Violations) == 0 {
		return false
	}
	for _, v := range r.Violations {
		if v.Code != code {
			return false
		}
	}
	return true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func joinWorktree(worktree, name string) string {
	return worktree + string(os.PathSeparator) + name
}
