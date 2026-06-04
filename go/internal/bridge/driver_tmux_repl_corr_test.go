package bridge

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmitChannelBreadcrumb_Format(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "inject_applied", "c1")
	if got := strings.TrimSpace(buf.String()); got != `{"evolve_channel":"inject_applied","corr_id":"c1"}` {
		t.Fatalf("breadcrumb = %s", got)
	}
}

func TestEmitChannelBreadcrumb_EmptyCorrIDNoOp(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "inject_applied", "")
	if buf.Len() != 0 {
		t.Fatalf("empty corr_id must not write, got %q", buf.String())
	}
}

func TestEmitChannelBreadcrumb_IdleReachedFormat(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "idle_reached", "c9")
	if got := strings.TrimSpace(buf.String()); got != `{"evolve_channel":"idle_reached","corr_id":"c9"}` {
		t.Fatalf("breadcrumb = %s", got)
	}
}

// NOTE: the busy→idle bracket integration (formerly
// TestRunTmuxREPL_EmitsBothBreadcrumbsOnBusyToIdle, which asserted breadcrumbs on
// stderr) moved to TestRunTmuxREPL_ChannelOn_BreadcrumbsToFile in
// driver_tmux_repl_panelive_test.go when RT2 (ADR-0037) redirected breadcrumbs to
// the <agent>-breadcrumbs.live file the Producer tails.
