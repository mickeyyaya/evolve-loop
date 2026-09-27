package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecoherence"
)

// commitGateAttestation mirrors the subset of .commit-gate/attestation.json
// this check reads. The full file also records ts/checks_passed.
type commitGateAttestation struct {
	TreeStateSHA string   `json:"tree_state_sha"`
	ReviewersRun []string `json:"reviewers_run"`
}

// reviewedByTrailer returns a "Reviewed-by:" trailer block from the
// attestation's reviewers_run, or "" when the commit was not reviewed.
// Embedded-newline reviewers are dropped so a corrupt attestation cannot
// inject spurious trailer lines.
func reviewedByTrailer(opts *Options) string {
	if opts.Class != ClassManual || opts.BypassCommitGate {
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
	return "\n" + b.String()
}

// verifyCommitGateAttestation must run after verifyManualConfirm's `git add
// -A`, so the computed SHA reflects the tree that will be committed.
func verifyCommitGateAttestation(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.DryRun {
		res.Logs = append(res.Logs, "[ship] commit-gate: dry-run — review attestation not required (no commit)")
		return nil
	}
	if opts.BypassCommitGate {
		res.Logs = append(res.Logs, "[ship] commit-gate: --bypass-commit-gate — review attestation skipped")
		return nil
	}

	attPath := filepath.Join(opts.ProjectRoot, ".commit-gate", "attestation.json")
	raw, err := os.ReadFile(attPath)
	if err != nil {
		if os.IsNotExist(err) {
			return shipErr(core.CodeCommitGateMissing, core.ShipClassConfig, core.StageVerifyClass,
				"--class manual requires a commit-gate review attestation, but .commit-gate/attestation.json is missing. "+
					"Run /commit (code-simplifier + code-reviewer + language reviewer + lint + targeted tests) to produce one, "+
					"or pass --bypass-commit-gate to bypass.",
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

func runPersonaLint(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.BypassCommitGate {
		res.Logs = append(res.Logs, "[ship] persona-lint: --bypass-commit-gate — persona lint skipped")
		return nil
	}

	profileDir := opts.envStr("EVOLVE_PROFILE_DIR")
	if profileDir == "" {
		profileDir = filepath.Join(opts.ProjectRoot, ".evolve", "profiles")
	}

	agentsDir := filepath.Join(opts.ProjectRoot, "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		res.Logs = append(res.Logs, "[ship] persona-lint: agents/ directory missing, skipping lint")
		return nil
	}
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		res.Logs = append(res.Logs, "[ship] persona-lint: profiles directory missing, skipping lint")
		return nil
	}

	overrides := make(map[string]string)
	if override := opts.envStr("EVOLVE_PERSONA_OVERRIDE"); override != "" {
		parts := strings.SplitN(override, ":", 2)
		if len(parts) == 2 {
			path := parts[0]
			name := parts[1]
			overrides[name] = path
		}
	}

	pcOpts := phasecoherence.Options{
		AgentsFS:   os.DirFS(opts.ProjectRoot),
		ProfilesFS: os.DirFS(profileDir),
		Overrides:  overrides,
	}

	violations1, err := phasecoherence.Check(pcOpts)
	if err != nil {
		return err
	}
	violations2, err := phasecoherence.CheckArtifactNames(pcOpts)
	if err != nil {
		return err
	}

	allViolations := append(violations1, violations2...)

	var errMsgs []string
	for _, v := range allViolations {
		msg := fmt.Sprintf("[ship] persona-lint: %s: %s: %s", v.Severity, v.Persona, v.Message)
		res.Logs = append(res.Logs, msg)

		if v.Kind == "disallowed" {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", v.Persona, v.Message))
		}
	}

	if len(errMsgs) > 0 {
		return shipErr(core.ShipErrorCode("PERSONA_COHERENCE_MISMATCH"), core.ShipClassIntegrity, core.StageVerifyClass,
			"persona coherence check failed: "+strings.Join(errMsgs, "; "))
	}

	res.Logs = append(res.Logs, "[ship] persona-lint: check completed with no blocking errors")
	return nil
}
