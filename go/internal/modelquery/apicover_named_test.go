package modelquery

import (
	"context"
	"strings"
	"testing"
)

// TestModelCapturer_InterfaceContract names the ModelCapturer interface and
// pins its single-method contract: a CaptureModelPicker implementation is
// usable through the interface and its returned pane flows verbatim out of
// RecipeLister via the parser. fakeCapturer (defined in recipe_test.go)
// satisfies the interface — binding it to a ModelCapturer var proves the
// method set matches.
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

// TestRunner_DefaultRunnerExecutes names the Runner func type and invokes the
// production defaultRunner through a Runner-typed variable. Contract: Runner
// shells out to (name, args), returns combined stdout+stderr and a nil error on
// a clean exit. `true` exits 0 with no output on macOS and Linux.
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

// TestRunner_CapturesCombinedOutput pins the "combined stdout+stderr" half of
// Runner's contract and that args are passed through: `sh -c` writing to both
// streams must appear in the single returned string.
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
