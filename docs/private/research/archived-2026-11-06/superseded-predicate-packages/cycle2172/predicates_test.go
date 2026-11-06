//go:build acs

package cycle2172

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPackage        = "./cmd/evolve"
	panestreamPackage = "./internal/bridge/panestream"
	journeyPrefix     = "TestWaveStatus_SeatHealth_"
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

func TestC2172_001_APaneAtItsPromptWithNoDeliverableReadsBlockedThroughWaveStatus(t *testing.T) {
	requireTests(t, cmdPackage, journeyPrefix+"APaneAtItsPromptWithNoDeliverableReadsBlocked")
}

func TestC2172_002_ADeadSessionWithAnOpenRunReadsZombieAndAnUnreachableSocketDoesNot(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ADeadSessionWithAnOpenRunReadsZombie",
		journeyPrefix+"AnUnreachableSocketNeverReadsZombie",
	)
}

func TestC2172_003_TheViewIsReadOnlyAndShowsNoSeatForAClosedRun(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"TheViewReadsTmuxThroughTheListSeamAndChangesNothing",
		journeyPrefix+"AClosedRunShowsNoSeat",
	)
}

func TestC2172_004_ADeliverableOutranksEveryOtherSeatState(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ADeliverableMakesTheSeatIdleEvenAtItsPrompt",
		journeyPrefix+"ADeliverableOutranksADeadSession",
	)
}

func TestC2172_005_WorkingAndStalledSplitOnTheStallWindow(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ABusyPaneWithFreshOutputReadsWorking",
		journeyPrefix+"ABusyPaneSilentForHoursReadsStalled",
	)
}

func TestC2172_006_ThePureClassifierNamesOneStatePerFactSet(t *testing.T) {
	requireTests(t, panestreamPackage,
		"TestSeatHealth_ClassifySeatNamesOneStatePerFactSet",
		"TestSeatHealth_StringNamesEachStateAndNoOtherValue",
	)
}

func TestC2172_007_EveryFunctionOfTheClassifierFileIsFullyCovered(t *testing.T) {
	coverFunc := classifierCoverFunc(t)
	file := fileDeclaring(t, coverFunc, "ClassifySeat")
	funcs := 0
	for _, line := range strings.Split(coverFunc, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.HasPrefix(fields[0], file+":") {
			continue
		}
		funcs++
		if pct := fields[len(fields)-1]; pct != "100.0%" {
			t.Errorf("%s in %s covers %s, want 100.0%%", fields[1], file, pct)
		}
	}
	if funcs < 2 {
		t.Errorf("%s declares %d function(s), want ClassifySeat and String at least", file, funcs)
	}
	if pct, err := acsassert.FuncCoverage(coverFunc, "ClassifySeat"); err != nil || pct != 100 {
		t.Errorf("ClassifySeat coverage = %v, %v, want 100", pct, err)
	}
}

func classifierCoverFunc(t *testing.T) string {
	t.Helper()
	scratch := t.TempDir()
	profile := filepath.Join(scratch, "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var coverFunc string
	for _, args := range [][]string{
		{"test", "-count=1", "-run", "^TestSeatHealth_", "-coverprofile=" + profile, panestreamPackage},
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
		coverFunc = out.String()
	}
	return coverFunc
}

func fileDeclaring(t *testing.T, coverFunc, funcName string) string {
	t.Helper()
	for _, line := range strings.Split(coverFunc, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == funcName {
			file, _, _ := strings.Cut(fields[0], ":")
			return file
		}
	}
	t.Fatalf("cover output lists no func %s", funcName)
	return ""
}

// acs-predicate: config-check
func TestC2172_008_TheSpecAndThePanestreamNoteDescribeTheBuiltSeatHealth(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, "docs", "architecture", "packages", "internal-bridge-panestream.md")
	spec := filepath.Join(root, "docs", "architecture", "regression-proof-and-supervision.md")
	acsassert.FileContains(t, note, "ClassifySeat")
	acsassert.FileContains(t, note, "SeatHealth")
	acsassert.FileContains(t, spec, "ClassifySeat")
}

func TestC2172_009_ALastSessionOfASocketThatDiedReadsZombieAgainstRealTmux(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ADeadSessionTheLastOneOnItsSocketReadsZombie",
		journeyPrefix+"AListFailureThatIsNotAGoneServerNeverReadsZombie",
		journeyPrefix+"ARealTmuxSocketWhoseLastSessionDiedReadsZombie",
		journeyPrefix+"ARealTmuxSessionAmongOthersReadsBlockedAtItsPrompt",
	)
}

func TestC2172_010_OneSharedFunctionListsTheTmuxSessions(t *testing.T) {
	requireTests(t, cmdPackage, "TestBridgeTmuxListing_OneFunctionOwnsTheListSessionsCall")
}

// acs-predicate: config-check
func TestC2172_011_TheSpecSaysTheViewStartsNoServerSeatOrAgent(t *testing.T) {
	spec := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "regression-proof-and-supervision.md")
	acsassert.FileContains(t, spec, "starts no server")
	acsassert.FileNotContains(t, spec, "starts no process")
}

func TestC2172_012_ADeliverableOfAnEarlierRoundNeverHidesTheSeatOfThisRound(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ADeliverableOfAnEarlierRoundNeverHidesTheStateOfThisRound",
		journeyPrefix+"ADeliverableWrittenByThisRoundReadsIdle",
		journeyPrefix+"ADeliverableMakesTheSeatIdleEvenAtItsPrompt",
		journeyPrefix+"ADeliverableOutranksADeadSession",
	)
}

func TestC2172_013_AListFailureThatIsNotAGoneServerReachesTheOperatorAsAWarning(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"AListFailureThatIsNotAGoneServerWarnsTheOperatorAndKeepsTheSeat",
		journeyPrefix+"AGoneServerIsAZombieNotAWarning",
		journeyPrefix+"AHealthyListingRaisesNoWarning",
	)
}

func TestC2172_014_ATmuxListFailureWithNoStderrEndsInNoEmptyReason(t *testing.T) {
	requireTests(t, cmdPackage, "TestBridgeTmuxListing_AFailureWithNoStderrNamesNoEmptyReason")
}

func TestC2172_015_ARealTmuxSessionIsListedUnderACLocale(t *testing.T) {
	requireTests(t, cmdPackage,
		journeyPrefix+"ARealTmuxSessionReadsBlockedUnderACLocale",
		"TestBridgeSessions_ARealTmuxSessionListsAsUnwatchedUnderACLocale",
	)
}
