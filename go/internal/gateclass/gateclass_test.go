package gateclass

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func constitution(path string) bool {
	return path == "go/internal/policy/healing.go" || path == "go/internal/phasecontract/diagnosis.go" || strings.HasPrefix(path, "go/internal/gateclass/")
}

func protectedGuards(path string) bool {
	return strings.HasPrefix(path, "go/internal/guards/") || constitution(path)
}

func modified(path string, lines ...string) string {
	return "diff --git a/" + path + " b/" + path + "\nindex 1111111..2222222 100644\n--- a/" + path + "\n+++ b/" + path + "\n@@ -1,1 +1,1 @@\n" + strings.Join(lines, "\n") + "\n"
}

func created(path string, lines ...string) string {
	return "diff --git a/" + path + " b/" + path + "\nnew file mode 100644\nindex 0000000..2222222\n--- /dev/null\n+++ b/" + path + "\n@@ -0,0 +1,1 @@\n" + strings.Join(lines, "\n") + "\n"
}

func deleted(path string, lines ...string) string {
	return "diff --git a/" + path + " b/" + path + "\ndeleted file mode 100644\nindex 1111111..0000000\n--- a/" + path + "\n+++ /dev/null\n@@ -1,1 +0,0 @@\n" + strings.Join(lines, "\n") + "\n"
}

func patternsOf(flags []Flag) []Pattern {
	var out []Pattern
	for _, f := range flags {
		out = append(out, f.Pattern)
	}
	return out
}

func TestGateClassifier_FlagsEachWeakeningPattern(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   Input
		want Flag
	}{
		{"touches a protected path", Input{Diff: modified("go/internal/guards/role.go", "-\treturn deny", "+\treturn allow")},
			Flag{Pattern: PatternProtectedPath, Path: "go/internal/guards/role.go"}},
		{"removes a test of a guard package", Input{Diff: modified("go/internal/sizeratchet/ratchet_test.go", "-func TestRatchet_RefusesGrowth(t *testing.T) {", "-}")},
			Flag{Pattern: PatternRemovedGuardTest, Path: "go/internal/sizeratchet/ratchet_test.go", Detail: "TestRatchet_RefusesGrowth"}},
		{"adds t.Skip to a test that ran", Input{Diff: modified("go/internal/widget/widget_guard_test.go", "+\tt.Skip(\"flaky\")")},
			Flag{Pattern: PatternAddedSkip, Path: "go/internal/widget/widget_guard_test.go", Detail: "t.Skip(\"flaky\")"}},
		{"changes a threshold or a budget constant", Input{Diff: modified("go/internal/loop/halt.go", "-const zeroShipHaltThreshold = 2", "+const zeroShipHaltThreshold = 3")},
			Flag{Pattern: PatternChangedThreshold, Path: "go/internal/loop/halt.go", Detail: "zeroShipHaltThreshold 2 -> 3"}},
		{"moves a stage key off enforce", Input{Diff: modified(".evolve/policy.json", "-      \"ste_stage\": \"enforce\",", "+      \"ste_stage\": \"shadow\",")},
			Flag{Pattern: PatternStageOffEnforce, Path: ".evolve/policy.json", Detail: "ste_stage enforce -> shadow"}},
		{"changes a frozen acceptance check", Input{Cycle: 12, Diff: modified("go/acs/cycle9/predicate_test.go", "-\tif got != 3 {", "+\tif got < 0 {")},
			Flag{Pattern: PatternFrozenCheck, Path: "go/acs/cycle9/predicate_test.go"}},
		{"changes the catalog, the budgets or the tier table", Input{Diff: modified("go/internal/policy/healing.go", "-\thealingMaxPerPass  = 2", "+\thealingMaxPerPass  = 9")},
			Flag{Pattern: PatternHealingConstitution, Path: "go/internal/policy/healing.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.Protected, tc.in.Constitution = protectedGuards, constitution
			flags := Classify(tc.in)
			for _, f := range flags {
				if f.Pattern == tc.want.Pattern {
					if f != tc.want {
						t.Fatalf("flag = %+v, want %+v", f, tc.want)
					}
					return
				}
			}
			t.Fatalf("Classify = %+v, want a %s flag", flags, tc.want.Pattern)
		})
	}
}

func TestGateClassifier_PassesAnOrdinaryChange(t *testing.T) {
	t.Parallel()
	diff := modified("go/internal/widget/widget.go", "-\treturn 1", "+\treturn 2") +
		modified("go/internal/widget/widget_test.go", "+func TestWidget_ReturnsTwo(t *testing.T) {", "+}") +
		created("go/internal/widget/new_test.go", "+func TestNew(t *testing.T) {", "+\tt.Skip(\"a new test\")", "+}") +
		created("go/acs/cycle12/predicate_test.go", "+package cycle12") +
		modified("go/internal/widget/limits.go", "-const maxWidgets = 2", "+const maxWidgets = 3") +
		modified("go/internal/policy/stage_test.go", "-\tStage: \"enforce\",", "+\tStage: \"shadow\",", "-\tThreshold: 2,", "+\tThreshold: 3,") +
		modified("go/internal/loop/halt.go", "-\tcoverageFloor  = 80", "+\tcoverageFloor = 80") +
		modified(".evolve/policy.json", "+      \"x_stage\": \"shadow\",") +
		created("go/acs/cycle3/p_test.go", "+package cycle3") +
		modified(".evolve/phases/a.json", "-      \"a_stage\": \"enforce\",", "+      \"a_stage\":  \"enforce\",") +
		modified(".evolve/phases/b.json", "-      \"a_stage\": \"enforce\",", "+      \"b_stage\": \"shadow\",", "+      \"a_stage\": \"enforce\",")

	if flags := Classify(Input{Diff: diff, Cycle: 12, Protected: protectedGuards}); len(flags) != 0 {
		t.Fatalf("Classify = %+v, want no flag for an ordinary change", flags)
	}
	if flags := Classify(Input{Diff: "\n", Untracked: []string{"go/internal/widget/new.go"}, Protected: protectedGuards}); len(flags) != 0 {
		t.Fatalf("Classify = %+v, want no flag for an empty diff and an ordinary new file", flags)
	}
}

func TestGateClassifier_KeepsAMovedSkipAndAMovedTest(t *testing.T) {
	t.Parallel()
	diff := modified("go/internal/widget/widget_test.go", "-\tt.Skip(\"slow\")", "+\tt.Skip(\"slow\")") +
		deleted("go/internal/guards/old_test.go", "-func TestGuard_Refuses(t *testing.T) {") +
		created("go/internal/guards/new_test.go", "+func TestGuard_Refuses(t *testing.T) {")

	got := patternsOf(Classify(Input{Diff: diff}))

	if len(got) != 0 {
		t.Fatalf("Classify = %v, want no flag for a moved skip and a moved guard test", got)
	}
}

func TestGateClassifier_FlagsARemovedContractTestInAnyPackageAndAnUntrackedProtectedFile(t *testing.T) {
	t.Parallel()
	in := Input{
		Diff:      modified("go/internal/bridge/sandbox_test.go", "-func TestAgentSandboxContract(t *testing.T) {"),
		Untracked: []string{"go/internal/guards/sneaky.go", "go/internal/widget/new.go"},
		Protected: protectedGuards,
	}

	want := []Flag{
		{Pattern: PatternRemovedGuardTest, Path: "go/internal/bridge/sandbox_test.go", Detail: "TestAgentSandboxContract"},
		{Pattern: PatternProtectedPath, Path: "go/internal/guards/sneaky.go"},
	}
	if got := Classify(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
}

func TestGateClassifier_ReadsAGoStageIdentifierAndAFrozenCheckOfAnUnknownCycle(t *testing.T) {
	t.Parallel()
	diff := modified("go/internal/docsfloor/config.go", "-\tSteStage: config.StageEnforce,", "+\tSteStage: config.StageOff,") +
		deleted("go/acs/regression/flagreaders/readers_test.go", "-package flagreaders")

	got := patternsOf(Classify(Input{Diff: diff}))

	if want := []Pattern{PatternStageOffEnforce, PatternFrozenCheck}; !reflect.DeepEqual(got, want) {
		t.Fatalf("patterns = %v, want %v", got, want)
	}
}

func TestTierOf_NamesTheTierOfEachSurface(t *testing.T) {
	t.Parallel()
	cases := map[string]Tier{
		"go/internal/guards/role.go":             T0,
		"go/internal/policy/healing.go":          T0,
		"go/internal/gateclass/gateclass.go":     T0,
		"go/internal/core/orchestrator.go":       T1,
		"go/internal/bridge/driver.go":           T1,
		"go/internal/cliroute/router.go":         T1,
		"go/internal/gc/gc.go":                   T1,
		"agents/evolve-scout.md":                 T1,
		"go/internal/widget/widget.go":           T2,
		"docs/architecture/llm-isolation.md":     T2,
		"go/internal/corestate/corestate.go":     T2,
		"go/internal/phasecontract/diagnosis.go": T0,
	}
	for path, want := range cases {
		if got := TierOf(path, protectedGuards); got != want {
			t.Errorf("TierOf(%s) = %s, want %s", path, got, want)
		}
	}
}

func TestViolationEvent_NamesTheCodeAndEachFlag(t *testing.T) {
	t.Parallel()
	flags := []Flag{{Pattern: PatternAddedSkip, Path: "a_test.go", Detail: "t.Skip()"}, {Pattern: PatternFrozenCheck, Path: "go/acs/cycle1/p_test.go"}}

	e := ViolationEvent(7, "run-7", "ship", flags)

	if e.Code != CodeI10Violation || e.Cycle != 7 || e.RunID != "run-7" || e.Fields["patterns"] != "added-skip,frozen-check" {
		t.Fatalf("event = %+v", e)
	}
	if !strings.Contains(e.Reason, "a_test.go") || !strings.Contains(e.Reason, "go/acs/cycle1/p_test.go") {
		t.Errorf("reason %q does not name each path", e.Reason)
	}
}

func fakeGit(outputs map[string]string, fail string) (func(args ...string) (string, error), *[][]string) {
	var calls [][]string
	return func(args ...string) (string, error) {
		calls = append(calls, args)
		verb := args[2]
		if verb == fail {
			return "", errors.New(verb + " failed")
		}
		return outputs[verb], nil
	}, &calls
}

func TestRead_RunsTheDiffAndTheUntrackedListingThatClassifyParses(t *testing.T) {
	t.Parallel()
	git, calls := fakeGit(map[string]string{"diff": "the diff", "ls-files": "a b.go\x00c.go\x00"}, "")

	in, err := Read(git, "base1")

	if err != nil || in.Diff != "the diff" || !reflect.DeepEqual(in.Untracked, []string{"a b.go", "c.go"}) {
		t.Fatalf("Read = %+v, %v", in, err)
	}
	want := [][]string{
		{"-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "-U0", "--no-renames", "base1"},
		{"-c", "core.quotePath=false", "ls-files", "--others", "--exclude-standard", "-z"},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("git calls = %v, want %v", *calls, want)
	}
}

func TestRead_ReturnsEachGitFailure(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"diff", "ls-files"} {
		git, _ := fakeGit(nil, verb)
		if _, err := Read(git, "HEAD"); err == nil || !strings.Contains(err.Error(), verb) {
			t.Errorf("Read with a failed %s: err = %v", verb, err)
		}
	}
}

func TestGateClassifier_FlagsASecondSkipBesideAMovedOne(t *testing.T) {
	t.Parallel()
	diff := modified("go/internal/widget/widget_test.go", "-\tt.Skip(\"slow\")", "+\tt.Skip(\"slow\")", "+\tt.Skip(\"slow\")")

	want := []Flag{{Pattern: PatternAddedSkip, Path: "go/internal/widget/widget_test.go", Detail: "t.Skip(\"slow\")"}}
	if got := Classify(Input{Diff: diff}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
}

func TestGateClassifier_ReadsTheVocabularyOfEachPattern(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, diff string
		want       Pattern
	}{
		{"budget", modified("a/b.go", "-\ttokenBudget = 2", "+\ttokenBudget = 3"), PatternChangedThreshold},
		{"floor", modified("a/b.go", "-\tshipFloor = 2", "+\tshipFloor = 3"), PatternChangedThreshold},
		{"halt", modified("a/b.go", "-\thaltAfter = 2", "+\thaltAfter = 3"), PatternChangedThreshold},
		{"ratchet", modified("a/b.go", "-\tratchetStep = 2", "+\tratchetStep = 3"), PatternChangedThreshold},
		{"coverage", modified("a/b.go", "-\tminCoverage = 80", "+\tminCoverage = 70"), PatternChangedThreshold},
		{"typed const", modified("a/b.go", "-const maxThreshold int = 2", "+const maxThreshold int = 3"), PatternChangedThreshold},
		{"json key", modified("a.json", "-  \"coverage_min\": 80,", "+  \"coverage_min\": 70,"), PatternChangedThreshold},
		{"regression test", modified("go/acs/regression/flagreaders/readers_test.go", "-func TestReaders(t *testing.T) {"), PatternRemovedGuardTest},
		{"guard file name", modified("go/internal/w/kill_guard_test.go", "-func TestKill(t *testing.T) {"), PatternRemovedGuardTest},
		{"SkipNow", modified("go/internal/w/w_test.go", "+\tt.SkipNow()"), PatternAddedSkip},
		{"Skipf", modified("go/internal/w/w_test.go", "+\tt.Skipf(\"x %d\", 1)"), PatternAddedSkip},
		{"typed stage constant", modified("go/internal/w/w.go", "-\tstage = config.StageEnforce", "+\tstage = config.StageShadow"), PatternStageOffEnforce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := patternsOf(Classify(Input{Diff: tc.diff, Cycle: 1})); !slices.Contains(got, tc.want) {
				t.Fatalf("patterns = %v, want %s", got, tc.want)
			}
		})
	}
}

func TestGateClassifier_ATestOfTheSameNameInAnUnrelatedPackageIsNoMove(t *testing.T) {
	t.Parallel()
	diff := modified("go/internal/sizeratchet/ratchet_test.go", "-func TestRatchet(t *testing.T) {") +
		created("go/internal/widget/widget_test.go", "+func TestRatchet(t *testing.T) {")

	if got := patternsOf(Classify(Input{Diff: diff})); !reflect.DeepEqual(got, []Pattern{PatternRemovedGuardTest}) {
		t.Fatalf("patterns = %v, want the removed guard test", got)
	}
}

func TestParseDiff_KeepsAChangedLineThatLooksLikeAHeader(t *testing.T) {
	t.Parallel()
	files := parseDiff(modified("db/q.sql", "--- an old comment", "+++ a new comment"))

	if len(files) != 1 || !reflect.DeepEqual(files[0].removed, []string{"-- an old comment"}) || !reflect.DeepEqual(files[0].added, []string{"++ a new comment"}) {
		t.Fatalf("parseDiff = %+v", files)
	}
}

func TestGateClassifier_AContractTestMovedInItsDirectoryIsNoRemoval(t *testing.T) {
	t.Parallel()
	diff := modified("go/internal/bridge/sandbox_test.go", "-func TestAgentSandboxContract(t *testing.T) {") +
		created("go/internal/bridge/sandbox_contract_test.go", "+func TestAgentSandboxContract(t *testing.T) {")

	if got := Classify(Input{Diff: diff}); len(got) != 0 {
		t.Fatalf("Classify = %+v, want no flag for a contract test moved inside its directory", got)
	}
}
