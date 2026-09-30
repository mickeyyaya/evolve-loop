//go:build acs

package cycle1648

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1648_001_AuditCalibrationIsReachableThroughTheRealCLI(t *testing.T) {
	c := fixtureCorpus(t)
	report, stderr, code := calibrate(t, c.Dossiers, c.Runs)
	if code != 0 {
		t.Fatalf("RED: `evolve audit calibration` exit=%d (want 0): %s", code, firstLine(stderr))
	}
	if strings.TrimSpace(report) == "" {
		t.Fatalf("RED: --output file missing or empty after a successful run")
	}
	if !strings.Contains(report, "# ") {
		t.Errorf("RED: report has no Markdown heading; got:\n%s", report)
	}
	stdout, stderr, _ := runEvolve(t, "help")
	usage := stdout + stderr
	if !strings.Contains(usage, "audit") || !strings.Contains(usage, "calibration") {
		t.Errorf("RED: `evolve help` does not document `audit calibration` (main.go usage block is hand-maintained beside registry.go)")
	}
}

func TestC1648_002_MatrixPreservesNarrativeGateShippedAndChainDistinctly(t *testing.T) {
	c := fixtureCorpus(t)
	report, stderr, code := calibrate(t, c.Dossiers, c.Runs)
	if code != 0 {
		t.Fatalf("RED: exit=%d: %s", code, firstLine(stderr))
	}
	if got := summaryCount(report, "Valid pairs"); got != 5 {
		t.Errorf("RED: `Valid pairs: 5` expected, got %d", got)
	}
	requireRow(t, report, "matrix (PASS,PASS)=1", row("PASS", "PASS", "1"))
	requireRow(t, report, "matrix (PASS,FAIL)=2", row("PASS", "FAIL", "2"))
	requireRow(t, report, "matrix (WARN,FAIL)=1", row("WARN", "FAIL", "1"))
	requireRow(t, report, "matrix (FAIL,PASS)=1", row("FAIL", "PASS", "1"))
	if got := matrixTotal(report); got != 5 {
		t.Errorf("RED: matrix cells sum to %d, want 5 (one row per observed (narrative,gate) cell)", got)
	}
	requireRow(t, report, "pair 101 clean PASS", row("101", "PASS", "PASS", "PASS", "PASS", ""))
	requireRow(t, report, "pair 102 narrative PASS, chain PASS, gate FAIL, shipped FAIL, override named", row("102", "PASS", "PASS", "FAIL", "FAIL", gateExplain))
	requireRow(t, report, "pair 103 chain `absent` preserved verbatim", row("103", "WARN", "absent", "FAIL", "FAIL", gateEGPS))
	requireRow(t, report, "pair 105 shipped falls back to the dossier's final_verdict", row("105", "FAIL", "FAIL", "PASS", "FAIL", ""))
	line := findRow(report, row("104", "PASS", "PASS", "FAIL", "FAIL"))
	if line == "" {
		t.Errorf("RED: pair 104 row missing or has wrong verdict columns")
	} else if !strings.Contains(line, gateEGPS) || !strings.Contains(line, gateAPI) {
		t.Errorf("RED: pair 104's row does not carry both overriding gates (%q, %q): %q", gateEGPS, gateAPI, line)
	}
}

func TestC1648_003_ForceOverriddenNarrativePassIsNeverCollapsedIntoAgreement(t *testing.T) {
	c := fixtureCorpus(t)
	report, stderr, code := calibrate(t, c.Dossiers, c.Runs)
	if code != 0 {
		t.Fatalf("RED: exit=%d: %s", code, firstLine(stderr))
	}
	requireRow(t, report, "override row for 102", rowExact("102", "PASS", "FAIL", gateExplain))
	requireRow(t, report, "override row for 103", rowExact("103", "WARN", "FAIL", gateEGPS))
	if line := findRow(report, rowExact("104", "PASS", "FAIL", anyCell)); line == "" || !strings.Contains(line, gateEGPS) || !strings.Contains(line, gateAPI) {
		t.Errorf("RED: override row for 104 must name both gates; got %q", line)
	}
	forbidRow(t, report, "101 was not overridden", rowExact("101", anyCell, anyCell, anyCell))
	forbidRow(t, report, "105 (narrative FAIL, gates green) is not an override", rowExact("105", anyCell, anyCell, anyCell))

	ctl := fixtureCorpus(t)
	ctl.shadow(t, "cycle-102", shadowJSON(102, "PASS", "PASS", "PASS", nil))
	ctl.dossier(t, 102, dossierJSON(102, "PASS"))
	if err := os.Remove(filepath.Join(ctl.Runs, "cycle-102", "audit-fail-reason.json")); err != nil {
		t.Fatal(err)
	}
	control, stderr, code := calibrate(t, ctl.Dossiers, ctl.Runs)
	if code != 0 {
		t.Fatalf("RED (control): exit=%d: %s", code, firstLine(stderr))
	}
	requireRow(t, control, "control matrix (PASS,PASS)=2", row("PASS", "PASS", "2"))
	requireRow(t, control, "control matrix (PASS,FAIL)=1", row("PASS", "FAIL", "1"))
	forbidRow(t, control, "control has no override row for 102", rowExact("102", anyCell, anyCell, anyCell))
	requireRow(t, control, "control pair 102 is a clean pass", row("102", "PASS", "PASS", "PASS", "PASS", ""))
	if report == control {
		t.Errorf("RED: the override corpus and its control rendered byte-identical reports — overrode_by is not read")
	}
}

func TestC1648_004_InvalidCorpusInputsFailLoudly(t *testing.T) {
	c := fixtureCorpus(t)
	missing := filepath.Join(t.TempDir(), "definitely-missing")
	notADir := filepath.Join(t.TempDir(), "file.txt")
	mustWrite(t, notADir, "not a directory\n")
	cases := []struct {
		name   string
		args   []string
		output string
		expect string
	}{
		{"no subcommand", []string{"audit"}, "", "calibration"},
		{"unknown subcommand", []string{"audit", "bogus"}, "", "bogus"},
		{"missing --output", []string{"audit", "calibration", "--dossiers-dir", c.Dossiers, "--runs-dir", c.Runs}, "", "output"},
		{"--runs-dir does not exist", []string{"audit", "calibration", "--dossiers-dir", c.Dossiers, "--runs-dir", missing}, "x", missing},
		{"--dossiers-dir is a file", []string{"audit", "calibration", "--dossiers-dir", notADir, "--runs-dir", c.Runs}, "x", notADir},
		{"--output dir does not exist", []string{"audit", "calibration", "--dossiers-dir", c.Dossiers, "--runs-dir", c.Runs}, filepath.Join(missing, "r.md"), missing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			out := ""
			switch tc.output {
			case "":
			case "x":
				out = filepath.Join(t.TempDir(), "never.md")
				args = append(args, "--output", out)
			default:
				out = tc.output
				args = append(args, "--output", out)
			}
			_, stderr, code := runEvolve(t, args...)
			if code == 0 {
				t.Errorf("RED: %s → exit 0 (want non-zero)", tc.name)
			}
			switch {
			case strings.TrimSpace(stderr) == "":
				t.Errorf("RED: %s → empty stderr (must fail LOUDLY)", tc.name)
			case strings.Contains(stderr, "unknown command"):
				t.Errorf("RED: %s → rejected by the dispatcher, not by `audit calibration`: %s", tc.name, firstLine(stderr))
			case !strings.Contains(stderr, tc.expect):
				t.Errorf("RED: %s → stderr does not name the offender %q: %s", tc.name, tc.expect, firstLine(stderr))
			}
			if out != "" {
				if _, err := os.Stat(out); err == nil {
					t.Errorf("RED: %s → a report was written despite the failure", tc.name)
				}
			}
		})
	}
}

func TestC1648_005_MalformedAndMissingPairsAreCountedAsExclusionsNeverAgreement(t *testing.T) {
	c := fixtureCorpus(t)
	report, stderr, code := calibrate(t, c.Dossiers, c.Runs)
	if code != 0 {
		t.Fatalf("RED: exit=%d: %s", code, firstLine(stderr))
	}
	if got := summaryCount(report, "Excluded"); got != 5 {
		t.Errorf("RED: `Excluded: 5` expected, got %d", got)
	}
	for cycle, reason := range map[string]string{
		"106": "missing-shadow",
		"107": "malformed-shadow",
		"108": "missing-dossier",
		"109": "malformed-dossier",
		"110": "malformed-fail-reason",
	} {
		requireRow(t, report, "exclusion "+cycle, row(cycle, reason))
	}
	if got := matrixTotal(report); got != 5 {
		t.Errorf("RED: matrix total %d ≠ 5 valid pairs — an excluded cycle leaked into the matrix", got)
	}
	for _, cycle := range []string{"106", "107", "108", "109", "110"} {
		for _, v := range []string{"PASS", "WARN", "FAIL"} {
			forbidRow(t, report, "excluded "+cycle+" must not be a pair row", row(cycle, v))
		}
	}
	forbidRow(t, report, "reset run dir is not a pair or exclusion", row("1623"))
	forbidRow(t, report, "orphan dossier 111 is out of sample", row("111"))

	empty := newCorpus(t)
	zero, stderr, code := calibrate(t, empty.Dossiers, empty.Runs)
	if code != 0 {
		t.Errorf("RED: empty corpus exit=%d (want 0: empty is valid, missing is not): %s", code, firstLine(stderr))
	}
	if summaryCount(zero, "Valid pairs") != 0 || summaryCount(zero, "Excluded") != 0 {
		t.Errorf("RED: empty corpus must report `Valid pairs: 0` and `Excluded: 0`; got:\n%s", zero)
	}
}

func TestC1648_006_ReportIsDeterministicAcrossRuns(t *testing.T) {
	c := fixtureCorpus(t)
	first, stderr, code := calibrate(t, c.Dossiers, c.Runs)
	if code != 0 {
		t.Fatalf("RED: exit=%d: %s", code, firstLine(stderr))
	}
	second, _, _ := calibrate(t, c.Dossiers, c.Runs)
	if first != second {
		t.Errorf("RED: two runs differ:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if m := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`).FindString(first); m != "" {
		t.Errorf("RED: report embeds a timestamp %q — non-reproducible", m)
	}
	if pairs := cyclesInOrder(first, pairRowRe); len(pairs) != 5 || !isAscending(pairs) {
		t.Errorf("RED: pairs table rows %v — want the 5 valid cycles ascending", pairs)
	}
	if ex := cyclesInOrder(first, exclusionRowRe); len(ex) != 5 || !isAscending(ex) {
		t.Errorf("RED: exclusions table rows %v — want the 5 excluded cycles ascending", ex)
	}
}

func TestC1648_007_DefectClassBreakdownCountsNormalizedReasonClasses(t *testing.T) {
	c := fixtureCorpus(t)
	report, stderr, code := calibrate(t, c.Dossiers, c.Runs)
	if code != 0 {
		t.Fatalf("RED: exit=%d: %s", code, firstLine(stderr))
	}
	requireRow(t, report, "class EGPS=2 (103,104)", row("EGPS", "2"))
	requireRow(t, report, "class verdict-conflict=2 (102,103)", row("verdict-conflict", "2"))
	forbidRow(t, report, "a raw reason must not be its own class", row(reasonEGPS, "2"))
	if line := findRow(report, row("102", "PASS", "PASS", "FAIL", "FAIL")); !strings.Contains(line, "verdict-conflict") {
		t.Errorf("RED: pair 102's row does not carry its verdict-conflict class: %q", line)
	}
	if line := findRow(report, row("101", "PASS", "PASS", "PASS", "PASS")); line == "" || strings.Contains(line, "verdict-conflict") || strings.Contains(line, "EGPS") {
		t.Errorf("RED: pair 101 has no reason artifact yet its row carries a class (or is missing): %q", line)
	}
}

func TestC1648_008_ProjectRootDefaultsResolveCorpusPathsExplicitDirsWin(t *testing.T) {
	c := fixtureCorpus(t)
	projectRoot := filepath.Dir(filepath.Dir(c.Runs))
	out := filepath.Join(t.TempDir(), "by-root.md")
	_, stderr, code := runEvolve(t, "audit", "calibration", "--project-root", projectRoot, "--output", out)
	if code != 0 {
		t.Fatalf("RED: --project-root defaults exit=%d: %s", code, firstLine(stderr))
	}
	byRoot, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	explicit, _, _ := calibrate(t, c.Dossiers, c.Runs)
	if string(byRoot) != explicit {
		t.Errorf("RED: --project-root defaults and explicit dirs render different reports")
	}
	empty := newCorpus(t)
	emptyRoot := filepath.Dir(filepath.Dir(empty.Runs))
	withBoth, _, code := calibrate(t, c.Dossiers, c.Runs, "--project-root", emptyRoot)
	if code != 0 || summaryCount(withBoth, "Valid pairs") != 5 {
		t.Errorf("RED: explicit --dossiers-dir/--runs-dir must override --project-root defaults (exit=%d, pairs=%d)", code, summaryCount(withBoth, "Valid pairs"))
	}
}

func TestC1648_009_RealCorpusProducesInterpretableReport(t *testing.T) {
	runs := filepath.Join(stateRoot(t), ".evolve", "runs")
	dossiers := filepath.Join(acsassert.RepoRoot(t), "knowledge-base", "cycles")
	if _, err := os.Stat(filepath.Join(runs, "cycle-1640", "audit-chain-shadow.json")); err != nil {
		t.Skipf("runtime state absent (%v) — real-corpus check not applicable", err)
	}
	if _, err := os.Stat(filepath.Join(dossiers, "cycle-1640.json")); err != nil {
		t.Skipf("dossier absent (%v)", err)
	}
	report, stderr, code := calibrate(t, dossiers, runs)
	if code != 0 {
		t.Fatalf("RED: real corpus exit=%d: %s", code, firstLine(stderr))
	}
	if n := summaryCount(report, "Valid pairs"); n < 1 {
		t.Errorf("RED: real corpus yields %d valid pairs (want >= 1)", n)
	}
	if n := summaryCount(report, "Excluded"); n < 0 {
		t.Errorf("RED: real corpus report has no `Excluded: N` line")
	}
	requireRow(t, report, "cycle-1640 override row", row("1640", "PASS", "FAIL", gateExplain))
	requireRow(t, report, "cycle-1640 pair row", row("1640", "PASS", "PASS", "FAIL", "FAIL", gateExplain))
}

func TestC1648_010_NewPackageIsEnrolledInTheRepoWideAPICoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	pkgDir := filepath.Join(goDir, "internal", "auditcalibration")
	if st, err := os.Stat(pkgDir); err != nil || !st.IsDir() {
		t.Fatalf("RED: %s does not exist — the calibration package is not implemented", pkgDir)
	}
	// acs-predicate: config-check — enrollment is inherently a config-presence fact.
	if !acsassert.FileContains(t, filepath.Join(goDir, ".apicover-enforce"), "./internal/auditcalibration") {
		t.Errorf("RED: go/.apicover-enforce lacks `./internal/auditcalibration` (ADR-0069 second gate)")
	}
	named := "go/internal/auditcalibration/apicover_named_test.go"
	if !acsassert.FileExists(t, filepath.Join(root, named)) {
		t.Errorf("RED: %s missing", named)
	} else if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", named); code != 0 {
		t.Errorf("RED: %s untracked — dropped at ship (cycle-93 lesson)", named)
	}
	if _, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "vet", "./internal/auditcalibration"); code != 0 {
		t.Errorf("RED: go vet ./internal/auditcalibration failed:\n%s", stderr)
	}
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "cover.txt")
	if out, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile="+profile, "./internal/auditcalibration"); code != 0 {
		t.Fatalf("RED: go test ./internal/auditcalibration exit=%d\n%s%s", code, out, stderr)
	}
	funcOut, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("RED: go tool cover -func: %s", stderr)
	}
	funcFile := filepath.Join(tmp, "cover.func.txt")
	mustWrite(t, funcFile, funcOut)
	var so, se bytes.Buffer
	if rc := apicover.Main([]string{"-enforce", "-cover", funcFile, pkgDir}, &so, &se); rc != 0 {
		t.Errorf("RED: apicover -enforce ./internal/auditcalibration exit=%d (an unnamed or 0%%-covered export)\n%s%s", rc, so.String(), se.String())
	}
}

func TestC1648_011_BuilderUnitTestsCoverNegativeAndOverrideCases(t *testing.T) {
	assertBoundTestsPass(t, calibrationPkg,
		"TestAuditCalibration_MalformedOrMissingPairIsExcluded",
		"TestAuditCalibration_NarrativePassForceOverriddenToFail")
	assertBoundTestsPass(t, cmdEvolvePkg,
		"TestAuditCalibration_InvalidRootFailsLoudly",
		"TestAuditCalibration_DeterministicMarkdown")
}

const (
	calibrationPkg = "./internal/auditcalibration"
	cmdEvolvePkg   = "./cmd/evolve"
)

func assertBoundTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", pattern, pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, _ := cmd.CombinedOutput()
	combined := string(out)
	for _, n := range names {
		if !strings.Contains(combined, "--- PASS: "+n) {
			t.Errorf("RED: binding test %s did NOT pass in %s (renamed, missing, or failing):\n%s", n, pkg, combined)
		}
	}
	if strings.Contains(combined, "--- FAIL:") {
		t.Errorf("RED: a bound test failed in %s:\n%s", pkg, combined)
	}
}
