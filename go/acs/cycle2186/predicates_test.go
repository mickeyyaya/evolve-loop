//go:build acs

package cycle2186

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPackage        = "./cmd/evolve"
	panestreamPackage = "./internal/bridge/panestream"
	panewatchPackage  = "./internal/panewatch"
	journeyPrefix     = "TestWaveStatus_SeatHealth_"
	specDoc           = "docs/architecture/regression-proof-and-supervision.md"
	panestreamNote    = "docs/architecture/packages/internal-bridge-panestream.md"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func requireTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func coverFunc(t *testing.T, pkg, runPattern string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var report string
	for _, args := range [][]string{
		{"test", "-count=1", "-run", runPattern, "-coverprofile=" + profile, pkg},
		{"tool", "cover", "-func=" + profile},
	} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = goDir(t)
		cmd.Env = ipcenv.GoTestEnv()
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("go %s: %v\n%s%s", strings.Join(args, " "), err, out.String(), errOut.String())
		}
		report = out.String()
	}
	return report
}

func linesOfFile(report, file string) string {
	var kept []string
	for _, line := range strings.Split(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.HasSuffix(strings.SplitN(fields[0], ":", 2)[0], file) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func requireFullCoverage(t *testing.T, report string, funcs ...string) {
	t.Helper()
	for _, name := range funcs {
		pct, err := acsassert.FuncCoverage(report, name)
		if err != nil {
			t.Errorf("coverage of %s: %v", name, err)
			continue
		}
		if pct != 100 {
			t.Errorf("%s covers %.1f%%, want 100%%", name, pct)
		}
	}
}

func TestC2186_001_APaneAtItsPromptWithNoDeliverableReadsBlockedThroughWaveStatus(t *testing.T) {
	requireTests(t, cmdPackage, journeyPrefix+"APaneAtItsPromptWithNoDeliverableReadsBlocked")
}

func TestC2186_002_ADeadSessionWithAnOpenRunReadsZombieAndAnUnreachableSocketDoesNot(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ADeadSessionWithAnOpenRunReadsZombie",
		journeyPrefix+"AnUnreachableSocketNeverReadsZombie",
		journeyPrefix+"ADeadSessionTheLastOneOnItsSocketReadsZombie",
		journeyPrefix+"AListFailureThatIsNotAGoneServerNeverReadsZombie",
	)
}

func TestC2186_003_TheViewReadsTmuxOnlyThroughTheListSeamAndChangesNothing(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"TheViewReadsTmuxThroughTheListSeamAndChangesNothing",
		journeyPrefix+"AClosedRunShowsNoSeat",
		"TestBridgeTmuxListing_OneFunctionOwnsTheListSessionsCall",
	)
}

func TestC2186_004_ADeliverableOutranksEveryOtherSeatState(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ADeliverableMakesTheSeatIdleEvenAtItsPrompt",
		journeyPrefix+"ADeliverableOutranksADeadSession",
		journeyPrefix+"ADeliverableOfAnEarlierRoundNeverHidesTheStateOfThisRound",
		journeyPrefix+"ADeliverableWrittenByThisRoundReadsIdle",
	)
}

func TestC2186_005_WorkingAndStalledSplitOnTheStallWindow(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ABusyPaneWithFreshOutputReadsWorking",
		journeyPrefix+"ABusyPaneSilentForHoursReadsStalled",
	)
}

func TestC2186_006_ThePureClassifierNamesOneStatePerFactSet(t *testing.T) {
	requireTests(t, panestreamPackage,
		"TestSeatHealth_ClassifySeatNamesOneStatePerFactSet",
		"TestSeatHealth_StringNamesEachStateAndNoOtherValue",
		"TestSeatHealthAmplify_EveryFactCombinationFollowsTheFrozenRuleOrder",
	)
}

func TestC2186_007_TheClassifierFunctionsAreFullyCovered(t *testing.T) {
	report := coverFunc(t, panestreamPackage, "^TestSeatHealth")
	requireFullCoverage(t, linesOfFile(report, "seathealth.go"), "ClassifySeat", "String")
}

func TestC2186_008_EveryFunctionOfTheSeatViewIsFullyCovered(t *testing.T) {
	report := coverFunc(t, cmdPackage, "^(TestWaveStatus_SeatHealth_|TestBridgeTmuxListing_|TestBridgeSessions_)")
	requireFullCoverage(t, linesOfFile(report, "cmd_wave_status_seat.go"),
		"collectWaveSeats", "waveSessionsAlive", "appendNew", "socketsOfSnapshots",
		"waveDeliverableExists", "writeWaveSeatsText",
	)
	requireFullCoverage(t, linesOfFile(report, "cmd_bridge_sessions.go"), "tmuxServerGone", "socketActivity")
}

func TestC2186_009_ThePaneWatchTrackerStampsTheRoundStartAndIsFullyCovered(t *testing.T) {
	report := coverFunc(t, panewatchPackage, ".")
	requireFullCoverage(t, linesOfFile(report, "tracker.go"), "Observe")
}

func TestC2186_010_TheSpecAndThePanestreamNoteAreSteCleanAndDescribeTheSeatHealth(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, rel := range []string{specDoc, panestreamNote} {
		findings, err := stelint.LintFile(filepath.Join(root, filepath.FromSlash(rel)), stelint.Options{Words: words})
		if err != nil {
			t.Fatalf("lint %s: %v", rel, err)
		}
		for _, f := range findings {
			if !f.IsSkipNotice() {
				t.Errorf("%s:%d %s %s: %s", rel, f.Line, f.Rule, f.Message, f.Excerpt)
			}
		}
	}
	acsassert.FileContains(t, filepath.Join(root, panestreamNote), "ClassifySeat")
	acsassert.FileContains(t, filepath.Join(root, specDoc), "ClassifySeat")
}

func TestC2186_011_TheSpecSaysTheViewStartsNoServerSeatOrAgentAndSendsNoKey(t *testing.T) {
	spec := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(specDoc))
	acsassert.FileContains(t, spec, "starts no server, seat or agent")
	acsassert.FileContains(t, spec, "it sends no key")
	acsassert.FileNotContains(t, spec, "starts no process")
}

func TestC2186_012_ARealTmuxSocketWhoseLastSessionDiedReadsZombieAndASurvivorReadsBlocked(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ARealTmuxSocketWhoseLastSessionDiedReadsZombie",
		journeyPrefix+"ARealTmuxSessionAmongOthersReadsBlockedAtItsPrompt",
		journeyPrefix+"ARealTmuxSessionReadsBlockedUnderACLocale",
	)
}

func TestC2186_013_AListFailureThatIsNotAGoneServerReachesTheOperatorAsAWarning(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"AListFailureThatIsNotAGoneServerWarnsTheOperatorAndKeepsTheSeat",
		journeyPrefix+"AGoneServerIsAZombieNotAWarning",
		journeyPrefix+"AHealthyListingRaisesNoWarning",
		journeyPrefix+"AnUnreadablePaneWatchFileWarnsAndShowsNoSeat",
		"TestBridgeTmuxListing_AFailureWithNoStderrNamesNoEmptyReason",
	)
}
