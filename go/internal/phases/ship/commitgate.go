// commitgate.go — commit-gate review-attestation enforcement for --class manual.
//
// Interactive commits go through `evolve ship --class manual` (bare `git commit`
// is blocked by ship-gate), and ship performs that commit as an internal
// subprocess — so a PreToolUse hook can't observe it. This is the single
// enforcement point for the review attestation: the manual class verifies it
// HERE, at the real chokepoint, reusing the exact sha256(git diff HEAD) that
// the standalone bash runner (commit-gate/commit-gate-runner.sh) wrote.
//
// Class scope: ONLY --class manual. --class cycle keeps audit-binding
// (autonomous cycles are exempt by construction); --class release/trivial are
// driven by their own pipelines and are not interactive commits.
//
// Bypass: EVOLVE_BYPASS_COMMIT_GATE=1 (routine use is a CLAUDE.md violation).
package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// commitGateAttestation mirrors the subset of .commit-gate/attestation.json
// this check reads. The full file also records ts/checks_passed.
type commitGateAttestation struct {
	TreeStateSHA string   `json:"tree_state_sha"`
	ReviewersRun []string `json:"reviewers_run"`
}

// reviewedByTrailer returns a "Reviewed-by:" git trailer block derived from the
// commit-gate attestation's reviewers_run — one standard `Reviewed-by: <name>`
// line per reviewer, as a trailing paragraph git parses as trailers. This makes
// "was this commit reviewed before commit, by whom" a durable, machine-parseable
// property of the SHA (`git log --format='%(trailers:key=Reviewed-by)'`).
//
// Returns "" (no trailer == not reviewed) unless ALL hold: class is manual (the
// only class that carries + verifies a review attestation), the commit gate was
// NOT bypassed (EVOLVE_BYPASS_COMMIT_GATE — a bypass means review was skipped, so
// a stale on-disk attestation must NOT falsely assert review), and the
// attestation parses with ≥1 valid reviewer. Best-effort: a read/parse error
// omits the trailer. Reviewers with embedded newlines are dropped so a corrupt
// attestation can't inject spurious lines into the trailer block.
func reviewedByTrailer(opts *Options) string {
	if opts.Class != ClassManual || opts.envBool("EVOLVE_BYPASS_COMMIT_GATE") {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(opts.ProjectRoot, ".commit-gate", "attestation.json"))
	if err != nil {
		return ""
	}
	var att commitGateAttestation
	if json.Unmarshal(raw, &att) != nil {
		return ""
	}
	var b strings.Builder
	for _, r := range att.ReviewersRun {
		if r = strings.TrimSpace(r); r == "" || strings.ContainsAny(r, "\n\r") {
			continue
		}
		fmt.Fprintf(&b, "\nReviewed-by: %s", r)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n" + b.String() // leading blank line separates the trailer block from the body
}

// verifyCommitGateAttestation requires a fresh review attestation whose
// tree_state_sha matches sha256(git diff HEAD). MUST be called AFTER
// verifyManualConfirm's `git add -A`, so the computed SHA reflects exactly the
// tree that will be committed.
func verifyCommitGateAttestation(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.DryRun {
		// Dry-run simulates and commits nothing, so the review attestation is
		// not required (matches the dry-run journal-only contract).
		res.Logs = append(res.Logs, "[ship] commit-gate: dry-run — review attestation not required (no commit)")
		return nil
	}
	if opts.envBool("EVOLVE_BYPASS_COMMIT_GATE") {
		res.Logs = append(res.Logs, "[ship] commit-gate: EVOLVE_BYPASS_COMMIT_GATE=1 — review attestation skipped")
		return nil
	}

	attPath := filepath.Join(opts.ProjectRoot, ".commit-gate", "attestation.json")
	raw, err := os.ReadFile(attPath)
	if err != nil {
		if os.IsNotExist(err) {
			return shipErr(core.CodeCommitGateMissing, core.ShipClassConfig, core.StageVerifyClass,
				"--class manual requires a commit-gate review attestation, but .commit-gate/attestation.json is missing. "+
					"Run /commit (code-simplifier + code-reviewer + language reviewer + lint + targeted tests) to produce one, "+
					"or set EVOLVE_BYPASS_COMMIT_GATE=1 to bypass.",
				"attestation_path", attPath)
		}
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read commit-gate attestation: "+err.Error(), "attestation_path", attPath)
	}

	var att commitGateAttestation
	if err := json.Unmarshal(raw, &att); err != nil {
		return shipErr(core.CodeCommitGateMalformed, core.ShipClassConfig, core.StageVerifyClass,
			fmt.Sprintf("commit-gate attestation is malformed JSON (%v) — re-run /commit", err),
			"attestation_path", attPath, "json_err", err.Error())
	}
	if att.TreeStateSHA == "" {
		return shipErr(core.CodeCommitGateMalformed, core.ShipClassConfig, core.StageVerifyClass,
			"commit-gate attestation has no tree_state_sha — re-run /commit", "attestation_path", attPath)
	}

	cur, err := computeTreeStateSHA(ctx, opts)
	if err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: commit-gate tree SHA: "+err.Error())
	}
	if att.TreeStateSHA != cur {
		return shipErr(core.CodeCommitGateStale, core.ShipClassConfig, core.StageVerifyClass,
			fmt.Sprintf(
				"commit-gate attestation is stale: reviewed tree=%s but staged tree=%s. "+
					"The change set differs from what was reviewed — re-run /commit.",
				att.TreeStateSHA, cur,
			),
			"reviewed_tree", att.TreeStateSHA, "staged_tree", cur)
	}
	res.Logs = append(res.Logs, "[ship] commit-gate: review attestation verified (tree "+cur+")")
	if res.Provenance != "" {
		res.Provenance += " + commit-gate attested"
	}
	return nil
}
