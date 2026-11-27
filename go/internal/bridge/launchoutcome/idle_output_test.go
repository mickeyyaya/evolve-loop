package launchoutcome

import "testing"

func TestIdleOutputLine(t *testing.T) {
	t.Parallel()
	line := IdleOutputLine(49)
	if want := "[bridge] artifact-timeout: cause=idle_output idle_s=49"; line != want {
		t.Fatalf("IdleOutputLine = %q, want %q", line, want)
	}
	if !TimeoutIdleOutput.Known() {
		t.Fatal("TimeoutIdleOutput is not Known")
	}
	if got := Classify(ExitArtifactTimeout, nil, line+"\n").CauseCode; got != string(TimeoutIdleOutput) {
		t.Fatalf("cause = %q, want %s", got, TimeoutIdleOutput)
	}
}
