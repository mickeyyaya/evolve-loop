//go:build acs

package cycle1819

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseobserver"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fixtureCycle = 1819

const usageExit = 10

const oversizedLedgerLineBytes = 17 << 20

const phaseRegistryRel = "docs/architecture/phase-registry.json"

var warnCodeRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

var registeredPhasesSubcommands = []string{
	"list",
	"validate",
	"check-coherence",
	"check-artifact-coherence",
	"add",
	"create",
	"check-provenance",
}

type cliRun struct {
	code   int
	stdout string
	stderr string
}

func (r cliRun) String() string {
	return "exit=" + strconv.Itoa(r.code) + "\n--- stdout ---\n" + r.stdout + "\n--- stderr ---\n" + r.stderr
}

func scrubbedProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	for _, d := range []string{
		filepath.Join(project, "agents"),
		filepath.Join(project, ".evolve", "profiles"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("fixture: mkdir %s: %v", d, err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	for _, k := range []string{
		"EVOLVE_PLUGIN_ROOT", "EVOLVE_PROFILES_DIR_OVERRIDE", "EVOLVE_PROFILE_DIR",
		"EVOLVE_PERSONA_OVERRIDE", "EVOLVE_PROMPTS_DIR", "EVOLVE_LEDGER_OVERRIDE",
	} {
		t.Setenv(k, "")
	}
	return project
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("fixture: mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("fixture: write %s: %v", path, err)
	}
}

func runPhases(args ...string) cliRun {
	var stdout, stderr bytes.Buffer
	code := phasecmd.RunPhases(args, strings.NewReader(""), &stdout, &stderr)
	return cliRun{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runObserver(args ...string) cliRun {
	var stdout, stderr bytes.Buffer
	code := phasecmd.RunPhaseObserver(args, strings.NewReader(""), &stdout, &stderr)
	return cliRun{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func nonDirectoryWorkspace(t *testing.T) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), "observer-ws-is-a-regular-file")
	writeFixture(t, ws, []byte("not a directory"))
	return ws
}

func livePGID() string {
	return strconv.Itoa(syscall.Getpgrp())
}

func codedPolicyWarnings(stderr string) []string {
	var hits []string
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.Contains(line, "WARN") || !strings.Contains(line, "policy") {
			continue
		}
		if !warnCodeRE.MatchString(line) {
			continue
		}
		hits = append(hits, line)
	}
	return hits
}

func observerRunWithPolicy(t *testing.T, policyBody *string, extraFlags ...string) cliRun {
	t.Helper()
	project := scrubbedProject(t)
	if policyBody != nil {
		writeFixture(t, filepath.Join(project, ".evolve", "policy.json"), []byte(*policyBody))
	}
	args := append(append([]string{}, extraFlags...),
		nonDirectoryWorkspace(t), livePGID(), strconv.Itoa(fixtureCycle), "build", "builder")
	return runObserver(args...)
}

func TestC1819_001_ObserverReportsUnparseablePolicyAsCodedWarn(t *testing.T) {
	clean := observerRunWithPolicy(t, nil)
	if hits := codedPolicyWarnings(clean.stderr); len(hits) > 0 {
		t.Fatalf("control: an absent policy.json must not warn, got %q\n%s", hits, clean)
	}
	if clean.code == phaseobserver.ExitOK {
		t.Fatalf("control: a non-directory workspace must end the observer with a non-zero exit\n%s", clean)
	}

	truncatedJSON := "{"
	malformed := observerRunWithPolicy(t, &truncatedJSON)
	if hits := codedPolicyWarnings(malformed.stderr); len(hits) == 0 {
		t.Errorf("RED: a malformed policy.json fell back to zero observer settings silently; want a stderr line with WARN, an UPPER_SNAKE code and the word policy\n%s", malformed)
	}
}

func TestC1819_002_StallResolutionReportsMistypedPolicyAsCodedWarn(t *testing.T) {
	clean := observerRunWithPolicy(t, nil, "--enforce")
	if hits := codedPolicyWarnings(clean.stderr); len(hits) > 0 {
		t.Fatalf("control: an absent policy.json must not warn under --enforce, got %q\n%s", hits, clean)
	}

	observerBlockOfWrongType := `{"observer":"not-an-object"}`
	mistyped := observerRunWithPolicy(t, &observerBlockOfWrongType, "--enforce")
	if hits := codedPolicyWarnings(mistyped.stderr); len(hits) == 0 {
		t.Errorf("RED: --enforce stall resolution over a policy.json that parses as JSON but fails to decode fell back silently; want a coded WARN naming policy\n%s", mistyped)
	}
}

func TestC1819_003_PersonaOverrideWithoutSeparatorIsUsageError(t *testing.T) {
	scrubbedProject(t)
	for _, malformed := range []string{"agents/evolve-fixture.md", "fixture"} {
		got := runPhases("--persona-override", malformed, "check-coherence")
		if got.code != usageExit {
			t.Errorf("RED: --persona-override %q (no ':') exit = %d, want usage error %d\n%s", malformed, got.code, usageExit, got)
		}
		if !strings.Contains(got.stderr, "persona-override") {
			t.Errorf("RED: --persona-override %q rejection must name the flag on stderr\n%s", malformed, got)
		}
	}
}

func TestC1819_004_PersonaOverrideWellFormedStillAccepted(t *testing.T) {
	project := scrubbedProject(t)
	overridePersona := filepath.Join(project, "override", "evolve-fixture.md")
	writeFixture(t, overridePersona, []byte("---\nname: evolve-fixture\ndescription: fixture\n---\n\n# fixture\n"))
	got := runPhases("--persona-override", overridePersona+":fixture", "check-coherence")
	if got.code != 0 {
		t.Errorf("a well-formed --persona-override <path>:<name> must keep working (exit 0), got\n%s", got)
	}
}

func TestC1819_005_UsageAndUnknownTextListEveryRegisteredSubcommand(t *testing.T) {
	scrubbedProject(t)
	usage := runPhases()
	if usage.code != usageExit {
		t.Fatalf("`evolve phases` with no subcommand exit = %d, want %d\n%s", usage.code, usageExit, usage)
	}
	unknown := runPhases("no-such-phases-subcommand")
	if unknown.code != usageExit || !strings.Contains(unknown.stderr, "unknown subcommand") {
		t.Fatalf("a bogus subcommand must be a usage error naming 'unknown subcommand', got\n%s", unknown)
	}
	for _, name := range registeredPhasesSubcommands {
		probe := runPhases(name)
		if strings.Contains(probe.stderr, "unknown subcommand") {
			t.Errorf("RED: %q is not dispatched by `evolve phases` (keep every subcommand name registered)\n%s", name, probe)
			continue
		}
		word := regexp.MustCompile(`(^|[^a-z-])` + regexp.QuoteMeta(name) + `([^a-z-]|$)`)
		if !word.MatchString(usage.stderr + usage.stdout) {
			t.Errorf("RED: usage text omits registered subcommand %q\n%s", name, usage)
		}
		if !word.MatchString(unknown.stderr) {
			t.Errorf("RED: 'unknown subcommand' text omits registered subcommand %q\n%s", name, unknown)
		}
	}
}

func TestC1819_006_ObserverRejectsZeroProcessGroupAtFlagBoundary(t *testing.T) {
	scrubbedProject(t)
	ws := nonDirectoryWorkspace(t)
	accepted := runObserver(ws, livePGID(), strconv.Itoa(fixtureCycle), "build", "builder")
	if strings.Contains(strings.ToLower(accepted.stderr), "pgid") {
		t.Fatalf("control: a live process-group id must not be rejected\n%s", accepted)
	}
	for _, bogus := range []string{"0", "not-a-number"} {
		got := runObserver(ws, bogus, strconv.Itoa(fixtureCycle), "build", "builder")
		if got.code != phaseobserver.ExitInvalidArgs {
			t.Errorf("RED: pgid %q exit = %d, want ExitInvalidArgs %d\n%s", bogus, got.code, phaseobserver.ExitInvalidArgs, got)
		}
		if !strings.Contains(strings.ToLower(got.stderr), "pgid") {
			t.Errorf("RED: pgid %q was not rejected at the flag boundary (stderr must name pgid)\n%s", bogus, got)
		}
	}
}

func provenanceHeader(phase string, cycle int, treeSHA string) string {
	return "<!-- evolve:provenance phase=" + phase + " cycle=" + strconv.Itoa(cycle) +
		" tree_sha=" + treeSHA + " inputs_digest=fixture-digest -->\n# Build Report\n"
}

func ledgerLine(cycle int, role, treeSHA string) string {
	return `{"cycle":` + strconv.Itoa(cycle) + `,"role":"` + role + `","tree_state_sha":"` + treeSHA + `"}` + "\n"
}

func provenanceProject(t *testing.T, buildReport, ledger string) (project, ledgerPath string) {
	t.Helper()
	project = scrubbedProject(t)
	registry, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(phaseRegistryRel)))
	if err != nil {
		t.Fatalf("fixture: read the repo phase registry: %v", err)
	}
	writeFixture(t, filepath.Join(project, filepath.FromSlash(phaseRegistryRel)), registry)
	writeFixture(t, filepath.Join(project, ".evolve", "runs", "cycle-"+strconv.Itoa(fixtureCycle), "build-report.md"), []byte(buildReport))
	ledgerPath = filepath.Join(project, ".evolve", "ledger.jsonl")
	writeFixture(t, ledgerPath, []byte(ledger))
	return project, ledgerPath
}

func checkProvenance(extra ...string) cliRun {
	return runPhases(append([]string{"check-provenance", "--cycle", strconv.Itoa(fixtureCycle)}, extra...)...)
}

func TestC1819_010_CheckProvenanceMismatchedArtifactExitsOne(t *testing.T) {
	provenanceProject(t,
		provenanceHeader("build", fixtureCycle+7, "recorded-sha"),
		ledgerLine(fixtureCycle, "builder", "recorded-sha"))
	got := checkProvenance()
	if got.code != 1 {
		t.Errorf("RED: an artifact whose header names another cycle must exit 1, got\n%s", got)
	}
	if !strings.Contains(got.stdout+got.stderr, "cycle mismatch") {
		t.Errorf("RED: the violation must be printed (want 'cycle mismatch')\n%s", got)
	}
}

func TestC1819_011_CheckProvenanceLedgerTreeMismatchExitsOne(t *testing.T) {
	provenanceProject(t,
		provenanceHeader("build", fixtureCycle, "forged-sha"),
		ledgerLine(fixtureCycle, "builder", "recorded-sha"))
	got := checkProvenance()
	if got.code != 1 {
		t.Errorf("RED: a tree_sha that disagrees with the ledger must exit 1, got\n%s", got)
	}
	if out := got.stdout + got.stderr; !strings.Contains(out, "ledger") || !strings.Contains(out, "recorded-sha") {
		t.Errorf("RED: the ledger cross-check violation must be printed with the ledger's tree sha\n%s", got)
	}
}

func TestC1819_012_CheckProvenanceCleanCycleExitsZero(t *testing.T) {
	provenanceProject(t,
		provenanceHeader("build", fixtureCycle, "recorded-sha"),
		ledgerLine(fixtureCycle, "builder", "recorded-sha"))
	got := checkProvenance()
	if got.code != 0 {
		t.Errorf("RED: a cycle whose artifact agrees with its header and the ledger must exit 0, got\n%s", got)
	}
	if strings.Contains(got.stdout+got.stderr, "mismatch") {
		t.Errorf("a clean cycle must print no mismatch\n%s", got)
	}
}

func TestC1819_013_CheckProvenanceJSONReportsViolation(t *testing.T) {
	provenanceProject(t,
		provenanceHeader("build", fixtureCycle+7, "recorded-sha"),
		ledgerLine(fixtureCycle, "builder", "recorded-sha"))
	got := checkProvenance("--json")
	if got.code != 1 {
		t.Errorf("RED: --json over a mismatched artifact must still exit 1, got\n%s", got)
	}
	if !json.Valid([]byte(strings.TrimSpace(got.stdout))) {
		t.Errorf("RED: --json stdout must be one valid JSON document\n%s", got)
	}
	if !strings.Contains(got.stdout, "cycle mismatch") {
		t.Errorf("RED: --json stdout must carry the violation message\n%s", got)
	}
}

func TestC1819_014_CheckProvenanceCycleFlagIsRequiredAndPositive(t *testing.T) {
	scrubbedProject(t)
	for _, args := range [][]string{
		{"check-provenance"},
		{"check-provenance", "--cycle", "0"},
		{"check-provenance", "--cycle", "-3"},
		{"check-provenance", "--cycle", "not-a-cycle"},
	} {
		got := runPhases(args...)
		if got.code != usageExit {
			t.Errorf("RED: %q exit = %d, want usage error %d\n%s", args, got.code, usageExit, got)
		}
		if strings.Contains(got.stderr, "unknown subcommand") || !strings.Contains(got.stderr, "cycle") {
			t.Errorf("RED: %q must be refused by check-provenance's own --cycle validation, not as an unknown subcommand\n%s", args, got)
		}
	}
}

func TestC1819_015_CheckProvenanceUnreadableLedgerExitsTwo(t *testing.T) {
	cleanArtifact := provenanceHeader("build", fixtureCycle, "recorded-sha")

	t.Run("ledger path is a directory", func(t *testing.T) {
		_, ledgerPath := provenanceProject(t, cleanArtifact, "")
		if err := os.Remove(ledgerPath); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(ledgerPath, 0o755); err != nil {
			t.Fatal(err)
		}
		got := checkProvenance()
		if got.code != 2 {
			t.Errorf("RED: a ledger that cannot be read must exit 2, never pass silently, got\n%s", got)
		}
		if !strings.Contains(strings.ToLower(got.stdout+got.stderr), "ledger") {
			t.Errorf("RED: the exit-2 diagnostic must name the ledger\n%s", got)
		}
	})

	t.Run("ledger file without read permission", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a mode-000 file; the directory case covers this criterion")
		}
		_, ledgerPath := provenanceProject(t, cleanArtifact, ledgerLine(fixtureCycle, "builder", "recorded-sha"))
		if err := os.Chmod(ledgerPath, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ledgerPath, 0o644) })
		got := checkProvenance()
		if got.code != 2 {
			t.Errorf("RED: an unreadable ledger must exit 2, never pass silently, got\n%s", got)
		}
	})
}

func TestC1819_016_CheckProvenanceOversizedLedgerLineExitsTwo(t *testing.T) {
	oversized := `{"cycle":1,"role":"scout","note":"` + strings.Repeat("x", oversizedLedgerLineBytes) + `"}` + "\n"
	ledger := ledgerLine(fixtureCycle, "builder", "recorded-sha") + oversized + ledgerLine(fixtureCycle, "builder", "later-sha")
	provenanceProject(t, provenanceHeader("build", fixtureCycle, "recorded-sha"), ledger)
	got := checkProvenance()
	if got.code != 2 {
		t.Errorf("RED: a ledger line past the scanner's limit must exit 2 (the scan stopped early), got exit=%d stderr=%q", got.code, got.stderr)
	}
	if !strings.Contains(strings.ToLower(got.stdout+got.stderr), "ledger") {
		t.Errorf("RED: the exit-2 diagnostic must name the ledger, got stderr=%q", got.stderr)
	}
}

// acs-predicate: config-check
func TestC1819_017_RuntimeReferenceDocumentsCheckProvenance(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, filepath.Join(root, rel), "phases check-provenance") {
		t.Errorf("RED: %s must document `evolve phases check-provenance` beside the coherence checks", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is not tracked", rel)
	}
}
