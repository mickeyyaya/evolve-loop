package modelquery

import (
	"context"
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
