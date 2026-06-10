package bridge

// faketmux_amplify_test.go — adversarial tests for FakeTmuxController (cycle-276
// T1). The builder's tmux_repl_fixture_test.go proves the three named fixture
// scenarios (boot-success, boot-timeout, artifact-delivery). These tests probe
// the controller's own behavioral contracts: panic-on-underrun, event-recording
// fidelity, and multi-operation ordering — all properties the ACS predicates
// rely on but don't directly test.

import (
	"context"
	"strings"
	"testing"
)

// TestFakeTmuxController_UnderrunPanics verifies the panic-on-underrun
// contract: calling CapturePane when CaptureFrames is empty must panic with
// the documented message. The panic makes misuse visible at test-design time
// (a missing frame = a wrong fixture, not a silent empty-pane return).
func TestFakeTmuxController_UnderrunPanics(t *testing.T) {
	t.Parallel()
	f := &FakeTmuxController{} // CaptureFrames is nil / empty
	ctx := context.Background()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("CapturePane on empty queue must panic; it did not")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value must be a string, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "underrun") {
			t.Fatalf("panic message must mention 'underrun'; got %q", msg)
		}
	}()

	_, _ = f.CapturePane(ctx, "test-session", 0)
}

// TestFakeTmuxController_UnderrunAfterExhaustion verifies panic fires on the
// first call AFTER the queue is emptied, not only when the queue starts empty.
func TestFakeTmuxController_UnderrunAfterExhaustion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{CaptureFrames: []string{"only-frame"}}

	// First call consumes the one queued frame — must succeed.
	got, err := f.CapturePane(ctx, "s", 0)
	if err != nil || got != "only-frame" {
		t.Fatalf("first CapturePane: got=%q err=%v, want 'only-frame' nil", got, err)
	}

	// Second call exhausts the queue — must panic.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CapturePane after queue exhaustion must panic")
		}
	}()
	_, _ = f.CapturePane(ctx, "s", 0)
}

// TestFakeTmuxController_EventOrderRecording verifies that Events records
// each operation in the exact call order: new-session → send → capture →
// load-buffer → paste-buffer → kill-session. This ordering is the observable
// contract that tests like TestCodexUpdateMenuDismiss use to assert that
// Skip (SendKeys) precedes the PasteBuffer inject.
func TestFakeTmuxController_EventOrderRecording(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{
		CaptureFrames: []string{"pane-content"},
	}

	_ = f.NewSession(ctx, "sess", 220, 50)
	_ = f.SendKeys(ctx, "sess", "2", true)
	_, _ = f.CapturePane(ctx, "sess", 0)
	_ = f.LoadBuffer(ctx, "sess", "/tmp/prompt.txt")
	_ = f.PasteBuffer(ctx, "sess")
	_ = f.KillSession(ctx, "sess")

	want := []string{
		"new-session:sess",
		"send:2|true",
		"capture",
		"load-buffer",
		"paste-buffer",
		"kill-session:sess",
	}
	if len(f.Events) != len(want) {
		t.Fatalf("Events length=%d, want %d: %v", len(f.Events), len(want), f.Events)
	}
	for i, w := range want {
		if f.Events[i] != w {
			t.Errorf("Events[%d]=%q, want %q", i, f.Events[i], w)
		}
	}
}

// TestFakeTmuxController_SendKeysRecording verifies that SentKeys accumulates
// just the key strings (no enter flag), and SentSeq accumulates the full
// "{keys}|{enter}" wire encoding used for ordering assertions.
func TestFakeTmuxController_SendKeysRecording(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{}

	_ = f.SendKeys(ctx, "s", "2", true)      // menu Skip
	_ = f.SendKeys(ctx, "s", "hello", false) // no trailing Enter

	if len(f.SentKeys) != 2 || f.SentKeys[0] != "2" || f.SentKeys[1] != "hello" {
		t.Fatalf("SentKeys=%v, want [\"2\" \"hello\"]", f.SentKeys)
	}
	if len(f.SentSeq) != 2 {
		t.Fatalf("SentSeq length=%d, want 2", len(f.SentSeq))
	}
	if !strings.Contains(f.SentSeq[0], "|true") {
		t.Errorf("SentSeq[0]=%q: enter=true must appear in the sequence string", f.SentSeq[0])
	}
	if !strings.Contains(f.SentSeq[1], "|false") {
		t.Errorf("SentSeq[1]=%q: enter=false must appear in the sequence string", f.SentSeq[1])
	}
}

// TestFakeTmuxController_PasteCountIncrement verifies that PasteCount
// increments by 1 per PasteBuffer call regardless of the session argument.
func TestFakeTmuxController_PasteCountIncrement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{}
	if f.PasteCount != 0 {
		t.Fatalf("initial PasteCount=%d, want 0", f.PasteCount)
	}
	_ = f.PasteBuffer(ctx, "sess-a")
	_ = f.PasteBuffer(ctx, "sess-b")
	_ = f.PasteBuffer(ctx, "sess-a")
	if f.PasteCount != 3 {
		t.Fatalf("PasteCount=%d after 3 pastes, want 3", f.PasteCount)
	}
}

// TestFakeTmuxController_KillRecorded verifies KillSession appends session
// names to KilledSessions in call order.
func TestFakeTmuxController_KillRecorded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{}
	_ = f.KillSession(ctx, "alpha")
	_ = f.KillSession(ctx, "beta")
	if len(f.KilledSessions) != 2 || f.KilledSessions[0] != "alpha" || f.KilledSessions[1] != "beta" {
		t.Fatalf("KilledSessions=%v, want [alpha beta]", f.KilledSessions)
	}
}

// TestFakeTmuxController_HasSessionUnknownReturnsFalse verifies that querying
// an unregistered session name returns false without panic.
func TestFakeTmuxController_HasSessionUnknownReturnsFalse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{}
	if f.HasSession(ctx, "nonexistent") {
		t.Fatal("HasSession for an unregistered session must return false")
	}
}

// TestFakeTmuxController_FrameQueueIsConsumingFIFO verifies that CapturePane
// returns frames in FIFO order — the first-queued frame is the first returned.
func TestFakeTmuxController_FrameQueueIsConsumingFIFO(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &FakeTmuxController{
		CaptureFrames: []string{"frame-1", "frame-2", "frame-3"},
	}
	for i, want := range []string{"frame-1", "frame-2", "frame-3"} {
		got, err := f.CapturePane(ctx, "s", 0)
		if err != nil {
			t.Fatalf("CapturePane[%d]: unexpected error: %v", i, err)
		}
		if got != want {
			t.Errorf("CapturePane[%d]=%q, want %q (FIFO order)", i, got, want)
		}
	}
}
