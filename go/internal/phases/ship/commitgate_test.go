// commitgate_test.go — verifyCommitGateAttestation (commitgate.go).
//
// The --class manual path is the interactive-commit chokepoint. These tests
// assert the review-attestation hard gate: missing/stale → refuse, valid →
// ship, EVOLVE_BYPASS_COMMIT_GATE=1 → skip. Cycle/release classes are NOT
// affected (covered by the native parity matrix).

package ship

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// excludeCommitGate adds .commit-gate/ to the repo's local git excludes so
// ship's `git add -A` never stages the attestation (which would otherwise
// mutate the tree and invalidate its own SHA). Mirrors the real repo's
// .gitignore entry.
func excludeCommitGate(t *testing.T, repo string) {
	t.Helper()
	p := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.WriteFile(p, []byte(".commit-gate/\n"), 0o644); err != nil {
		t.Fatalf("write exclude: %v", err)
	}
}

func writeAttestation(t *testing.T, repo, treeSHA string) {
	t.Helper()
	mustMkdir(t, filepath.Join(repo, ".commit-gate"))
	body := fmt.Sprintf(`{"tree_state_sha":%q,"ts":"2026-05-27T00:00:00Z","checks_passed":["go:gofmt","go:test"],"reviewers_run":["code-simplifier","code-reviewer","go-reviewer"],"tool":"shasum"}`+"\n", treeSHA)
	mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"), body)
}

// Missing attestation → manual ship refuses (ExitIntegrity).
func TestCommitGate_ManualMissingAttestation_Refuses(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nchange w/o review\n")

	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "unreviewed change",
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if res.ExitCode != ExitFailure {
		t.Fatalf("want ExitFailure (missing commit-gate attestation is a config error), got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "requires a commit-gate review attestation") {
		t.Errorf("missing attestation-required message in: %v", res.Logs)
	}
}

// Valid attestation matching the staged tree → ships.
func TestCommitGate_ManualValidAttestation_Ships(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nreviewed change\n")

	// git diff HEAD is identical whether the change is staged or not, and
	// .commit-gate/ is excluded, so this SHA matches what ship computes after
	// its own `git add -A`.
	writeAttestation(t, repo, treeStateSHA(t, repo))

	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "reviewed change",
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "review attestation verified") {
		t.Errorf("missing 'review attestation verified' in: %v", res.Logs)
	}
}

// Attestation present but bound to a different tree → stale → refuse.
func TestCommitGate_ManualStaleAttestation_Refuses(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nactual change\n")
	// Attestation for some OTHER tree state.
	writeAttestation(t, repo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "actual change",
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if res.ExitCode != ExitFailure {
		t.Fatalf("want ExitFailure (stale commit-gate attestation is a config error), got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "stale") {
		t.Errorf("missing 'stale' message in: %v", res.Logs)
	}
}

// Dry-run requires no attestation (it commits nothing).
func TestCommitGate_ManualDryRun_SkipsAttestation(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ndry change\n")
	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "dry change",
		DryRun:        true,
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
}

// EVOLVE_BYPASS_COMMIT_GATE=1 → ships without an attestation.
func TestCommitGate_ManualBypass_Ships(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nbypassed change\n")

	res, _ := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "bypassed change",
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1", "EVOLVE_BYPASS_COMMIT_GATE": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "EVOLVE_BYPASS_COMMIT_GATE=1") {
		t.Errorf("missing bypass log in: %v", res.Logs)
	}
}
