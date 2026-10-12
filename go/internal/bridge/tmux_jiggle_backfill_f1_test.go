//go:build integration

package bridge

import (
	"context"
	"strings"
	"testing"
)

func TestRealTmuxBackfillF1_JiggleWindowIsANetZeroResizeOnALiveSession(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	ctx := context.Background()
	session := itSession("jiggle-f1")
	if err := itTmuxCtl.NewSession(ctx, session, 90, 30); err != nil {
		t.Fatalf("new-session: %v", err)
	}
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(context.Background(), session) })
	width := func() string {
		w, err := itTmuxCtl.run(ctx, "display-message", "-p", "-t", ExactSessionTarget(session), "#{window_width}")
		if err != nil {
			t.Fatalf("display-message: %v", err)
		}
		return strings.TrimSpace(w)
	}
	before := width()
	if err := itTmuxCtl.JiggleWindow(ctx, session); err != nil {
		t.Fatalf("JiggleWindow on a live session: %v", err)
	}
	if after := width(); after != before || before != "90" {
		t.Fatalf("window width %s -> %s, want an unchanged 90 (the two resizes must cancel)", before, after)
	}
}
