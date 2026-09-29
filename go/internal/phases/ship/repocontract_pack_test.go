package ship

import (
	"context"
	"path"
	"strings"
	"testing"
)

const ratchetRed = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"

func TestRunRepoContractPack_RunsTheGatesOwnPackAndNamesItsReds(t *testing.T) {
	dirs := swapRepoContractTest(t, redPack(ratchetRed))
	reds, err := RunRepoContractPack(context.Background(), "/lane/go")
	if err == nil || len(reds) != 1 || reds[0] != ratchetRed {
		t.Fatalf("a red pack names its failing test with the run error; got reds=%v err=%v", reds, err)
	}
	if len(*dirs) != 1 || (*dirs)[0] != "/lane/go" {
		t.Fatalf("the build floor's run must go through the ship gate's own pack seam, once, in the module dir it names; ran in %v", *dirs)
	}
}

func TestRunRepoContractPack_GreenNamesNothingAndAnUnnamedExitKeepsItsError(t *testing.T) {
	swapRepoContractTest(t, greenPack())
	if reds, err := RunRepoContractPack(context.Background(), "/m"); err != nil || len(reds) != 0 {
		t.Fatalf("a green pack is no red and no error; got reds=%v err=%v", reds, err)
	}
	swapRepoContractTest(t, ambiguousPack())
	if reds, err := RunRepoContractPack(context.Background(), "/m"); err == nil || len(reds) != 0 {
		t.Fatalf("an exit that names no test keeps its error and names nothing, so the caller can tell infra from a contract red; got reds=%v err=%v", reds, err)
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
	want := "fixed scanner pack (phasespec, profiles, phasecoherence, routingtest, rawgitratchet)"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("the ship's red message stays byte-identical; want %q in %q", want, err.Error())
	}
}
