package detectcli

import (
	"errors"
	"testing"
)

// TestResult_DetectExplicitOverride names the detectcli.Result type (returned by
// Detect but never named in a test) and pins the full struct contract for the
// deterministic explicit-override branch: Options.Platform wins and Detect
// reports both the chosen CLI and the matching reason.
func TestResult_DetectExplicitOverride(t *testing.T) {
	got := Detect(Options{
		Platform: "custom",
		Env:      func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", errors.New("unused") },
	})

	want := Result{CLI: "custom", Reason: "explicit override via Options.Platform"}
	if got != want {
		t.Errorf("Detect = %+v, want %+v", got, want)
	}
}
