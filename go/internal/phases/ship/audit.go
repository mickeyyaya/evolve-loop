// audit.go — audit-binding verification for --class cycle.
//
// Mirrors ship.sh sections 3-6 (lines 396-575):
//
//  3. Locate most recent Auditor ledger entry (kind=agent_subprocess, role=auditor)
//  4. Verify exit_code ∈ {0,1}, artifact exists, SHA matches
//     4b. Parse Verdict from artifact: PASS/WARN/FAIL with dual-detection
//     4c. EGPS predicate suite gate: acs-verdict.json:red_count == 0
//  5. Cycle binding: current git HEAD + tree must match ledger entry
//  6. Freshness: artifact age < 7 days
package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
	"github.com/mickeyyaya/evolveloop/go/internal/treestate"
)

// auditEntry is the subset of LedgerEntry fields ship cares about.
type auditEntry struct {
	Role            string `json:"role"`
	Kind            string `json:"kind"`
	RunID           string `json:"run_id,omitempty"` // ADR-0049 S4 / G5: run-scope the binding lookup
	ExitCode        int    `json:"exit_code"`
	ArtifactPath    string `json:"artifact_path"`
	ArtifactSHA256  string `json:"artifact_sha256"`
	GitHEAD         string `json:"git_head"`
	TreeStateSHA    string `json:"tree_state_sha"`
	WorktreeTreeSHA string `json:"worktree_tree_sha"`
}

// verifyAuditBinding implements the full audit-binding contract.
// res.Provenance is set on success; integrity errors return *IntegrityError.
//
// Sets opts internal-ish state via res.Logs and (indirectly via writeStateMap)
// for downstream phases. Returns the audit_bound_tree_sha (if present in
// the artifact) for the gitops layer's pre-merge check.
func verifyAuditBinding(ctx context.Context, opts *Options, res *RunResult) error {
	ledgerPath := filepath.Join(opts.ProjectRoot, ".evolve", "ledger.jsonl")
	entry, err := findLatestAudit(ledgerPath, opts.RunID)
	if err != nil {
		return err
	}

	// 4. Exit code: 0|1 ok, 2+ is true error.
	switch entry.ExitCode {
	case 0, 1:
		// fall through
	default:
		return shipErr(core.CodeAuditBindingAuditorExit, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("most recent Auditor exited %d (error state — not a Unix-convention findings signal)", entry.ExitCode),
			"auditor_exit_code", fmt.Sprintf("%d", entry.ExitCode))
	}

	// 4. Artifact existence + SHA.
	if _, err := os.Stat(entry.ArtifactPath); err != nil {
		return shipErr(core.CodeAuditBindingArtifactMissing, core.ShipClassPrecondition, core.StageVerifyClass,
			"audit-report.md missing on disk: "+entry.ArtifactPath, "artifact_path", entry.ArtifactPath)
	}
	actualSHA, err := sha256File(entry.ArtifactPath)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: SHA audit-report.md: "+err.Error(), "artifact_path", entry.ArtifactPath)
	}
	if actualSHA != entry.ArtifactSHA256 {
		return shipErr(core.CodeAuditBindingArtifactSHA, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("audit-report.md SHA mismatch (ledger=%s actual=%s) — artifact mutated post-audit", entry.ArtifactSHA256, actualSHA),
			"ledger_sha", entry.ArtifactSHA256, "actual_sha", actualSHA, "artifact_path", entry.ArtifactPath)
	}

	// 4b. Verdict parse with dual-verdict detection (v8.30.0).
	body, err := os.ReadFile(entry.ArtifactPath)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read audit-report.md: "+err.Error(), "artifact_path", entry.ArtifactPath)
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
		// clean ship
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

	// Extract audit_bound_tree_sha for the gitops pre/post-merge tree-drift check.
	// Source priority: the orchestrator's ledger binding entry (WorktreeTreeSHA =
	// the worktree CHANGES tree it will commit) WINS over the auditor's report
	// comment, because the auditor persona binds HEAD^{tree} = the unchanged base
	// (the cycle's changes are uncommitted in the worktree at audit time), which
	// can never equal the changes-commit tree → INTEGRITY_TREE_DRIFT every cycle
	// (cycle-152). The report comment is the fallback for the non-worktree flow.
	if entry.WorktreeTreeSHA != "" {
		opts.internalAuditBoundTreeSHA = entry.WorktreeTreeSHA
	} else if m := auditBoundTreeSHARe.FindStringSubmatch(string(body)); m != nil {
		opts.internalAuditBoundTreeSHA = strings.TrimSpace(strings.Trim(m[1], "`"))
	}

	// 4c. EGPS predicate gate (acs-verdict.json:red_count == 0).
	egpsPath := filepath.Join(filepath.Dir(entry.ArtifactPath), "acs-verdict.json")
	if err := checkEGPSGate(egpsPath, res); err != nil {
		return err
	}

	// 5. Cycle binding: current HEAD/tree must match ledger entry.
	if entry.GitHEAD == "" || entry.TreeStateSHA == "" {
		return shipErr(core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, core.StageVerifyClass,
			"Auditor ledger entry predates v8.13.0 cycle-binding (no git_head/tree_state_sha) — re-run audit")
	}
	currentHEAD, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	currentHEAD = strings.TrimSpace(currentHEAD)
	if currentHEAD != entry.GitHEAD {
		return shipErr(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, core.StageVerifyClass,
			fmt.Sprintf("git HEAD has moved since audit (audited=%s current=%s) — re-run Auditor on the new state", entry.GitHEAD, currentHEAD),
			"audited", entry.GitHEAD, "current", currentHEAD)
	}
	currentTree, err := computeTreeStateSHA(ctx, opts)
	if err != nil {
		return err
	}
	if currentTree != entry.TreeStateSHA {
		return shipErr(core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, core.StageVerifyClass,
			"uncommitted changes have been added since audit (tree-state mismatch) — re-run Auditor",
			"audited_tree", entry.TreeStateSHA, "current_tree", currentTree)
	}

	// 6. Freshness (7d cap when cycle-bound).
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

var auditBoundTreeSHARe = regexp.MustCompile(`(?m)^audit_bound_tree_sha:\s*` + "`?" + `([0-9a-f]+)` + "`?")

// findLatestAudit walks ledger.jsonl backwards, returning the most
// recent agent_subprocess entry with role=auditor.
//
// Missing/empty ledger → IntegrityError. Found-but-no-auditor →
// IntegrityError. Any unmarshal error on a candidate line is treated as
// "not an auditor entry" (forward-compat: alien lines should not crash
// ship-gate).
// findLatestAudit returns the auditor ledger entry ship binds to. When runID is
// set, it prefers the latest auditor entry stamped with THIS run (ADR-0049 S4 /
// gap G5), so a concurrent run's later auditor entry can't be bound; it falls
// back to the latest auditor entry overall when none matches (standalone
// runID=="" or a legacy/unstamped entry), preserving pre-S4 behavior with zero
// regression. No-op for the live loop: today's single run's entry IS the latest,
// so the exact match returns the same entry.
func findLatestAudit(ledgerPath, runID string) (*auditEntry, error) {
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, shipErr(core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, core.StageVerifyClass,
				fmt.Sprintf("no ledger at %s — no Auditor has ever run", ledgerPath), "ledger_path", ledgerPath)
		}
		return nil, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read ledger: "+err.Error(), "ledger_path", ledgerPath)
	}
	lines := strings.Split(string(raw), "\n")
	var latestAny *auditEntry // latest auditor entry regardless of run (fallback)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var e auditEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Kind != "agent_subprocess" || e.Role != "auditor" {
			continue
		}
		if runID == "" || e.RunID == runID {
			entry := e
			return &entry, nil
		}
		if latestAny == nil {
			entry := e
			latestAny = &entry
		}
	}
	if latestAny != nil {
		return latestAny, nil
	}
	return nil, shipErr(core.CodeAuditBindingNoAuditor, core.ShipClassPrecondition, core.StageVerifyClass,
		"no Auditor ledger entry found — independent review missing", "ledger_path", ledgerPath)
}

// parseVerdicts grep-and-awk's the audit report for PASS/WARN/FAIL.
// Mirrors the bash has_pass/has_warn/has_fail logic:
//
//  1. Inline `Verdict: <X>` (case-insensitive, optional asterisks)
//  2. Heading-style: `# Verdict\n**X**` (within 5 lines)
func parseVerdicts(body string, stage config.Stage) (pass, warn, fail bool) {
	if stage >= config.StageEnforce {
		// ADR-0050 §3.10 Slice 6: sentinel-first at enforce. The machine-readable
		// verdict is authoritative and single-valued, so the prose regex below
		// (which can match multiple verdict words and trip the dual-verdict guard at
		// audit.go) is gated off. No usable sentinel → all false →
		// CodeAuditBindingMalformed, i.e. the sentinel becomes mandatory.
		//
		// The sentinel MUST be the audit phase's own: this is a ship gate, so a
		// foreign-phase sentinel (e.g. a build-report sentinel quoted into the
		// audit artifact) must not be allowed to satisfy it. ParseVerdictSentinelFull
		// surfaces the phase field; only an exact "audit" phase is trusted. SKIPPED
		// and any out-of-vocab verdict also fall through to all-false (malformed).
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
// surrounding asterisks. Mirrors the bash grep -qiE pattern.
var inlineVerdictRe = map[string]*regexp.Regexp{
	"PASS": regexp.MustCompile(`(?i)Verdict\s*:\s*\*?\*?\s*PASS(\s|$|\*)`),
	"WARN": regexp.MustCompile(`(?i)Verdict\s*:\s*\*?\*?\s*WARN(\s|$|\*)`),
	"FAIL": regexp.MustCompile(`(?i)Verdict\s*:\s*\*?\*?\s*FAIL(\s|$|\*)`),
}

// headingVerdictRe matches the `## Verdict` heading followed, within 5
// lines, by either `**X**` (bash awk window parity) or a BARE verdict line
// (exactly `X` — the cycle-249 shape; a sentence containing the word must
// not match).
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

// checkEGPSGate enforces acs-verdict.json:red_count == 0 when the file
// exists. Missing file is a pre-v10.0.0 bootstrap (no predicates yet) —
// fluent posture from audit-report.md still applies.
func checkEGPSGate(path string, res *RunResult) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read acs-verdict.json: "+err.Error(), "path", path)
	}
	var v struct {
		RedCount       int      `json:"red_count"`
		GreenCount     int      `json:"green_count"`
		SkipCount      int      `json:"skip_count"`
		Verdict        string   `json:"verdict"`
		RedIDs         []string `json:"red_ids"`
		PredicateSuite struct {
			Total int `json:"total"`
		} `json:"predicate_suite"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		// Malformed acs-verdict.json: don't block (bash falls through silently).
		return nil
	}
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

// computeTreeStateSHA computes sha256(git diff HEAD) — the same
// fingerprint the bash Auditor records. This is the audit-binding
// model: tracked-file mutations after audit invalidate ship.
//
// The git-run + hashing live in the shared internal/treestate package so the
// commit-gate attestation reader (verifyCommitGate) and the audit-binding
// verifier hash byte-identically; this thin wrapper maps treestate's typed
// failure onto ship's error vocabulary without changing behavior.
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

// underlyingErr returns the runner error carried by a treestate.RunError, or the
// error itself otherwise — so the "git_err" field stays the raw runner message
// (boom), matching the pre-extraction behavior byte-for-byte.
func underlyingErr(err error) error {
	var re *treestate.RunError
	if errors.As(err, &re) && re.Err != nil {
		return re.Err
	}
	return err
}
