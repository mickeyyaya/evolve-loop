package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func TestBridgeModelCapturer_CapturesThroughTheResolvedDriver(t *testing.T) {
	t.Parallel()
	boot := errors.New("no tmux in a test")
	c := bridgeModelCapturer{workspace: t.TempDir(), deps: bridge.Deps{Tmux: &bridge.FakeTmuxController{NewSessionErr: boot}}}

	_, err := c.CaptureModelPicker(context.Background(), "claude")

	if !errors.Is(err, boot) {
		t.Fatalf("err=%v, want the capture to reach the fake pane of the claude driver", err)
	}
}

func TestBridgeModelCapturer_AnUnknownCLIIsRefusedBeforeAnySession(t *testing.T) {
	t.Parallel()
	tmux := &bridge.FakeTmuxController{}
	c := bridgeModelCapturer{workspace: t.TempDir(), deps: bridge.Deps{Tmux: tmux}}

	_, err := c.CaptureModelPicker(context.Background(), "nosuchcli")

	var unknown *bridge.UnknownFamilyError
	if !errors.As(err, &unknown) || len(tmux.Events) != 0 {
		t.Fatalf("err=%v events=%v, want the typed refusal and no pane", err, tmux.Events)
	}
}

func TestCanary_AnUnknownBenchedFamilyIsNotProbedAndKeepsItsBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "nosuchfamily", 1)
	var probed []string
	var log bytes.Buffer

	runCLIHealthCanary(context.Background(), root, nil, func(driver string) (int, string, string) {
		probed = append(probed, driver)
		return 0, "", ""
	}, &log)

	if len(probed) != 0 {
		t.Errorf("probed %v, want no probe: an unknown family has no driver", probed)
	}
	if _, benched := clihealth.NewStore(root, nil).Expired()["nosuchfamily"]; !benched {
		t.Errorf("the bench of an unknown family was cleared, want it untouched")
	}
	if !strings.Contains(log.String(), `"nosuchfamily"`) || !strings.Contains(log.String(), "bench untouched") {
		t.Errorf("log=%q, want the refusal named", log.String())
	}
}

func TestFamilyEvidence_AnUnknownFamilyIsUnavailableWithoutAQuery(t *testing.T) {
	t.Parallel()
	var asked []string
	explain := func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
		asked = append(asked, driver)
		return usageprobe.Evidence{Verdict: usageprobe.VerdictHealthy}
	}

	ev := familyEvidence(explain)(context.Background(), "nosuchfamily")

	if ev.Verdict != usageprobe.VerdictUnavailable || ev.Family != "nosuchfamily" || !strings.Contains(ev.Detail, "nosuchfamily") || len(asked) != 0 {
		t.Errorf("evidence=%+v asked=%v, want unavailable with the reason and no query", ev, asked)
	}
}

func TestUsageResetFrom_AnUnknownFamilyIsSkipped(t *testing.T) {
	t.Parallel()
	var asked []string
	explain := func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
		asked = append(asked, driver)
		return usageprobe.Evidence{Verdict: usageprobe.VerdictHealthy}
	}

	_, ok := usageResetFrom(explain)([]string{"nosuchfamily", "agy"}, time.Now())

	if ok || !slices.Equal(asked, []string{"agy-tmux"}) {
		t.Errorf("ok=%v asked=%v, want only the agy driver asked", ok, asked)
	}
}
