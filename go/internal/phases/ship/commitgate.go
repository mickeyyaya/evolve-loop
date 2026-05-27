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
)

// commitGateAttestation mirrors the subset of .commit-gate/attestation.json
// this check reads. The full file also records ts/checks_passed/reviewers_run.
type commitGateAttestation struct {
	TreeStateSHA string `json:"tree_state_sha"`
}

// verifyCommitGateAttestation requires a fresh review attestation whose
// tree_state_sha matches sha256(git diff HEAD). MUST be called AFTER
// verifyManualConfirm's `git add -A`, so the computed SHA reflects exactly the
// tree that will be committed.
func verifyCommitGateAttestation(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.envBool("EVOLVE_BYPASS_COMMIT_GATE") {
		res.Logs = append(res.Logs, "[ship] commit-gate: EVOLVE_BYPASS_COMMIT_GATE=1 — review attestation skipped")
		return nil
	}

	attPath := filepath.Join(opts.ProjectRoot, ".commit-gate", "attestation.json")
	raw, err := os.ReadFile(attPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &IntegrityError{
				Msg: "--class manual requires a commit-gate review attestation, but .commit-gate/attestation.json is missing. " +
					"Run /commit (code-simplifier + code-reviewer + language reviewer + lint + targeted tests) to produce one, " +
					"or set EVOLVE_BYPASS_COMMIT_GATE=1 to bypass.",
			}
		}
		return fmt.Errorf("ship: read commit-gate attestation: %w", err)
	}

	var att commitGateAttestation
	if err := json.Unmarshal(raw, &att); err != nil {
		return &IntegrityError{Msg: fmt.Sprintf("commit-gate attestation is malformed JSON (%v) — re-run /commit", err)}
	}
	if att.TreeStateSHA == "" {
		return &IntegrityError{Msg: "commit-gate attestation has no tree_state_sha — re-run /commit"}
	}

	cur, err := computeTreeStateSHA(ctx, opts)
	if err != nil {
		return fmt.Errorf("ship: commit-gate tree SHA: %w", err)
	}
	if att.TreeStateSHA != cur {
		return &IntegrityError{
			Msg: fmt.Sprintf(
				"commit-gate attestation is stale: reviewed tree=%s but staged tree=%s. "+
					"The change set differs from what was reviewed — re-run /commit.",
				att.TreeStateSHA, cur,
			),
		}
	}
	res.Logs = append(res.Logs, "[ship] commit-gate: review attestation verified (tree "+cur+")")
	if res.Provenance != "" {
		res.Provenance += " + commit-gate attested"
	}
	return nil
}
