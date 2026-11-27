package bridge

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

type orderedCaptureTmux struct {
	*FakeTmuxController
	log *[]string
}

func (o orderedCaptureTmux) CapturePane(context.Context, string, int) (string, error) {
	*o.log = append(*o.log, "capture")
	return "❯", nil
}

func TestVerifyInjected_SettlesBeforeTheCaptureAndRecordsTheSite(t *testing.T) {
	t.Parallel()
	for _, site := range []string{"findings", "nudge"} {
		t.Run(site, func(t *testing.T) {
			t.Parallel()
			var log []string
			rec := interaction.NewRecorder("")
			w := replWaiter{
				ctx: context.Background(),
				cfg: &Config{Cycle: 7},
				deps: Deps{
					Tmux:   orderedCaptureTmux{FakeTmuxController: &FakeTmuxController{}, log: &log},
					Sleep:  func(d time.Duration) { log = append(log, "sleep "+d.String()) },
					Stderr: io.Discard,
				},
				launch:    tmuxLaunch{session: "s1", inputLineMarker: "❯"},
				recorder:  rec,
				phaseName: "build",
			}

			outcome := w.verifyInjected(site, "the payload")

			if len(log) < 2 || log[0] != "sleep "+submitVerifySettle.String() || log[1] != "capture" {
				t.Fatalf("order = %v, want the settle of %s before the first capture", log, submitVerifySettle)
			}
			if outcome.Result != interaction.ResultSubmitVerified {
				t.Errorf("result = %q, want %q", outcome.Result, interaction.ResultSubmitVerified)
			}
			rows := rec.Outcomes()
			if len(rows) != 1 || rows[0].Kind != interaction.KindSubmitVerify || rows[0].Payload != "site="+site+" resends=0" {
				t.Errorf("recorded = %+v, want one submit_verify row for site %s", rows, site)
			}
		})
	}
}
