package ship

import (
	"context"
	"path"
	"strings"
	"testing"
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
