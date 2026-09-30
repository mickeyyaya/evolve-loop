package modelquery

import (
	"context"
	"strings"
	"testing"
)

func TestModelCapturer_InterfaceContract(t *testing.T) {
	t.Parallel()
	var c ModelCapturer = fakeCapturer{panes: map[string]string{"claude": claudePickerPane}}

	pane, err := c.CaptureModelPicker(context.Background(), "claude")
	if err != nil {
		t.Fatalf("CaptureModelPicker: %v", err)
	}
	if pane != claudePickerPane {
		t.Errorf("captured pane = %q, want the claude picker fixture", pane)
	}
}

func TestRunner_DefaultRunnerExecutes(t *testing.T) {
	t.Parallel()
	var run Runner = defaultRunner

	out, err := run(context.Background(), "true", nil, "")
	if err != nil {
		t.Fatalf("Runner(true): %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("`true` should produce no output, got %q", out)
	}
}

func TestRunner_CapturesCombinedOutput(t *testing.T) {
	t.Parallel()
	var run Runner = defaultRunner

	out, err := run(context.Background(), "sh", []string{"-c", "echo to-stdout; echo to-stderr 1>&2"}, "")
	if err != nil {
		t.Fatalf("Runner(sh): %v", err)
	}
	if !strings.Contains(out, "to-stdout") || !strings.Contains(out, "to-stderr") {
		t.Errorf("combined output missing a stream: %q", out)
	}
}
