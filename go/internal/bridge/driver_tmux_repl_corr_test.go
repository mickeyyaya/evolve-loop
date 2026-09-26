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
