//go:build acs

// Package cycle2554 holds the predicates of the stage verify and stage report contract.
package cycle2554

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	stageKey       = "inbox.curator.stage"
	stagePolicy    = `{"inbox":{"curator":{"stage":"shadow","limit":3}},"other":{"knob":1}}`
	evidencePkg    = "./internal/stageevidence"
	evolveCmdPkg   = "./cmd/evolve"
	exitNotReady   = 1
	exitBadUsage   = 10
	exitUnreadable = 2
)

var evolveBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cycle2554-evolve-")
	if err != nil {
		os.Exit(1)
	}
	evolveBinary = filepath.Join(dir, "evolve")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func builtEvolve(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(evolveBinary); err == nil {
		return evolveBinary
	}
	moduleDir := acsassert.GoModuleDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", evolveBinary, evolveCmdPkg)
	if code != 0 {
		t.Fatalf("RED: go build %s exited %d: %v\n%s", evolveCmdPkg, code, err, acsassert.OutputTail(stderr, 20))
	}
	return evolveBinary
}

func runGoTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s (go test exit %d)", name, pkg, code)
		}
	}
	if code != 0 {
		t.Errorf("RED: go test exited %d for %s:\n%s", code, pkg, acsassert.OutputTail(out, 25))
	}
}

func projectRoot(t *testing.T, policy string) string {
	t.Helper()
	root := acsassert.SetupTempProject(t)
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module fixture\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func stage(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	stdout, stderr, code, _ = acsassert.SubprocessOutput(builtEvolve(t), append([]string{"stage"}, args...)...)
	if code == -1 {
		t.Fatalf("evolve stage %v did not launch", args)
	}
	return stdout, stderr, code
}

func result(t *testing.T, stdout string) stageevidence.Result {
	t.Helper()
	var res stageevidence.Result
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("RED: evolve stage verify --json printed %q: %v", stdout, err)
	}
	return res
}

func currentVersion(t *testing.T, root string) string {
	t.Helper()
	stdout, _, _ := stage(t, "verify", stageKey, "--json", "--project-root", root)
	res := result(t, stdout)
	if res.Version == "" {
		t.Fatalf("RED: evolve stage verify prints no decision version: %q", stdout)
	}
	return res.Version
}

func writeChain(t *testing.T, version string, fieldsPerCycle ...map[string]string) (string, stageevidence.Chain) {
	t.Helper()
	chain := stageevidence.Chain{}
	for i, fields := range fieldsPerCycle {
		rec := stageevidence.Link("", stageevidence.Record{Key: stageKey, Version: version, Kind: stageevidence.KindDecision, ID: i + 1, Seq: i + 1, Fields: fields})
		chain.Records = append(chain.Records, rec)
		chain.Outcomes = append(chain.Outcomes, stageevidence.Outcome{ID: i + 1, Shipped: true})
	}
	return saveChain(t, chain), chain
}

func saveChain(t *testing.T, chain stageevidence.Chain) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "chain-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(chain); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func TestC2554_001_ScorerVersionDigestAndVerdictUnitTestsPass(t *testing.T) {
	runGoTests(t, evidencePkg,
		"TestVerdictRecomputesFromFixtureChain",
		"TestScorerMutationChangesGolden",
		"TestVersionDigestStableUnderUnrelatedChange",
		"TestVersionDigestChangesWithDeciderClosure",
		"TestEvidenceOfOtherVersionNotCounted",
	)
	runGoTests(t, evolveCmdPkg, "TestEveryStageHasScorerAndVerdict")
}

func TestC2554_002_AVerdictIsRecomputedFromAChainFileThroughTheRealVerb(t *testing.T) {
	root := projectRoot(t, stagePolicy)
	version := currentVersion(t, root)

	cleanPath, _ := writeChain(t, version, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "false"})
	stdout, stderr, code := stage(t, "verify", stageKey, "--json", "--project-root", root, "--chain", cleanPath)
	res := result(t, stdout)
	if code != 0 || !res.Ready || res.Decisions != 2 || res.FirstCycle != 1 || res.LastCycle != 2 || res.ChainDigest == "" || res.Version != version {
		t.Errorf("RED: clean chain: exit %d ready %v decisions %d range %d to %d digest %q version %q; stderr %s", code, res.Ready, res.Decisions, res.FirstCycle, res.LastCycle, res.ChainDigest, res.Version, stderr)
	}

	faultyPath, _ := writeChain(t, version, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "true"})
	stdout, _, code = stage(t, "verify", stageKey, "--json", "--project-root", root, "--chain", faultyPath)
	res = result(t, stdout)
	wantCase := stageevidence.Case{Class: stageevidence.ClassFalseStop, Cycle: 2, Seq: 2}
	if code != exitNotReady || res.Ready || res.FalseStops != 1 || !slices.Contains(res.Cases, wantCase) {
		t.Errorf("RED: chain with one false stop: exit %d ready %v false stops %d cases %+v", code, res.Ready, res.FalseStops, res.Cases)
	}
}

func TestC2554_003_ABrokenChainIsNotReadyWithATypedErrorAndNoPartialVerdict(t *testing.T) {
	root := projectRoot(t, stagePolicy)
	version := currentVersion(t, root)
	chain := stageevidence.Chain{Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true}}}
	prev := ""
	for seq := 1; seq <= 3; seq++ {
		rec := stageevidence.Link(prev, stageevidence.Record{Key: stageKey, Version: version, Kind: stageevidence.KindDecision, ID: 1, Seq: seq, Fields: map[string]string{"would_stop": "false"}})
		chain.Records = append(chain.Records, rec)
		prev = rec.Hash
	}
	editedMiddle := slices.Clone(chain.Records)
	editedMiddle[1].Fields = map[string]string{"would_stop": "true"}
	deletedMiddle := slices.Delete(slices.Clone(chain.Records), 1, 2)
	swapped := slices.Clone(chain.Records)
	swapped[1], swapped[2] = swapped[2], swapped[1]

	for name, records := range map[string][]stageevidence.Record{"edit": editedMiddle, "delete": deletedMiddle, "reorder": swapped} {
		path := saveChain(t, stageevidence.Chain{Records: records, Outcomes: chain.Outcomes})
		stdout, _, code := stage(t, "verify", stageKey, "--json", "--project-root", root, "--chain", path)
		res := result(t, stdout)
		if code != exitNotReady || res.Ready || res.Error != "broken-chain" || res.Decisions != 0 {
			t.Errorf("RED: %s: exit %d ready %v error %q decisions %d, want not ready, broken-chain and no partial verdict", name, code, res.Ready, res.Error, res.Decisions)
		}
	}
}

func TestC2554_004_EvidenceOfAnotherDecisionVersionIsReportedAndNotCounted(t *testing.T) {
	root := projectRoot(t, stagePolicy)
	stalePath, _ := writeChain(t, "version-from-an-older-decider", map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "false"})
	stdout, _, code := stage(t, "verify", stageKey, "--json", "--project-root", root, "--chain", stalePath)
	res := result(t, stdout)
	if code != exitNotReady || res.Ready || res.Decisions != 0 || res.OtherVersions["version-from-an-older-decider"] != 2 {
		t.Errorf("RED: stale evidence: exit %d ready %v decisions %d other versions %v", code, res.Ready, res.Decisions, res.OtherVersions)
	}
}

func TestC2554_005_AnUnrelatedPolicyEditKeepsTheEvidenceAndASubtreeEditDropsIt(t *testing.T) {
	root := projectRoot(t, stagePolicy)
	version := currentVersion(t, root)
	path, _ := writeChain(t, version, map[string]string{"would_stop": "false"})
	policyPath := filepath.Join(root, ".evolve", "policy.json")

	if err := os.WriteFile(policyPath, []byte(`{"inbox":{"curator":{"stage":"shadow","limit":3}},"other":{"knob":42,"added":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := stage(t, "verify", stageKey, "--json", "--project-root", root, "--chain", path)
	if res := result(t, stdout); res.Decisions != 1 || res.Version != version {
		t.Errorf("RED: after an unrelated policy edit: decisions %d version %s, want 1 and %s", res.Decisions, res.Version, version)
	}

	if err := os.WriteFile(policyPath, []byte(`{"inbox":{"curator":{"stage":"shadow","limit":4}},"other":{"knob":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ = stage(t, "verify", stageKey, "--json", "--project-root", root, "--chain", path)
	if res := result(t, stdout); res.Decisions != 0 || res.Version == version || res.OtherVersions[version] != 1 {
		t.Errorf("RED: after a subtree edit: decisions %d version %s other versions %v, want 0, a new version and the old one reported", res.Decisions, res.Version, res.OtherVersions)
	}
}

func TestC2554_006_TheReportHasOneScoredLinePerRegistryStageKeyOnTheRealTree(t *testing.T) {
	stdout, stderr, code := stage(t, "report", "--json", "--project-root", acsassert.RepoRoot(t))
	if code != 0 {
		t.Fatalf("RED: evolve stage report exited %d: %s", code, stderr)
	}
	var results []stageevidence.Result
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("RED: evolve stage report --json printed %q: %v", acsassert.OutputTail(stdout, 5), err)
	}
	var want, got []string
	for _, e := range gateregistry.StageEntries() {
		want = append(want, e.Shadow.PolicyKey)
	}
	for _, r := range results {
		got = append(got, r.Key)
		if r.Scorer == "" || r.Version == "" || r.Ready || r.Error != "empty-chain" {
			t.Errorf("RED: %s: scorer %q version %q ready %v error %q, want a scorer, a version and not ready on an empty chain", r.Key, r.Scorer, r.Version, r.Ready, r.Error)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("RED: the report lists %d keys, the registry has %d", len(got), len(want))
	}
}

func TestC2554_007_TheVerbRefusesAKeyOutsideTheRegistryAndAnUnreadableChain(t *testing.T) {
	root := projectRoot(t, stagePolicy)
	if _, _, code := stage(t, "verify", "not.a.stage.key", "--project-root", root); code != exitBadUsage {
		t.Errorf("RED: an unknown key exited %d, want %d", code, exitBadUsage)
	}
	if _, _, code := stage(t, "verify", stageKey, "--project-root", root, "--chain", filepath.Join(root, "missing.json")); code != exitUnreadable {
		t.Errorf("RED: an unreadable chain file exited %d, want %d", code, exitUnreadable)
	}
	if _, _, code := stage(t, "flip", stageKey); code != exitBadUsage {
		t.Errorf("RED: an unknown subcommand exited %d, want %d", code, exitBadUsage)
	}
}

func TestC2554_008_ThePackageIsEnrolledInTheApicoverGateAndPassesIt(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(enforce), "\n"), evidencePkg) {
		t.Fatalf("RED: %s is not listed in go/.apicover-enforce", evidencePkg)
	}
	moduleDir := acsassert.GoModuleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	if _, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-tags", "integration", "-coverprofile="+profile, evidencePkg); code != 0 {
		t.Fatalf("RED: go test %s exited %d:\n%s", evidencePkg, code, acsassert.OutputTail(stderr, 20))
	}
	funcs, _, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("RED: go tool cover exited %d", code)
	}
	funcFile := filepath.Join(t.TempDir(), "cover.func.txt")
	if err := os.WriteFile(funcFile, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(moduleDir, "internal", "stageevidence")
	if out, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "run", "./cmd/apicover", "-enforce", "-cover", funcFile, pkgDir); code != 0 {
		t.Errorf("RED: apicover -enforce exited %d for %s:\n%s", code, evidencePkg, acsassert.OutputTail(out+stderr, 25))
	}
}

func TestC2554_009_TheNewTestsRunWithoutSkipsAndEachCaseOfTheContractIsPinned(t *testing.T) {
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-v", "-count=1", evidencePkg)
	out := stdout + stderr
	if code != 0 {
		t.Fatalf("RED: go test %s exited %d:\n%s", evidencePkg, code, acsassert.OutputTail(out, 25))
	}
	if strings.Contains(out, "--- SKIP") {
		t.Errorf("RED: a stageevidence test skipped; a skipped test cannot prove the contract")
	}
	for _, name := range []string{
		"TestBrokenChainGivesNotReadyAndTypedError",
		"TestIncompleteAndLateEvidenceNeverCount",
		"TestPromotionDataDecidesReadiness",
		"TestIntactChainVerifiesAndEmptyChainIsNotReady",
		"TestFileChainChecksTheWholeChainBeforeItFilters",
		"TestEveryKeyGetsAScorer",
	} {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
	runGoTests(t, evolveCmdPkg,
		"TestStageVerifyRecomputesTheVerdictFromTheChainFile",
		"TestStageVerifyPrintsTheRangeTheDigestTheCountsAndEachCase",
		"TestStageVerifyBrokenChainIsNotReadyWithATypedError",
		"TestStageVerifyDoesNotCountEvidenceOfAnotherVersion",
		"TestStageVerbRefusesWhatItCannotRun",
	)
}
