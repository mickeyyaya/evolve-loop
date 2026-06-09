package core

import "testing"

func TestBridgePIDFile(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/ws/build-stdout.log": "/ws/build.bridge-pid",
		"/ws/tdd-stdout.log":   "/ws/tdd.bridge-pid",
		"build-stdout.log":     "build.bridge-pid",
		"":                     "",
		"/ws/build.log":        "", // not the -stdout.log convention
	}
	for in, want := range cases {
		if got := BridgePIDFile(in); got != want {
			t.Errorf("BridgePIDFile(%q) = %q, want %q", in, got, want)
		}
	}
}
