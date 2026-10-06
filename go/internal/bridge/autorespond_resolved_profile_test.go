package bridge

import (
	"context"
	"testing"
	"time"
)

func TestAutoResponder_AgySpinnerWithoutFooterGatesEscalateLikeTheCheckpoint(t *testing.T) {
	busyPane := "Which absolute path should I write the deliverable to?\n⣷  Editing files...\n────\n>\n────\n"
	lp := agyLaunchForTest()
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{busyPane}}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newLaunchAutoResponder(t.TempDir(), deps, lp, false)
	ar.prompts = escalatePrompt

	if !deps.LivenessCenter.BusyOf(busyPane, paneProfileFor(lp)) {
		t.Fatal("precondition: the launch's resolved profile reads the spinner line as busy")
	}
	if _, rc := ar.tick(context.Background(), lp.session); rc != 0 {
		t.Errorf("rc = %d, want 0: the auto-responder must judge busy with the launch's resolved profile, as the checkpoint does", rc)
	}
}
