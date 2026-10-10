package usageevidence

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type signalledBridge struct {
	*fixtures.FakeBridge
	center *signalcenter.Center
}

func (b signalledBridge) Signals() *signalcenter.Center { return b.center }

func (b signalledBridge) SignalsWired() bool { return b.center != nil }

func exiting(code int) *fixtures.FakeBridge {
	return &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: code}}
}

func verdictFor(v usageprobe.Verdict) Explain {
	return func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
		return usageprobe.Evidence{CLI: "agy", Family: "agy-claude", Verdict: v, Detail: driver + " detail", ObservedAt: time.Now()}
	}
}

func readRecords(t *testing.T, workspace string) []Line {
	t.Helper()
	f, err := os.Open(filepath.Join(workspace, core.UsageEvidenceFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []Line
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var l Line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatalf("record %q: %v", sc.Text(), err)
		}
		out = append(out, l)
	}
	return out
}

func TestBridge_EveryFailureQuotaCouldExplainIsExplainedRecordedAndSignalled(t *testing.T) {
	for _, exit := range []int{80, 81, 85, 86, 1} {
		ws := t.TempDir()
		center := signalcenter.New()
		var explained []string
		inner := signalledBridge{FakeBridge: exiting(exit), center: center}
		b := Wrap(inner, func(ctx context.Context, driver string, since time.Time) usageprobe.Evidence {
			explained = append(explained, driver)
			return verdictFor(usageprobe.VerdictHealthy)(ctx, driver, since)
		}, inner.Signals)

		res, _ := b.Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Agent: "build", Cycle: 7, Workspace: ws})

		if res.ExitCode != exit || len(explained) != 1 || explained[0] != "agy-claude-tmux" {
			t.Fatalf("exit %d: response %+v, explained %v; want one usage query for the failing driver", exit, res, explained)
		}
		records := readRecords(t, ws)
		if len(records) != 1 || records[0].ExitCode != exit || records[0].Evidence.Verdict != usageprobe.VerdictHealthy || !strings.HasPrefix(records[0].Summary, "quota ruled out") {
			t.Errorf("exit %d: records = %+v", exit, records)
		}
		var signalled []signalcenter.Event
		for _, e := range center.Recent() {
			if e.Code == CodeUsageEvidence {
				signalled = append(signalled, e)
			}
		}
		if len(signalled) != 1 || signalled[0].Fields["verdict"] != "healthy" || signalled[0].Fields["exit_code"] != itoa(exit) || signalled[0].Cycle != 7 {
			t.Errorf("exit %d: signals = %+v", exit, signalled)
		}
	}
}

func TestBridge_ASuccessOrAMissingBinaryIsNotQuotaEvidence(t *testing.T) {
	for _, exit := range []int{0, 127} {
		ws := t.TempDir()
		calls := 0
		b := Wrap(exiting(exit), func(context.Context, string, time.Time) usageprobe.Evidence {
			calls++
			return usageprobe.Evidence{}
		}, nil)

		_, _ = b.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Workspace: ws})

		if calls != 0 || len(readRecords(t, ws)) != 0 {
			t.Errorf("exit %d: %d usage queries; quota cannot explain it", exit, calls)
		}
	}
}

func TestBridge_ACancelledLaunchQueriesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	b := Wrap(exiting(81), func(context.Context, string, time.Time) usageprobe.Evidence { calls++; return usageprobe.Evidence{} }, nil)

	_, _ = b.Launch(ctx, core.BridgeRequest{CLI: "claude-tmux", Workspace: t.TempDir()})

	if calls != 0 {
		t.Errorf("%d usage queries after the interrupt", calls)
	}
}

func TestBridge_ForwardsTheInnerSignalCenterAndItsWiringProof(t *testing.T) {
	center := signalcenter.New()
	b := Wrap(signalledBridge{FakeBridge: exiting(0), center: center}, nil, nil)
	if b.Signals() != center || !b.SignalsWired() {
		t.Fatalf("the decorator hides the inner bridge's Signal Center")
	}
	if Wrap(signalledBridge{FakeBridge: exiting(0)}, nil, nil).SignalsWired() {
		t.Errorf("SignalsWired is true without a Center")
	}
}

func TestReport_AnUnavailableVerdictIsAWarningAndNoWorkspaceIsOnlyASignal(t *testing.T) {
	center := signalcenter.New()
	ev := verdictFor(usageprobe.VerdictUnavailable)(context.Background(), "agy-tmux", time.Time{})

	if err := Report(center, Record{Origin: "TestReport", Driver: "agy-tmux", ExitCode: 80, Evidence: ev}); err != nil {
		t.Fatal(err)
	}

	recent := center.Recent()
	if len(recent) != 1 || recent[0].Severity != signalcenter.SeverityWarn || !strings.Contains(recent[0].Reason, "auth, install or network") {
		t.Fatalf("signals = %+v", recent)
	}
	if m, ok := signalcenter.IsRegistered(CodeUsageEvidence); !ok || m != signalcenter.ModuleBridge {
		t.Errorf("%s is not registered under the bridge module", CodeUsageEvidence)
	}
}

func TestBridge_OnlyAnExhaustedUsageVerdictMarksTheAttemptAsProvenExhausted(t *testing.T) {
	for _, tc := range []struct {
		verdict usageprobe.Verdict
		want    bool
	}{
		{usageprobe.VerdictExhausted, true},
		{usageprobe.VerdictHealthy, false},
		{usageprobe.VerdictUnknown, false},
		{usageprobe.VerdictUnavailable, false},
	} {
		t.Run(string(tc.verdict), func(t *testing.T) {
			b := Wrap(exiting(81), verdictFor(tc.verdict), nil)

			res, _ := b.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Agent: "build", Workspace: t.TempDir()})

			if res.UsageExhausted != tc.want {
				t.Errorf("a stall with a %s usage verdict: UsageExhausted=%v, want %v: only positive evidence proves a quota wall", tc.verdict, res.UsageExhausted, tc.want)
			}
		})
	}
}

func TestBridge_ARecordThatCannotBeWrittenStillMarksTheVerdictAndReturnsTheAttempt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	b := Wrap(exiting(81), verdictFor(usageprobe.VerdictExhausted), nil)

	res, _ := b.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Agent: "build", Workspace: missing})

	if res.ExitCode != 81 || !res.UsageExhausted {
		t.Errorf("res=%+v, want exit 81 marked exhausted: a failed record write never drops the evidence", res)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("the decorator created %s: it must only report the failed write", missing)
	}
}
