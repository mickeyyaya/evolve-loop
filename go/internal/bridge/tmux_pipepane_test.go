package bridge

import (
	"context"
	"testing"
)

// TestPipePaneArgs verifies the pure arg-builder that backs execTmux.PipePane.
// We test this helper directly (rather than through execTmux, which shells out
// to a real tmux binary) so the production arg logic is actually exercised —
// consistent with how other arg-building helpers in this package are tested.
func TestPipePaneArgs_StartForm(t *testing.T) {
	args := pipePaneArgs("my-session", "cat >> /tmp/out.log")
	want := []string{"pipe-pane", "-o", "-t", "my-session", "cat >> /tmp/out.log"}
	if len(args) != len(want) {
		t.Fatalf("start-form: got %v, want %v", args, want)
	}
	for i, v := range want {
		if args[i] != v {
			t.Errorf("args[%d] = %q, want %q", i, args[i], v)
		}
	}
}

func TestPipePaneArgs_StopForm(t *testing.T) {
	// Empty shellCmd → stop piping (-o toggle with no command appended).
	args := pipePaneArgs("my-session", "")
	want := []string{"pipe-pane", "-o", "-t", "my-session"}
	if len(args) != len(want) {
		t.Fatalf("stop-form: got %v, want %v", args, want)
	}
	for i, v := range want {
		if args[i] != v {
			t.Errorf("args[%d] = %q, want %q", i, args[i], v)
		}
	}
}

// TestFakeTmux_PipePane_Recording verifies that the recording fake correctly
// records start-form and stop-form calls as distinct entries.
func TestFakeTmux_PipePane_Recording(t *testing.T) {
	ctx := context.Background()
	f := &fakeTmux{}

	if err := f.PipePane(ctx, "sess", "cat >> /x"); err != nil {
		t.Fatalf("PipePane start: %v", err)
	}
	if err := f.PipePane(ctx, "sess", ""); err != nil {
		t.Fatalf("PipePane stop: %v", err)
	}

	if len(f.pipePaneCalls) != 2 {
		t.Fatalf("expected 2 recorded calls, got %d", len(f.pipePaneCalls))
	}
	if f.pipePaneCalls[0] != "cat >> /x" {
		t.Errorf("call[0] = %q, want %q", f.pipePaneCalls[0], "cat >> /x")
	}
	if f.pipePaneCalls[1] != "" {
		t.Errorf("call[1] = %q, want empty (stop-form)", f.pipePaneCalls[1])
	}
}
