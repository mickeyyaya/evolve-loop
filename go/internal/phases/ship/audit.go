package ship

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
	"github.com/mickeyyaya/evolve-loop/go/internal/treestate"
)

// auditEntry is the auditor ledger row ship binds to.
type auditEntry = auditledger.Entry

// verifyAuditBinding implements the full audit-binding contract.
// res.Provenance is set on success; integrity errors return *IntegrityError.
//
// Sets opts.internalAuditBoundTreeSHA from the ledger's worktree_tree_sha for
// the gitops layer's pre-commit and post-push tree checks.
func verifyAuditBinding(ctx context.Context, opts *Options, res *RunResult) error {
	ledgerPath := filepath.Join(opts.ProjectRoot, ".evolve", "ledger.jsonl")
	entry, err := findLatestAudit(ledgerPath, opts.RunID)
	if err != nil {
		return err
	}

	body, err := readAuditArtifact(entry)
	if err != nil {
		return err
	}
	pass, warn, fail := parseVerdicts(string(body), opts.PhaseIO)

	if fail && pass {
		return shipErr(core.CodeAuditBindingDualVerdict, core.ShipClassPrecondition, core.StageVerifyClass,
			"audit-report.md declares BOTH 'Verdict: FAIL' AND 'Verdict: PASS' — auditor produced an inconsistent artifact. Re-run audit, or split into separate Verdict and per-eval-result sections.",
			"artifact_path", entry.ArtifactPath)
	}
	switch {
	case fail:
		return shipErr(core.CodeAuditBindingVerdictFail, core.ShipClassPrecondition, core.StageVerifyClass,
			"audit-report.md declares 'Verdict: FAIL' — auditor explicitly rejected this build",
			"artifact_path", entry.ArtifactPath)
	case pass:
	case warn:
		if policy.StrictAuditFor(opts.ProjectRoot) {
			return shipErr(core.CodeAuditBindingVerdictWarn, core.ShipClassPrecondition, core.StageVerifyClass,
				"audit-report.md declares 'Verdict: WARN' and policy.json workflow.strict_audit is set — strict mode rejects WARN",
				"artifact_path", entry.ArtifactPath)
		}
		res.Logs = append(res.Logs,
			"[ship] audit verdict: WARN — shipping per fluent-by-default policy (set workflow.strict_audit in .evolve/policy.json to block on WARN)",
		)
	default:
		return shipErr(core.CodeAuditBindingMalformed, core.ShipClassPrecondition, core.StageVerifyClass,
			"audit-report.md declares no recognizable verdict (PASS/WARN/FAIL) — auditor output malformed",
			"artifact_path", entry.ArtifactPath)
	}

	opts.internalAuditBoundTreeSHA = entry.WorktreeTreeSHA
	opts.internalAuditArtifactSHA = entry.ArtifactSHA256

	if entry.GitHEAD == "" || entry.TreeStateSHA == "" {
		return shipErr(core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, core.StageVerifyClass,
			"Auditor ledger entry predates v8.13.0 cycle-binding (no git_head/tree_state_sha) — re-run audit")
	}
	// The report SHA above binds the host receipt; the mutable verdict must also
	// match its exact execution identity and bytes before it can authorize ship.
	if err := verifyPredicateReceipt(opts, entry, string(body), res); err != nil {
		return err
	}

	testedRoot := opts.ActiveWorktree
	if testedRoot == "" {
		testedRoot = opts.ProjectRoot
	}
	currentHEAD, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	currentHEAD = strings.TrimSpace(currentHEAD)
	if currentHEAD != entry.GitHEAD {
		// The composition entry binds the composed tree already, so the
		// base-tree check below is superseded when it carries forward.
		carried, cfErr := tryTrivialRebaseCarryForward(ctx, opts, res, ledgerPath, entry, currentHEAD)
		if cfErr != nil {
			return cfErr
		}
		if !carried {
			return shipErr(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, core.StageVerifyClass,
				fmt.Sprintf("git HEAD has moved since audit (audited=%s current=%s) — re-run Auditor on the new state", entry.GitHEAD, currentHEAD),
				"audited", entry.GitHEAD, "current", currentHEAD)
		}
	} else if filepath.Clean(testedRoot) == filepath.Clean(opts.ProjectRoot) {
		currentTree, err := computeTreeStateSHA(ctx, opts)
		if err != nil {
			return err
		}
		if currentTree != entry.TreeStateSHA {
			return shipErr(core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, core.StageVerifyClass,
				"uncommitted changes have been added since audit (tree-state mismatch) — re-run Auditor",
				"audited_tree", entry.TreeStateSHA, "current_tree", currentTree)
		}
	}

	if err := verifyExecutionTree(ctx, opts, res, testedRoot); err != nil {
		return err
	}

	fi, err := os.Stat(entry.ArtifactPath)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: stat audit-report.md: "+err.Error(), "artifact_path", entry.ArtifactPath)
	}
	age := opts.NowFn().Unix - fi.ModTime().Unix()
	const maxAge = 7 * 24 * 3600
	if age > maxAge {
		return shipErr(core.CodeAuditBindingStale, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("audit-report.md is %ds old (>%ds); re-run Auditor", age, maxAge),
			"age_seconds", fmt.Sprintf("%d", age), "max_age_seconds", fmt.Sprintf("%d", maxAge))
	}

	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: audit verified — verdict PASS, SHA matches, HEAD/tree bound to audit, age %ds", age))
	return nil
}

func verifyExecutionTree(ctx context.Context, opts *Options, res *RunResult, testedRoot string) error {
	audited := opts.internalAuditBoundTreeSHA
	current, err := treefence.Take(ctx, testedRoot)
	if err == nil && current.Tree == audited {
		return nil
	}
	unexplained := ""
	if err == nil {
		ok, detail := auditBindingSatisfied(ctx, opts, testedRoot, current.Tree)
		if ok {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: predicate execution tree drift (audit=%s current=%s) explained%s — accepted", audited, current.Tree, detail))
			return nil
		}
		unexplained = detail
	}
	return shipErr(core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, core.StageVerifyClass,
		fmt.Sprintf("predicate execution tree-state mismatch or unavailable after Audit (audited=%s current=%s error=%v)%s; re-run Audit", audited, current.Tree, err, unexplained), "audited_tree", audited, "current_tree", current.Tree)
}

// findLatestAudit returns the auditor ledger entry ship binds to: the newest
// auditor row of opts.RunID's run, or of any run when runID is empty. A miss is
// an integrity stop, never a fallback to another run's audit.
func findLatestAudit(ledgerPath, runID string) (*auditEntry, error) {
	entry, err := auditledger.LatestAuditorEntry(ledgerPath, runID)
	switch {
	case err == nil:
		return &entry, nil
	case errors.Is(err, auditledger.ErrNoAuditorForRun):
		return nil, shipErr(core.CodeAuditBindingNoAuditor, core.ShipClassPrecondition, core.StageVerifyClass,
			err.Error()+" — independent review missing",
			"ledger_path", ledgerPath)
	case errors.Is(err, os.ErrNotExist):
		return nil, shipErr(core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("no ledger at %s — no Auditor has ever run", ledgerPath), "ledger_path", ledgerPath)
	default:
		return nil, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: "+err.Error(), "ledger_path", ledgerPath)
	}
}

// parseVerdicts recognizes two verdict shapes in the audit report: an inline
// `Verdict: <X>` line, or a heading-style `# Verdict` followed by `**X**`
// within 5 lines.
func parseVerdicts(body string, stage config.Stage) (pass, warn, fail bool) {
	if stage >= config.StageEnforce {
		// See ADR-0050.
		if s, ok := phasecontract.ParseVerdictSentinelFull(body); ok && s.Phase == string(core.PhaseAudit) {
			switch s.Verdict {
			case core.VerdictPASS:
				pass = true
			case core.VerdictWARN:
				warn = true
			case core.VerdictFAIL:
				fail = true
			}
		}
		return
	}
	pass = hasVerdict(body, "PASS")
	warn = hasVerdict(body, "WARN")
	fail = hasVerdict(body, "FAIL")
	return
}

// inlineVerdictRe matches lines like:
//
//	Verdict: PASS
//	Verdict:  **PASS**
//	**Verdict: PASS**
//
// The pattern is case-insensitive on the verdict word and allows
// surrounding asterisks.
var inlineVerdictRe = map[string]*regexp.Regexp{
	"PASS": regexp.MustCompile(`(?i)Verdict\s*:\s*\*?\*?\s*PASS(\s|$|\*)`),
	"WARN": regexp.MustCompile(`(?i)Verdict\s*:\s*\*?\*?\s*WARN(\s|$|\*)`),
	"FAIL": regexp.MustCompile(`(?i)Verdict\s*:\s*\*?\*?\s*FAIL(\s|$|\*)`),
}

// headingVerdictRe matches the `## Verdict` heading followed, within 5
// lines, by either `**X**` or a bare verdict line (exactly `X`; a sentence
// containing the word must not match).
var headingVerdictRe = map[string]*regexp.Regexp{
	"PASS": regexp.MustCompile(`(?m)^#+[ \t]+Verdict[ \t]*\n(?:.*\n){0,4}(?:.*\*\*PASS\*\*|[ \t]*PASS[ \t]*$)`),
	"WARN": regexp.MustCompile(`(?m)^#+[ \t]+Verdict[ \t]*\n(?:.*\n){0,4}(?:.*\*\*WARN\*\*|[ \t]*WARN[ \t]*$)`),
	"FAIL": regexp.MustCompile(`(?m)^#+[ \t]+Verdict[ \t]*\n(?:.*\n){0,4}(?:.*\*\*FAIL\*\*|[ \t]*FAIL[ \t]*$)`),
}

func hasVerdict(body, verdict string) bool {
	if inlineVerdictRe[verdict].MatchString(body) {
		return true
	}
	return headingVerdictRe[verdict].MatchString(body)
}

// readAuditArtifact checks the exact bytes consumed below, avoiding a second
// read between SHA verification and interpretation.
func readAuditArtifact(entry *auditEntry) ([]byte, error) {
	switch entry.ExitCode {
	case 0, 1:
	default:
		return nil, shipErr(core.CodeAuditBindingAuditorExit, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("most recent Auditor exited %d (error state — not a Unix-convention findings signal)", entry.ExitCode),
			"auditor_exit_code", fmt.Sprintf("%d", entry.ExitCode))
	}

	body, err := os.ReadFile(entry.ArtifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, shipErr(core.CodeAuditBindingArtifactMissing, core.ShipClassPrecondition, core.StageVerifyClass,
				"audit-report.md missing on disk: "+entry.ArtifactPath, "artifact_path", entry.ArtifactPath)
		}
		return nil, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read audit-report.md: "+err.Error(), "artifact_path", entry.ArtifactPath)
	}
	actualSHA := fmt.Sprintf("%x", sha256.Sum256(body))
	if actualSHA != entry.ArtifactSHA256 {
		return nil, shipErr(core.CodeAuditBindingArtifactSHA, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("audit-report.md SHA mismatch (ledger=%s actual=%s) — artifact mutated post-audit", entry.ArtifactSHA256, actualSHA),
			"ledger_sha", entry.ArtifactSHA256, "actual_sha", actualSHA, "artifact_path", entry.ArtifactPath)
	}
	return body, nil
}

func verifyPostPushPredicateEvidence(ctx context.Context, opts *Options, res *RunResult, commit string) error {
	entry, err := findLatestAudit(filepath.Join(opts.ProjectRoot, ".evolve", "ledger.jsonl"), opts.RunID)
	if err != nil {
		return err
	}
	body, err := readAuditArtifact(entry)
	if err != nil {
		return err
	}
	landedTree, err := captureGitOutput(ctx, opts, "rev-parse", commit+"^{tree}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(landedTree) != entry.WorktreeTreeSHA {
		return shipErr(core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, core.StageVerifyClass,
			"landed commit differs from the host predicate execution tree; re-run Audit")
	}

	return verifyPredicateReceipt(opts, entry, string(body), res)
}

// checkEGPSGate refuses incomplete or internally contradictory evidence.
func checkEGPSGate(path string, res *RunResult) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, shipErr(core.CodeAuditBindingMalformed, core.ShipClassPrecondition, core.StageVerifyClass,
			"ship predicate evidence unavailable; re-run Audit: "+err.Error(), "path", path)
	}
	v, err := acssuite.ReadVerdict(raw)
	if err != nil {
		return nil, shipErr(core.CodeAuditBindingMalformed, core.ShipClassPrecondition, core.StageVerifyClass,
			"ship predicate evidence invalid; re-run Audit: "+err.Error(), "path", path)
	}
	if err := checkPredicateResult(v, res); err != nil {
		return nil, err
	}
	return raw, nil
}

func checkPredicateResult(v acssuite.Verdict, res *RunResult) error {
	if v.RedCount != 0 {
		return shipErr(core.CodeEGPSRedCount, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("EGPS predicate suite has %d RED predicate(s): %s (acs-verdict.json verdict=%s total=%d)",
				v.RedCount, strings.Join(v.RedIDs, ","), v.Verdict, v.PredicateSuite.Total),
			"red_count", fmt.Sprintf("%d", v.RedCount), "red_ids", strings.Join(v.RedIDs, ","),
			"verdict", v.Verdict, "total", fmt.Sprintf("%d", v.PredicateSuite.Total))
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: EGPS predicate suite verdict=%s (green=%d skip=%d total=%d)", v.Verdict, v.GreenCount, v.SkipCount, v.PredicateSuite.Total))
	return nil
}

func verifyPredicateReceipt(opts *Options, entry *auditEntry, report string, res *RunResult) error {
	path := filepath.Join(filepath.Dir(entry.ArtifactPath), acsverdict.Filename)
	raw, err := checkEGPSGate(path, res)
	if err != nil {
		return err
	}
	_, err = acssuite.VerifyEvidence(report, raw, acssuite.EvidenceIdentity{
		Cycle: opts.CycleID, RunID: opts.RunID, Round: opts.AuditRound, TreeSHA: entry.WorktreeTreeSHA,
	})
	if err == nil {
		return nil
	}
	return shipErr(core.CodeAuditBindingMalformed, core.ShipClassPrecondition, core.StageVerifyClass,
		"ship predicate evidence invalid; re-run Audit: "+err.Error(), "path", path)
}

// computeTreeStateSHA computes sha256(git diff HEAD): a tracked-file
// mutation after audit invalidates the ship.
func computeTreeStateSHA(ctx context.Context, opts *Options) (string, error) {
	sum, err := treestate.SHA(ctx, opts.runner(), opts.ProjectRoot, os.Environ())
	if err != nil {
		var re *treestate.RunError
		if errors.As(err, &re) && re.Err == nil {
			// Fatal git exit (>1) — rc=1 (differences) is handled inside SHA.
			return "", shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
				fmt.Sprintf("ship: git diff HEAD exit %d", re.ExitCode), "git_rc", fmt.Sprintf("%d", re.ExitCode))
		}
		runnerErr := underlyingErr(err)
		return "", shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: git diff HEAD: "+runnerErr.Error(), "git_err", runnerErr.Error())
	}
	return sum, nil
}

// underlyingErr returns the runner error carried by a treestate.RunError, or
// the error itself otherwise, so the "git_err" field stays the raw runner
// message.
func underlyingErr(err error) error {
	var re *treestate.RunError
	if errors.As(err, &re) && re.Err != nil {
		return re.Err
	}
	return err
}
