package config

import (
	"slices"
	"testing"
)

func TestSandboxMode_TheModesAreTheWireWordsOfSandboxMode(t *testing.T) {
	got := []string{SandboxModeAuto, SandboxModeOn, SandboxModeOff}
	if want := []string{"auto", "on", "off"}; !slices.Equal(got, want) {
		t.Errorf("sandbox modes = %v, want %v: policy.json sandbox.mode and the bridge read these words", got, want)
	}
}
