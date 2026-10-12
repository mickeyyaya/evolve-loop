//go:build integration

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunFleetSoak_WithoutAnInjectedLauncherRunsTheBinaryInSimulateMode(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantHash string
	}{
		{"the default goal hash", nil, "soak-noop"},
		{"an explicit goal hash", []string{"--goal-hash", "explicit-hash"}, "explicit-hash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e2StubSoak(t, nil)
			e2StubFleetExecutable(t, fakeCycleBinary(t, "echo \"args:$*\"\nexit 0\n"), nil)
			var stdout, stderr bytes.Buffer
			args := append([]string{"--count", "2", "--evolve-dir", t.TempDir()}, tc.args...)

			code := runFleetSoak(args, nil, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			want := "args:cycle run --goal-hash " + tc.wantHash + " -simulate\n"
			if got := strings.Count(stdout.String(), want); got != 2 {
				t.Fatalf("the child ran %d times with %q, want 2:\n%s", got, want, stdout.String())
			}
		})
	}
}

func TestRunFleetSoak_AChildThatFailsFailsTheSoak(t *testing.T) {
	e2StubSoak(t, nil)
	e2StubFleetExecutable(t, fakeCycleBinary(t, "exit 9\n"), nil)
	var stdout, stderr bytes.Buffer

	code := runFleetSoak([]string{"--count", "1", "--evolve-dir", t.TempDir()}, nil, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "1/1 launches failed") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
