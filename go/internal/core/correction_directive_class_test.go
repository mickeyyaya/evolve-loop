package core

import (
	"os"
	"strings"
	"testing"
)

// gateARejection is a real Gate A reject reason, verbatim from a production
// scout-prompt dispatch.
const gateARejection = "scout did not materialize evals for selected slug(s): " +
	"judgment-phase-shadow-config, judgment-verdict-shadow-classifier"

// gateARemediation is what the gate should supply alongside that reason.
const gateARemediation = "Create each missing eval file at " +
	"<workspace>/.evolve/evals/<slug>.md (also accepted: <projectRoot>/.evolve/evals/<slug>.md), " +
	"each containing at least one [code] grader."

// forbidsFileCreation reports whether a directive tells the agent not to touch
// other files — fatal when the remedy REQUIRES creating them.
func forbidsFileCreation(directive string) bool {
	return strings.Contains(directive, "Do not change unrelated files")
}

func TestComposeCorrection_SidecarClassDoesNotForbidTheFix(t *testing.T) {
	got := composeCorrection(gateARejection, gateARemediation)

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

// TestComposeCorrection_MalformedClassIsByteIdentical pins the no-regression
// half: the class the generic template WAS written for must not change at all.
// Without this, "fix the directive" could silently weaken every other gate's
// correction.
func TestComposeCorrection_MalformedClassIsByteIdentical(t *testing.T) {
	const reason = "audit-report.md missing required section(s): ## Verdict"
	want := "Your previous output for this phase was REJECTED by the deliverable contract check:\n\n" +
		reason +
		"\n\nFix the deliverable so it satisfies the contract — write it at the EXACT contracted path " +
		"with all required sections / valid structure — then finish. Do not change unrelated files."

	if got := composeCorrection(reason, ""); got != want {
		t.Errorf("a malformed-artifact rejection (no remediation) must produce today's directive byte-for-byte\n  got:  %q\n  want: %q", got, want)
	}
}

// TestComposeContractSalvageRetry_CarriesRemediation: Gate A failures reach the
// SECOND-STRIKE path, so fixing only composeCorrection leaves the defect live
// on the path that actually decides those cycles.
func TestComposeContractSalvageRetry_CarriesRemediation(t *testing.T) {
	got := composeContractSalvageRetry(gateARejection, gateARemediation)

	if forbidsFileCreation(got) {
		t.Errorf("the SECOND-STRIKE directive still forbids changing other files — the escalation path "+
			"reproduces the defect for exactly the cycles that reached it\n  got: %s", got)
	}
	if !strings.Contains(got, ".evolve/evals/<slug>.md") {
		t.Errorf("the second-strike directive does not carry the remediation\n  got: %s", got)
	}
}

// TestComposeContractSalvageRetry_MalformedClassKeepsTodaysText: the salvage
// rung for the class it was designed for is unchanged.
func TestComposeContractSalvageRetry_MalformedClassKeepsTodaysText(t *testing.T) {
	const reason = "[MISSING_SECTION] build-report.md lacks ## Task:"
	got := composeContractSalvageRetry(reason, "")
	if !forbidsFileCreation(got) {
		t.Errorf("a malformed-artifact salvage retry must KEEP 'Do not change unrelated files' — "+
			"removing it unconditionally would invite collateral edits on the class the clause exists for\n  got: %s", got)
	}
	if !strings.Contains(got, contractSalvageRetryDirectiveHeading) {
		t.Errorf("salvage retry lost its distinct heading\n  got: %s", got)
	}
}

// TestCorrectionCallSites_PassTheRemediation pins the JOIN. The gate half
// (evalgate) proves Remediation reaches ReviewResult; the composer half above
// proves a non-empty remediation changes the directive. Neither covers the one
// line that connects them, and composeCorrection is unexported so no
// cross-package test can reach it.
//
// A call site that still passes only rr.Reason compiles, keeps every other test
// green, and silently restores the 0-for-4 behavior — the gate would compute a
// remediation nobody reads. Both dispatch paths must pass it.
func TestCorrectionCallSites_PassTheRemediation(t *testing.T) {
	var source strings.Builder
	for _, name := range []string{"cyclerun_review.go", "cyclerun_correction.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source.Write(src)
	}
	body := source.String()
	for _, want := range []string{
		"composeCorrection(rr.Reason, rr.Remediation)",
		"composeContractSalvageRetry(rr.Reason, rr.Remediation)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the cycle review components do not call %s — the gate's remediation is computed and then "+
				"dropped, so the directive reverts to the generic text that forbids the fix", want)
		}
	}
}

// TestComposeContractSalvageRetry_EmptyRemediationIsByteIdentical is the golden
// twin of TestComposeCorrection_MalformedClassIsByteIdentical. The salvage rung
// had only substring assertions, so a reworded clause could drift unnoticed on
// the path that fires for repeat contract blocks. Written as a literal on
// purpose: an intentional edit SHOULD fail here and be re-approved.
func TestComposeContractSalvageRetry_EmptyRemediationIsByteIdentical(t *testing.T) {
	const reason = "[MISSING_SECTION] build-report.md lacks ## Task:"
	want := composeCorrection(reason, "") + "\n\n" + contractSalvageRetryDirectiveHeading + "\n\n" +
		"This is the second consecutive block reporting the SAME defect, and no other CLI family is " +
		"available to escalate to — this is the last correction before the contract gate's circuit " +
		"breaker opens and the gate stops enforcing for the rest of this run.\n\n" +
		"The contract validator's output, verbatim:\n\n" + reason + "\n\n" +
		"Do not re-summarize it. Take each bracketed [violation_code] above in turn, state the exact " +
		"section heading or file path that code refers to, then re-emit the whole deliverable at the " +
		"contracted path with that specific defect closed. Do not change unrelated files."

	if got := composeContractSalvageRetry(reason, ""); got != want {
		t.Errorf("the salvage rung's no-remediation text drifted\n  got:  %q\n  want: %q", got, want)
	}
}
