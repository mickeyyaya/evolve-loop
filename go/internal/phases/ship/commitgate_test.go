//go:build integration

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

func TestCommitGate_ManualBypass_Ships(t *testing.T) {
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nbypassed change\n")

	res, _ := runShip(t, repo, Options{
		Class:            ClassManual,
		CommitMessage:    "bypassed change",
		BypassCommitGate: true,
		Env:              map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "--bypass-commit-gate") {
		t.Errorf("missing bypass log in: %v", res.Logs)
	}
}

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
func lintOpts(root string) *Options {
	return &Options{
		Class:       ClassCycle,
		ProjectRoot: root,
	}
}

func TestPersonaLint_CleanTreePasses(t *testing.T) {
	root := t.TempDir()
	writePersonaFixture(t, root, "builder", []string{"Read", "Bash"}, []string{"Read", "Bash"})

	res := &RunResult{}
	if err := runPersonaLint(context.Background(), lintOpts(root), res); err != nil {
		t.Fatalf("clean personas: want nil, got %v (logs=%v)", err, res.Logs)
	}
}

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

func TestPersonaLint_MissingDirsSkips(t *testing.T) {
	res := &RunResult{}
	if err := runPersonaLint(context.Background(), lintOpts(t.TempDir()), res); err != nil {
		t.Fatalf("missing agents/profiles dirs must skip the lint, got %v", err)
	}
}

func TestPersonaLint_BypassSkipsLint(t *testing.T) {
	root := t.TempDir()
	writePersonaFixture(t, root, "builder", []string{"Read", "Bash"}, []string{"Read"}) // would block

	opts := lintOpts(root)
	opts.BypassCommitGate = true
	res := &RunResult{}
	if err := runPersonaLint(context.Background(), opts, res); err != nil {
		t.Fatalf("bypass env must skip persona lint, got %v", err)
	}
	if !containsLog(*res, "--bypass-commit-gate") {
		t.Errorf("bypass must be logged (loud, not silent); logs=%v", res.Logs)
	}
}

func TestCommitGate_CyclePersonaLint(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	writePersonaFixture(t, repo, "builder", []string{"Read"}, []string{"Read"})
	runGit(t, repo, "add", "agents/evolve-builder.md")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add persona fixture")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ncycle change\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "evolve-cycle 1: goal=test",
	})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("clean cycle ship: want ExitOK, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if !containsLog(res, "persona-lint") {
		t.Errorf("--class cycle ship must run the persona lint; logs=%v", res.Logs)
	}
}
