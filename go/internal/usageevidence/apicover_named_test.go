package usageevidence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func TestQuotaCouldExplain_EveryNonZeroExitButAMissingBinary(t *testing.T) {
	for exit, want := range map[int]bool{0: false, 127: false, 1: true, 80: true, 81: true, 85: true, 86: true, 124: true} {
		if got := QuotaCouldExplain(exit); got != want {
			t.Errorf("QuotaCouldExplain(%d) = %v, want %v", exit, got, want)
		}
	}
}

func TestBridge_ProbeDelegatesToTheInnerBridge(t *testing.T) {
	var b core.Bridge = Wrap(exiting(0), nil, nil)
	if _, ok := b.(*Bridge); !ok {
		t.Fatalf("Wrap returned %T", b)
	}
	if _, err := b.Probe(context.Background()); err != nil {
		t.Errorf("Probe: %v", err)
	}
}

func TestReadWindowsNow_ReadsAScreenThroughItsCLIsManifest(t *testing.T) {
	windows := ReadWindowsNow("claude", usageFixture(t, "claude_usage_fable_exhausted.txt"))
	if len(windows) != 3 || windows[2].Model != "Fable" || !windows[2].Exhausted || windows[2].Kind != quotastate.KindWeek {
		t.Fatalf("windows = %+v; want claude's three windows with the Fable week exhausted", windows)
	}
}

func TestUsagePane_AFamilyWithNoUsageCommandIsUnsupportedWithoutABoot(t *testing.T) {
	tmux := &bridge.FakeTmuxController{}
	factory := bridge.NewControllerFactory(t.TempDir(), t.TempDir(), "test", bridge.Deps{Tmux: tmux})

	_, err := UsagePane(factory)(context.Background(), "ollama")

	if !errors.Is(err, clicontrol.ErrUnsupported) || len(tmux.SentKeys) != 0 {
		t.Fatalf("err = %v after %d keys; ollama declares no usage command, so nothing boots", err, len(tmux.SentKeys))
	}
}

func TestBridge_AnInnerBridgeWithoutASignalCenterStillRecordsTheEvidence(t *testing.T) {
	ws := t.TempDir()
	b := Wrap(exiting(81), verdictFor(usageprobe.VerdictUnknown), nil)

	_, _ = b.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Workspace: ws})

	if b.Signals() != nil || b.SignalsWired() {
		t.Errorf("an inner bridge with no Signal Center is forwarded as none")
	}
	if records := readRecords(t, ws); len(records) != 1 || records[0].Trigger != "exit 81" {
		t.Errorf("records = %+v; the workspace record does not need a Signal Center", records)
	}
}

func TestReport_ARecordThatCannotBeWrittenIsReturnedAndNamedInTheSignal(t *testing.T) {
	center := signalcenter.New()
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := Report(center, Record{Origin: "TestReport", Workspace: notADir, Driver: "agy-tmux", Trigger: "exit 80", ExitCode: 80, Evidence: verdictFor(usageprobe.VerdictHealthy)(context.Background(), "agy-tmux", time.Time{})})

	recent := center.Recent()
	if err == nil || len(recent) != 1 || recent[0].Fields["record_error"] == "" {
		t.Fatalf("err %v, signals %+v; an unwritable record must be returned and named in the signal", err, recent)
	}
}

func TestNew_ADriverOutsideTheRoutingTableIsQueriedByItsStem(t *testing.T) {
	var probed []string
	explain := New(Options{ProjectRoot: t.TempDir(), Probe: func(_ context.Context, family string) (string, error) {
		probed = append(probed, family)
		return "", nil
	}})

	ev := explain(context.Background(), "itest-tmux", time.Time{})

	if len(probed) != 1 || probed[0] != "itest" || ev.CLI != "itest" {
		t.Fatalf("probed %v, evidence %+v; an unregistered driver's screen is its stem's", probed, ev)
	}
}
