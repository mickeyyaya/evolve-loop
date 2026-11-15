//go:build acs

package cycle2411

import (
	"go/ast"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
)

var routerFiles = []string{"router_dispatch.go", "cmd_router_dispatch_golden_test.go", "cmd_router_dispatch_golden_helper_test.go"}

var bridgeFiles = []string{"cmd_bridge_sessions_test.go", "cmd_wave_status_seat_test.go", "cmd_wave_status_seat_repair_test.go",
	"cmd_wave_status_seat_warn_test.go", "cmd_wave_status_seat_noserver_test.go", "cmd_wave_test.go"}

var rootEventsFiles = []string{"cmd_root_events_test.go", "cmd_quality_test.go", "cmd_integration_stop_test.go"}

var productionFilesOfTheSeams = []string{"router_dispatch.go", "cmd_bridge_sessions.go", "cmd_wave_status_seat.go",
	"cmd_cycle.go", "cmd_quality.go", "cmd_integration_stop.go"}

var assertionBaseline = map[string]int{
	"TestBridgeSessions_ADeadWritersSnapshotIsNotListedAsLive":                                  2,
	"TestBridgeSessions_ASnapshotWhoseSessionIsGoneReadsGone":                                   1,
	"TestBridgeSessions_AnUnreachableSocketWarnsAndKeepsTheSnapshotState":                       2,
	"TestBridgeSessions_EmptyProjectSaysSo":                                                     2,
	"TestBridgeSessions_ListsEachLivePaneWithItsLiveness":                                       4,
	"TestBridgeSessions_LiveLoopSocketsShowUnwatchedSessions":                                   1,
	"TestBridgeSessions_TableNamesEveryColumn":                                                  2,
	"TestBridgeSessions_WritesNothing":                                                          1,
	"TestEmitIntegrationSignal_SendsTheEventThroughACenterOfTheIntegrationRole":                 2,
	"TestLegacyAdvisorDispatch_MatchesTheGolden":                                                1,
	"TestNewQualityCenter_AttachesThePublisherUnderTheQualityRole":                              1,
	"TestNewRoleSignalCenter_AnUnwritableEventsDirectoryNeverFailsTheRootAndWarnsOnce":          4,
	"TestNewRoleSignalCenter_PublishesUnderItsRoleAndTheCloserDrainsTheQueues":                  3,
	"TestRootSignalCenter_ThePublisherLeavesTheCycleRecordUnchanged":                            1,
	"TestRouterDispatch_ARefusedRouteIsAnError":                                                 1,
	"TestWaveStatus_SeatHealth_ABusyPaneSilentForHoursReadsStalled":                             1,
	"TestWaveStatus_SeatHealth_ABusyPaneWithFreshOutputReadsWorking":                            1,
	"TestWaveStatus_SeatHealth_AClosedRunShowsNoSeat":                                           2,
	"TestWaveStatus_SeatHealth_ADeadSessionTheLastOneOnItsSocketReadsZombie":                    1,
	"TestWaveStatus_SeatHealth_ADeadSessionWithAnOpenRunReadsZombie":                            2,
	"TestWaveStatus_SeatHealth_ADeliverableMakesTheSeatIdleEvenAtItsPrompt":                     1,
	"TestWaveStatus_SeatHealth_ADeliverableOfAnEarlierRoundNeverHidesTheStateOfThisRound":       1,
	"TestWaveStatus_SeatHealth_ADeliverableOutranksADeadSession":                                1,
	"TestWaveStatus_SeatHealth_ADeliverableWrittenByThisRoundReadsIdle":                         1,
	"TestWaveStatus_SeatHealth_AGoneServerIsAZombieNotAWarning":                                 2,
	"TestWaveStatus_SeatHealth_AHealthyListingRaisesNoWarning":                                  2,
	"TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerNeverReadsZombie":                1,
	"TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerWarnsTheOperatorAndKeepsTheSeat": 3,
	"TestWaveStatus_SeatHealth_APaneAtItsPromptWithNoDeliverableReadsBlocked":                   2,
	"TestWaveStatus_SeatHealth_AnUnreachableSocketNeverReadsZombie":                             1,
	"TestWaveStatus_SeatHealth_AnUnreadablePaneWatchFileWarnsAndShowsNoSeat":                    3,
	"TestWaveStatus_SeatHealth_TheViewReadsTmuxThroughTheListSeamAndChangesNothing":             4,
}

var (
	runsOnce      sync.Once
	routerRun     testRun
	otherClusters testRun
)

func movedRuns(t *testing.T) (testRun, testRun) {
	t.Helper()
	runsOnce.Do(func() {
		routerRun = runNamedTests(t, routerCluster, ambientRouterEnv())
		otherClusters = runNamedTests(t, stateWritingClusters(), nil)
	})
	return routerRun, otherClusters
}

func runOf(name string, router, others testRun) testRun {
	if slices.Contains(routerCluster, name) {
		return router
	}
	return others
}

func TestC2411_001_TheParallelGuardShowsEveryMovedTestAsParallel(t *testing.T) {
	tree := loadTree(t)
	cfg := tree.guardConfig(t)
	serial := tree.serialKeys(t)
	violations := paralleltest.Violations(tree.sources, cfg)
	for _, name := range allMoved() {
		fd := tree.topLevelTest(t, name)
		if fd == nil {
			continue
		}
		key := "main." + name
		if serial[key] {
			t.Errorf("%s is still listed in serialTests", key)
		}
		if !firstStatementIsParallel(fd) {
			t.Errorf("%s does not start with t.Parallel", name)
		}
		for _, v := range violations {
			if strings.HasPrefix(v, key+" ") || strings.HasPrefix(v, key+":") {
				t.Errorf("the guard reports: %s", v)
			}
		}
	}
}

func TestC2411_002_EveryMovedTestPausesAtRunTimeAndPasses(t *testing.T) {
	router, others := movedRuns(t)
	for _, name := range allMoved() {
		run := runOf(name, router, others)
		if !run.paused(name) {
			t.Errorf("%s never paused: it ran on the serial chain (go test tail:\n%s)", name, run.tail)
		}
		if !run.passed(name) {
			t.Errorf("%s did not pass (go test tail:\n%s)", name, run.tail)
		}
	}
}

func TestC2411_003_NoMovedTestSwapsASeamOrWritesTheProcessEnvironment(t *testing.T) {
	tree := loadTree(t)
	clusters := []struct {
		names []string
		scope reach
	}{
		{routerCluster, inFiles(routerFiles...)},
		{bridgeSessionsCluster, anyFunctionAnyMethodIn(bridgeFiles...)},
		{seatHealthCluster, anyFunctionAnyMethodIn(bridgeFiles...)},
		{rootEventsCluster, anyFunctionAnyMethodIn(rootEventsFiles...)},
	}
	for _, cluster := range clusters {
		for _, name := range cluster.names {
			root := tree.topLevelTest(t, name)
			if root == nil {
				continue
			}
			for _, fd := range tree.closure(root, cluster.scope) {
				testSide := strings.HasSuffix(tree.owner[fd], "_test.go")
				for _, m := range processStateMutations(fd, testSide) {
					t.Errorf("%s reaches %s, which %s", name, tree.where(fd), m)
				}
			}
		}
	}
}

func TestC2411_004_TheRouterTestsReadOnlyTheEnvironmentTheyInject(t *testing.T) {
	tree := loadTree(t)
	for _, name := range routerCluster {
		root := tree.topLevelTest(t, name)
		if root == nil {
			continue
		}
		for _, fd := range tree.closure(root, inFiles(routerFiles...)) {
			for _, m := range environmentReads(fd) {
				t.Errorf("%s reaches %s, which %s: the environment it reads is not the one it injects", name, tree.where(fd), m)
			}
		}
	}
	router, _ := movedRuns(t)
	for _, name := range routerCluster {
		if !router.passed(name) {
			t.Errorf("%s did not pass with EVOLVE_CLI and EVOLVE_ROUTER_CLI set to codex-tmux in the process (go test tail:\n%s)", name, router.tail)
		}
	}
}

func TestC2411_005_EveryMovedTestKeepsItsAssertions(t *testing.T) {
	tree := loadTree(t)
	router, others := movedRuns(t)
	for _, name := range allMoved() {
		fd := tree.topLevelTest(t, name)
		if fd == nil {
			continue
		}
		if got, want := assertionCount(fd), assertionBaseline[name]; got < want {
			t.Errorf("%s has %d assertions, it had %d before the move", name, got, want)
		}
		if run := runOf(name, router, others); !run.passed(name) {
			t.Errorf("%s does not pass (go test tail:\n%s)", name, run.tail)
		}
	}
}

func TestC2411_006_TheSerialChainOfTheMovedTestsIsEmptyAndTheSerialListIsShorter(t *testing.T) {
	tree := loadTree(t)
	router, others := movedRuns(t)
	var serialSeconds, parallelSeconds float64
	for _, name := range allMoved() {
		run := runOf(name, router, others)
		if run.paused(name) {
			parallelSeconds += run.elapsed(name)
		} else {
			serialSeconds += run.elapsed(name)
		}
	}
	if serialSeconds != 0 {
		t.Errorf("the moved tests still hold %.2fs of the serial chain of cmd/evolve", serialSeconds)
	}
	if parallelSeconds == 0 {
		t.Errorf("no moved test ran in the parallel phase")
	}
	if got, ceiling := len(tree.serialKeys(t)), serialEntriesAtBase-len(allMoved()); got > ceiling {
		t.Errorf("serialTests lists %d tests, want at most %d: the moved tests must leave the list", got, ceiling)
	}
}

func TestC2411_007_EveryFunctionOfTheSeamFilesIsReachedFromMainThroughProductionCode(t *testing.T) {
	tree := loadTree(t)
	reached := tree.productionReach(t)
	for _, file := range productionFilesOfTheSeams {
		for _, decl := range tree.files[file].Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil && !reached[fd] {
				t.Errorf("%s:%s has no production caller: a seam that only a test reaches is dead code", file, fd.Name.Name)
			}
		}
	}
}

func TestC2411_008_TheProductionDefaultsOfTheSeamsStayWired(t *testing.T) {
	tree := loadTree(t)
	production := func(owner string, _ bool) bool { return !strings.HasSuffix(owner, "_test.go") }
	wiring := []struct {
		entry string
		ident string
	}{
		{"runBridgeSessions", "bridgeSessionsTmux"},
		{"runBridgeSessions", "bridgeLiveSockets"},
		{"runBridgeSessions", "bridgeSessionsNow"},
		{"collectWaveSeats", "bridgeSessionsTmux"},
		{"newRoleSignalCenter", "attachRootEventsFn"},
	}
	for _, w := range wiring {
		var entry *ast.FuncDecl
		for _, fd := range tree.funcs[w.entry] {
			entry = fd
		}
		if entry == nil {
			t.Errorf("production entry %s is gone", w.entry)
			continue
		}
		if !tree.closureMentions(entry, production, w.ident) {
			t.Errorf("%s no longer reaches the production default %s", w.entry, w.ident)
		}
	}
	var wire *ast.FuncDecl
	for _, fd := range tree.funcs["wireRouterAdvisor"] {
		wire = fd
	}
	if wire == nil {
		t.Fatal("production entry wireRouterAdvisor is gone")
	}
	reads := 0
	for _, fd := range tree.closure(wire, production) {
		reads += len(environmentReads(fd))
	}
	if reads == 0 {
		t.Error("wireRouterAdvisor no longer reaches the process environment: production must still read os.Environ")
	}
}

func TestC2411_009_TheGuardsThatPinTheSeamDefaultsStayGreen(t *testing.T) {
	names := []string{
		"TestRootSignalCenter_EveryRootSubscribesThePublisher",
		"TestRootSignalCenter_EveryRootRunsUnderARoleOfTheClosedSet",
		"TestRootSignalCenter_EveryRootDefersItsCloser",
		"TestRootSignalCenter_NoRootDropsItsCloserAndEveryDepsLiteralCarriesIt",
		"TestBridgeTmuxListing_OneFunctionOwnsTheListSessionsCall",
		"TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial",
	}
	run := runNamedTests(t, names, nil)
	for _, name := range names {
		if !run.passed(name) {
			t.Errorf("%s did not pass (go test tail:\n%s)", name, run.tail)
		}
	}
}
