//go:build acs

package cycle1300

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "./internal/core/"

const escalationDoc = "docs/architecture/contract-block-cli-escalation.md"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", corePkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", corePkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func report(t *testing.T, what string, missing []string, out string) {
	t.Helper()
	if len(missing) > 0 {
		t.Errorf("RED: %s — and these contract tests did not report PASS (deleted or skipped): %v\n%s",
			what, missing, tail(out, 40))
		return
	}
	t.Errorf("RED: %s\n%s", what, tail(out, 40))
}

func TestC1300_001_SalvageRetryFiresWhenNoEscalationFamily(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestContractEscalation_SalvageRetry_WhenNoOtherFamily",
		"TestContractEscalation_SalvageRetry_NotWhenEscalationTargetExists")
	if !ok {
		report(t, "a contract-blocked phase whose whole dispatch chain is one CLI family still falls through unremedied — the block-2 correction must become a breaker-neutral structured re-prompt (same CLI, verbatim reason, no extra dispatch), and must stay disjoint from CLI escalation where a target exists", missing, out)
	}
}

func TestC1300_002_SalvageRetryRespectsTriggerScoping(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestContractEscalation_SalvageRetry_NotOnFirstBlock",
		"TestContractEscalation_SalvageRetry_NotOnNonContractRejection")
	if !ok {
		report(t, "the salvage retry fired outside its trigger window — it must start at the same block as escalation would (block 2) and never on a Blocks==0 non-contract rejection", missing, out)
	}
}

func TestC1300_003_DemotionRecordDistinguishesSalvageAttempt(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestContractEscalation_SalvageRetry_LedgerRecordsSalvageAttempted",
		"TestContractEscalation_SalvageRetry_WarnDistinguishesAttemptFromNoRemedy")
	if !ok {
		report(t, "a gate demotion still cannot be told apart from one where no remedy was possible — the ledger Action must record salvage_attempted, and the WARN must not claim 'did NOT run' after a salvage retry was attempted", missing, out)
	}
}

// acs-predicate: config-check — this criterion IS a documentation-content
func TestC1300_004_EscalationDocNamesSalvageOutcome(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), escalationDoc)
	if !acsassert.FileExists(t, doc) {
		return
	}
	for _, want := range []string{"salvage_attempted", "structured re-prompt"} {
		if !acsassert.FileContains(t, doc, want) {
			t.Errorf("RED: %s does not document %q — the doc is Status: live and is the operator's map of this ladder; a remedy that exists only in code is a remedy nobody knows to look for", escalationDoc, want)
		}
	}
}
