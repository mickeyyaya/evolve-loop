//go:build acs

package cycle2171

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdPkg = "./cmd/evolve"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func requireTestsPass(t *testing.T, names ...string) {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", cmdPkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: exit=%d err=%v\n%s\n%s", code, err, acsassert.OutputTail(out, 40), acsassert.OutputTail(errOut, 20))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2171_001_UncommittedControlPlaneFileAtPoolStartEmitsAWarnSignal(t *testing.T) {
	requireTestsPass(t,
		"TestPoolPreflight_UncommittedControlPlaneFileEmitsAWarnSignalNamingTheReason",
		"TestPoolPreflight_ACleanControlPlaneEmitsNoPreflightSignal",
		"TestPoolPreflight_AnEmptyBacklogIsNotAPreflightFailure",
	)
}

func TestC2171_002_WaveStatusNamesThePoolPreflightFailureReason(t *testing.T) {
	requireTestsPass(t,
		"TestWaveStatus_NamesThePoolPreflightFailureReason",
		"TestWaveStatus_JSONCarriesThePoolPreflightFailureReasonAsAWarning",
		"TestWaveStatus_IgnoresAPoolPreflightSignalFromBeforeTheWaveStarted",
		"TestWaveStatus_StaysSilentWhenNoPoolPreflightFailedAndOtherWaveSignalsExist",
	)
}

func TestC2171_003_EventsWatchOfTheErrorsChannelWakesOnThePreflightSignal(t *testing.T) {
	requireTestsPass(t,
		"TestPoolPreflight_TheErrorsChannelCarriesTheSignalAndEventsWatchWakesOnIt",
		"TestPoolPreflight_AWatchOfTheErrorsChannelStaysQuietForACleanControlPlane",
	)
}

func TestC2171_004_ChangedFilesCarryNoCodeComments(t *testing.T) {
	cmdDir := filepath.Join(goDir(t), "cmd", "evolve")
	for _, name := range []string{"cmd_loop_window.go", "cmd_wave_status.go", "cmd_loop_pool_preflight_signal_test.go"} {
		path := filepath.Join(cmdDir, name)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, group := range parsed.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") && !strings.HasPrefix(c.Text, "// acs-predicate:") {
					t.Errorf("%s carries the comment %q: code has no comments", name, c.Text)
				}
			}
		}
	}
}
