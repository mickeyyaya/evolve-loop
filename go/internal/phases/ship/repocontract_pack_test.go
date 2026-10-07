package ship

import (
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const ratchetRed = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"

const ratchetMessage = "internal/cyclesimulator/characterization_test.go builds a raw git repo (1 init sites) outside internal/gittest"

func TestRunRepoContractPack_RunsTheGatesOwnPackAndNamesItsReds(t *testing.T) {
	red := redPack(ratchetRed)
	red.failureLog = "    rawgitratchet_test.go:49: " + ratchetMessage + "\n"
	dirs := swapRepoContractTest(t, red)
	reds, diagnostic, err := RunRepoContractPack(context.Background(), "/lane")
	if err == nil || len(reds) != 1 || reds[0] != ratchetRed {
		t.Fatalf("a red pack names its failing test with the run error; got reds=%v err=%v", reds, err)
	}
	if !strings.Contains(diagnostic, ratchetMessage) {
		t.Fatalf("a red's diagnostic is the failing tests' own output; got %q", diagnostic)
	}
	if len(*dirs) != 1 || (*dirs)[0] != "/lane/go" {
		t.Fatalf("the build floor's run must go through the ship gate's own pack seam, once, in the tree's go/ module exactly as the gate does; ran in %v", *dirs)
	}
}

func TestRunRepoContractPack_NamesRedsOnlyForTheGatesRealRed(t *testing.T) {
	swapRepoContractTest(t, greenPack())
	if reds, diagnostic, err := RunRepoContractPack(context.Background(), "/m"); err != nil || len(reds) != 0 || diagnostic != "" {
		t.Fatalf("a green pack is no red, no diagnostic and no error; got reds=%v diagnostic=%q err=%v", reds, diagnostic, err)
	}
	swapRepoContractTest(t, packOutcome{failures: []packFailure{packFailureOf("pkg.TestX")}})
	if reds, _, err := RunRepoContractPack(context.Background(), "/m"); err != nil || len(reds) != 0 {
		t.Fatalf("named failures with a clean exit are green by the gate's rule (realRed), so they name nothing; got reds=%v err=%v", reds, err)
	}
	swapRepoContractTest(t, ambiguousPack())
	reds, diagnostic, err := RunRepoContractPack(context.Background(), "/m")
	if err == nil || err.Error() != "signal: killed" || len(reds) != 0 || !strings.Contains(diagnostic, scanChatter) {
		t.Fatalf("an exit that names no test keeps its own error and carries the pack's own output; got reds=%v diagnostic=%q err=%v", reds, diagnostic, err)
	}
}

func TestContractRed_NamesEverySuiteOfTheOnePackList(t *testing.T) {
	err := contractRed("scanner pack", redPack("pkg.TestX"))
	for _, pattern := range repoContractPackages {
		suite := path.Base(strings.TrimSuffix(pattern, "/..."))
		if !strings.Contains(err.Error(), suite) {
			t.Errorf("the ship's red message names the pack's suites from the one list; %q missing from %q", suite, err.Error())
		}
	}
	want := "fixed scanner pack (phasespec, profiles, phasecoherence, routingtest, rawgitratchet, sizeratchet, testmainexit, repocontract, policy, guards, acssuite, fleet, evalqualitycheck, inboxrank)"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("the ship's red message stays byte-identical; want %q in %q", want, err.Error())
	}
}

func TestRepoContractSelectionArgs_RunsEachSelectedTestByNameInItsPackages(t *testing.T) {
	args := repoContractSelectionArgs([]repocontract.TestSelection{
		{Package: "./internal/core", Tests: []string{"TestPhaseTimings_SingleWriter", "TestSeam_OneConstructionSite"}},
		{Package: "./cmd/evolve", Tests: []string{"TestSeam_OneConstructionSite"}},
	})
	want := append(repoContractTestArgs(nil, nil), "-run", "^(TestPhaseTimings_SingleWriter|TestSeam_OneConstructionSite)$", "./internal/core", "./cmd/evolve")
	if !slices.Equal(args, want) {
		t.Fatalf("the by-name run takes the gate's budgeted argv, one anchored -run of every selected test once, then the packages;\ngot  %q\nwant %q", args, want)
	}
}

func TestPackOutcomeMerged_KeepsEveryRunsRedsAndErrors(t *testing.T) {
	whole := redPack("pkg.TestWhole")
	whole.failureLog = "whole\n"
	byName := redPack("other.TestByName")
	byName.failureLog = "by name\n"
	merged := whole.merged(byName)
	if !merged.realRed() || !slices.Equal(merged.failedNames(), []string{"pkg.TestWhole", "other.TestByName"}) || merged.failureLog != "whole\nby name\n" {
		t.Fatalf("both runs' reds and output must reach the verdict; got %v %q %v", merged.failedNames(), merged.failureLog, merged.err)
	}
	if cancelled := ambiguousPack().merged(packOutcome{err: context.Canceled}); cancelled.green() || cancelled.realRed() {
		t.Fatalf("a cancel in either run stays ambiguous, never green or a named red; got %+v", cancelled)
	}
	if green := (packOutcome{}).merged(packOutcome{}); !green.green() {
		t.Fatalf("two green runs are green; got %+v", green)
	}
	if red := (packOutcome{}).merged(redPack("pkg.TestLate")); !red.realRed() {
		t.Fatalf("a red by-name run after a green whole pack is the pack's red; got %+v", red)
	}
}
