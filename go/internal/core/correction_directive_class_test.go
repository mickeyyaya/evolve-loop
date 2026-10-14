package core

import (
	"os"
	"strings"
	"testing"
)

const gateARejection = "scout did not materialize evals for selected slug(s): " +
	"judgment-phase-shadow-config, judgment-verdict-shadow-classifier"

const gateARemediation = "Create each missing eval file at " +
	"<workspace>/.evolve/evals/<slug>.md (also accepted: <projectRoot>/.evolve/evals/<slug>.md), " +
	"each containing at least one [code] grader."

func forbidsFileCreation(directive string) bool {
	return strings.Contains(directive, "Do not change unrelated files")
}

func TestComposeCorrection_SidecarClassDoesNotForbidTheFix(t *testing.T) {
	t.Parallel()

	got := composeCorrection(1, gateARejection, gateARemediation)

	if forbidsFileCreation(got) {
		t.Errorf("the correction for a MISSING-SIDECAR rejection still says 'Do not change unrelated files' — "+
			"that forbids creating the eval files, which is the only action that satisfies the gate "+
			"(0-for-4 recovery in production)\n  got: %s", got)
	}
	if !strings.Contains(got, ".evolve/evals/<slug>.md") {
		t.Errorf("the correction does not name WHERE the missing file goes — the agent is told a slug "+
			"stem with no directory\n  got: %s", got)
	}
	if !strings.Contains(got, gateARejection) {
		t.Errorf("the correction dropped the gate's own reason\n  got: %s", got)
	}
}

func TestComposeCorrection_MalformedClassIsByteIdentical(t *testing.T) {
	t.Parallel()

	const reason = "audit-report.md missing required section(s): ## Verdict"
	want := "Your previous output for this phase was REJECTED by the deliverable contract check:\n\n" +
		reason +
		"\n\nFix the deliverable so it satisfies the contract — write it at the EXACT contracted path " +
		"with all required sections / valid structure — then finish. Do not change unrelated files." +
		"\n\nBefore you finish, append a `## Correction 1` section to the deliverable that names what you fixed and where — " +
		"also when the fix is in another file — so the review that follows, and every later reader, can see what each correction changed. " +
		"A deliverable that is not Markdown is written again in full instead."

	if got := composeCorrection(1, reason, ""); got != want {
		t.Errorf("a malformed-artifact rejection (no remediation) must produce today's directive byte-for-byte\n  got:  %q\n  want: %q", got, want)
	}
}

func TestComposeContractSalvageRetry_CarriesRemediation(t *testing.T) {
	t.Parallel()

	got := composeContractSalvageRetry(2, gateARejection, gateARemediation)

	if forbidsFileCreation(got) {
		t.Errorf("the SECOND-STRIKE directive still forbids changing other files — the escalation path "+
			"reproduces the defect for exactly the cycles that reached it\n  got: %s", got)
	}
	if !strings.Contains(got, ".evolve/evals/<slug>.md") {
		t.Errorf("the second-strike directive does not carry the remediation\n  got: %s", got)
	}
}

func TestComposeContractSalvageRetry_MalformedClassKeepsTodaysText(t *testing.T) {
	t.Parallel()

	const reason = "[MISSING_SECTION] build-report.md lacks ## Task:"
	got := composeContractSalvageRetry(2, reason, "")
	if !forbidsFileCreation(got) {
		t.Errorf("a malformed-artifact salvage retry must KEEP 'Do not change unrelated files' — "+
			"removing it unconditionally would invite collateral edits on the class the clause exists for\n  got: %s", got)
	}
	if !strings.Contains(got, contractSalvageRetryDirectiveHeading) {
		t.Errorf("salvage retry lost its distinct heading\n  got: %s", got)
	}
}

func TestCorrectionCallSites_PassTheRemediation(t *testing.T) {
	t.Parallel()

	var source strings.Builder
	for _, name := range []string{"cyclerun_review.go", "cyclerun_correction.go", "resume.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source.Write(src)
	}
	body := source.String()
	for _, want := range []string{
		"composeCorrection(corr, rr.Reason, rr.Remediation)",
		"composeContractSalvageRetry(corr, rr.Reason, rr.Remediation)",
		"composeCorrection(correction, review.Reason, review.Remediation)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the cycle review components do not call %s — the gate's remediation is computed and then "+
				"dropped, so the directive reverts to the generic text that forbids the fix", want)
		}
	}
}

func TestComposeContractSalvageRetry_EmptyRemediationIsByteIdentical(t *testing.T) {
	t.Parallel()

	const reason = "[MISSING_SECTION] build-report.md lacks ## Task:"
	want := composeCorrection(2, reason, "") + "\n\n" + contractSalvageRetryDirectiveHeading + "\n\n" +
		"This is the second consecutive block reporting the SAME defect, and no other CLI family is " +
		"available to escalate to — this is the last correction before the contract gate's circuit " +
		"breaker opens and the gate stops enforcing for the rest of this run.\n\n" +
		"The contract validator's output, verbatim:\n\n" + reason + "\n\n" +
		"Do not re-summarize it. Take each bracketed [violation_code] above in turn, state the exact " +
		"section heading or file path that code refers to, then re-emit the whole deliverable at the " +
		"contracted path with that specific defect closed. Do not change unrelated files."

	if got := composeContractSalvageRetry(2, reason, ""); got != want {
		t.Errorf("the salvage rung's no-remediation text drifted\n  got:  %q\n  want: %q", got, want)
	}
}
