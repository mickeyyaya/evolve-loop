//go:build integration

// commitgate_test.go — verifyCommitGateAttestation (commitgate.go).
//
// The --class manual path is the interactive-commit chokepoint. These tests
// assert the review-attestation hard gate: missing/stale → refuse, valid →
// ship, EVOLVE_BYPASS_COMMIT_GATE=1 → skip. Cycle/release classes are NOT
// affected (covered by the native parity matrix).

package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// --- persona lint (cycle-241, migration step 5: commitgate-persona-lint) ---
//
// runPersonaLint wires phasecoherence.Check + CheckArtifactNames into the
// ship gate for BOTH --class manual and --class cycle, so persona↔profile
// drift cannot silently enter the commit chain. Contract pinned here:
//
//   - layout: agents/ under opts.ProjectRoot; profiles under
//     <ProjectRoot>/.evolve/profiles (the phasesCheckCoherence default).
//   - Kind "disallowed" (persona declares a tool its profile forbids — a
//     contradiction) BLOCKS with *IntegrityError. A persona lying about its
//     capabilities is an integrity breach, never acceptable drift.
//   - Kind "undeclared"/artifact-name "mismatch" (profile allows more than
//     the persona declares) LOGS loudly but does NOT block: the real repo
//     carries ~40 such WARNs today (`evolve phases check-coherence`);
//     blocking on them would brick every ship including this cycle's own.
//   - missing agents/ or profiles dir → skip with a log (repos without
//     personas — including every other ship test fixture — are unaffected).
//   - EVOLVE_BYPASS_COMMIT_GATE=1 → lint skipped (consistent with the
//     attestation bypass; routine use is a CLAUDE.md violation).
//
// NOTE for Builder: the real repo has 7 "disallowed" contradictions across
// doc-sync/intent/scout personas. Those persona/profile pairs MUST be
// reconciled in this cycle or the new gate blocks our own --class cycle ship.

// writePersonaFixture writes agents/evolve-<name>.md (tools frontmatter) and
// .evolve/profiles/<name>.json (allowed_tools) under root, mirroring the real
// repo layout that phasecoherence.Options consumes.
func writePersonaFixture(t *testing.T, root, name string, personaTools, allowedTools []string) {
	t.Helper()
	quoted := make([]string, len(personaTools))
	for i, p := range personaTools {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	persona := "---\nname: evolve-" + name + "\ntools: [" + strings.Join(quoted, ", ") + "]\n---\n\n# " + name + " persona\n"
	mustWrite(t, filepath.Join(root, "agents", "evolve-"+name+".md"), persona)

	prof, err := json.Marshal(map[string]any{
		"name":          name,
		"role":          name,
		"allowed_tools": allowedTools,
	})
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	mustWrite(t, filepath.Join(root, ".evolve", "profiles", name+".json"), string(prof)+"\n")
}

// lintOpts builds the minimal Options for unit-calling runPersonaLint.
// EVOLVE_BYPASS_COMMIT_GATE is explicitly pinned off so an ambient env var
// can't silently turn these tests into no-ops (envBool falls back to
// os.Getenv when the key is absent from Env).
func lintOpts(root string) *Options {
	return &Options{
		Class:       ClassCycle,
		ProjectRoot: root,
		Env:         map[string]string{"EVOLVE_BYPASS_COMMIT_GATE": "0"},
	}
}

// Coherent persona/profile pair → lint passes.
func TestPersonaLint_CleanTreePasses(t *testing.T) {
	root := t.TempDir()
	writePersonaFixture(t, root, "builder", []string{"Read", "Bash"}, []string{"Read", "Bash"})

	res := &RunResult{}
	if err := runPersonaLint(context.Background(), lintOpts(root), res); err != nil {
		t.Fatalf("clean personas: want nil, got %v (logs=%v)", err, res.Logs)
	}
}

// Injected persona→profile contradiction (persona declares a tool the profile
// disallows) → *IntegrityError.
func TestPersonaLint_ViolationBlocks(t *testing.T) {
	root := t.TempDir()
	// Persona claims Bash; profile only allows Read → Kind "disallowed".
	writePersonaFixture(t, root, "builder", []string{"Read", "Bash"}, []string{"Read"})

	res := &RunResult{}
	err := runPersonaLint(context.Background(), lintOpts(root), res)
	if err == nil {
		t.Fatalf("disallowed-tool contradiction must block; logs=%v", res.Logs)
	}
	var ie *IntegrityError
	if !errors.As(err, &ie) {
		t.Fatalf("want *IntegrityError, got %T: %v", err, err)
	}
}

// Undeclared drift (profile allows more than the persona declares) is the
// pre-existing repo-wide WARN class: logged loudly, never blocking.
func TestPersonaLint_UndeclaredDriftLogsButPasses(t *testing.T) {
	root := t.TempDir()
	writePersonaFixture(t, root, "builder", []string{"Read"}, []string{"Read", "WebSearch"})

	res := &RunResult{}
	if err := runPersonaLint(context.Background(), lintOpts(root), res); err != nil {
		t.Fatalf("undeclared drift must NOT block (real repo carries ~40 such WARNs): %v", err)
	}
	if !containsLog(*res, "persona-lint") {
		t.Errorf("undeclared drift must be logged loudly (silent WARN is the retro defect class); logs=%v", res.Logs)
	}
}

// Repo without agents/ or profiles dirs (every other ship test fixture) →
// lint skips instead of erroring.
func TestPersonaLint_MissingDirsSkips(t *testing.T) {
	res := &RunResult{}
	if err := runPersonaLint(context.Background(), lintOpts(t.TempDir()), res); err != nil {
		t.Fatalf("missing agents/profiles dirs must skip the lint, got %v", err)
	}
}

// EVOLVE_BYPASS_COMMIT_GATE=1 skips the lint even over a blocking violation.
func TestPersonaLint_BypassEnvSkipsLint(t *testing.T) {
	root := t.TempDir()
	writePersonaFixture(t, root, "builder", []string{"Read", "Bash"}, []string{"Read"}) // would block

	opts := lintOpts(root)
	opts.Env["EVOLVE_BYPASS_COMMIT_GATE"] = "1"
	res := &RunResult{}
	if err := runPersonaLint(context.Background(), opts, res); err != nil {
		t.Fatalf("bypass env must skip persona lint, got %v", err)
	}
	if !containsLog(*res, "EVOLVE_BYPASS_COMMIT_GATE") {
		t.Errorf("bypass must be logged (loud, not silent); logs=%v", res.Logs)
	}
}

// End-to-end wiring pin: a full --class cycle ship on a repo with coherent
// personas runs the lint (visible in logs) and ships. This is what makes
// runPersonaLint a gate rather than dead code.
func TestCommitGate_CyclePersonaLint(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	writePersonaFixture(t, repo, "builder", []string{"Read"}, []string{"Read"})
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ncycle change\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "evolve-cycle 1: goal=test",
		Env:           map[string]string{"EVOLVE_BYPASS_COMMIT_GATE": "0"},
	})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("clean cycle ship: want ExitOK, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if !containsLog(res, "persona-lint") {
		t.Errorf("--class cycle ship must run the persona lint; logs=%v", res.Logs)
	}
}
