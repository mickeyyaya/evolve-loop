package agentcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

const (
	usageHoldPolicy    = `{"cli_health":{"usage_hold_percent":90}}`
	usageResetsText    = "Nov 20, 4am (UTC)"
	usageResetsRFC3339 = "2099-11-20T04:00:00Z"
)

var (
	usageResetsAhead = time.Date(2099, 11, 20, 4, 0, 0, 0, time.UTC)
	usageResetsPast  = time.Date(2020, 1, 1, 4, 0, 0, 0, time.UTC)
)

func writeUsageHoldPolicy(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	mkErr := os.MkdirAll(dir, 0o755)
	if mkErr == nil {
		mkErr = os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644)
	}
	if mkErr != nil {
		t.Fatal(mkErr)
	}
}

func recordClaudeWeek(t *testing.T, root string, percent float64, resetsAt time.Time) {
	t.Helper()
	obs := usageprobe.Observation{CLI: "claude", ObservedAt: fixedNow(), Windows: []quotastate.UsageWindow{{
		Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: percent,
		ResetsText: usageResetsText, ResetsAt: &resetsAt, Family: "claude",
	}}}
	if err := usageprobe.RecordObservation(filepath.Join(root, ".evolve"), obs); err != nil {
		t.Fatal(err)
	}
}

func recordFailedProbe(t *testing.T, root string) {
	t.Helper()
	obs := usageprobe.Observation{CLI: "claude", ObservedAt: fixedNow(), Error: "the usage pane did not render"}
	if err := usageprobe.RecordObservation(filepath.Join(root, ".evolve"), obs); err != nil {
		t.Fatal(err)
	}
}

func corruptWindowsFile(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usageprobe.WindowsPath(filepath.Join(root, ".evolve")), []byte(`{"claude": {not json`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runSeat(t *testing.T, root string) (code int, stderr string, launches int) {
	t.Helper()
	fb := &fakeBridge{artifact: passReport}
	var out, errOut bytes.Buffer
	code = New(depsWith(fb))(runArgs(root), strings.NewReader(diff), &out, &errOut)
	return code, errOut.String(), len(fb.requests)
}

func TestUsageBrake_ASeatPastTheLimitIsRefusedAndTheTextNamesTheLimitTheReadingAndTheReset(t *testing.T) {
	cases := []struct {
		name    string
		percent float64
		reading string
	}{
		{"past the limit", 97, "97"},
		{"exactly at the limit", 90, "90"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			writeUsageHoldPolicy(t, root, usageHoldPolicy)
			recordClaudeWeek(t, root, tc.percent, usageResetsAhead)

			code, stderr, launches := runSeat(t, root)

			if code != exitRefused || launches != 0 {
				t.Fatalf("exit=%d launches=%d, want exit %d and no seat launched (stderr=%s)", code, launches, exitRefused, stderr)
			}
			for _, want := range []string{"90", tc.reading} {
				if !strings.Contains(stderr, want) {
					t.Errorf("refusal text %q does not name %q (the limit 90 and the reading %s)", stderr, want, tc.reading)
				}
			}
			if !strings.Contains(stderr, usageResetsRFC3339) && !strings.Contains(stderr, usageResetsText) {
				t.Errorf("refusal text %q does not name the reset (%s or %q)", stderr, usageResetsRFC3339, usageResetsText)
			}
			if !strings.Contains(strings.ToLower(stderr), "usage") {
				t.Errorf("refusal text %q does not say that the usage limit caused the refusal", stderr)
			}
		})
	}
}

func TestUsageBrake_ASeatUnderTheLimitOrWithTheBrakeOffOrAfterTheResetStillRuns(t *testing.T) {
	cases := []struct {
		name     string
		policy   string
		percent  float64
		resetsAt time.Time
	}{
		{"just under the limit", usageHoldPolicy, 89.9, usageResetsAhead},
		{"no hold percent in the policy", `{"cli_health":{}}`, 99, usageResetsAhead},
		{"no policy file at all", "", 99, usageResetsAhead},
		{"the window reset already", usageHoldPolicy, 99, usageResetsPast},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			if tc.policy != "" {
				writeUsageHoldPolicy(t, root, tc.policy)
			}
			recordClaudeWeek(t, root, tc.percent, tc.resetsAt)

			code, stderr, launches := runSeat(t, root)

			if code != 0 || launches != 1 {
				t.Errorf("exit=%d launches=%d, want a normal run: exit 0 and one seat launched (stderr=%s)", code, launches, stderr)
			}
		})
	}
}

func TestUsageBrake_AnUnreadableReadingIsNotUnderTheLimitAndASeatIsRefused(t *testing.T) {
	cases := []struct {
		name  string
		write func(t *testing.T, root string)
	}{
		{"the windows file is corrupt", corruptWindowsFile},
		{"the last probe recorded an error and no reading", recordFailedProbe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := project(t)
			writeUsageHoldPolicy(t, root, usageHoldPolicy)
			tc.write(t, root)

			code, stderr, launches := runSeat(t, root)

			if code != exitRefused || launches != 0 {
				t.Fatalf("exit=%d launches=%d, want exit %d and no seat launched: an unreadable reading must not read as under the limit (stderr=%s)", code, launches, exitRefused, stderr)
			}
			lower := strings.ToLower(stderr)
			if !strings.Contains(lower, "unreadable") || !strings.Contains(stderr, "90") {
				t.Errorf("refusal text %q must say that the reading is unreadable and name the limit 90", stderr)
			}
		})
	}
}

func TestUsageBrake_AnUnreadableReadingDoesNotRefuseASeatWhenTheBrakeIsOff(t *testing.T) {
	root := project(t)
	corruptWindowsFile(t, root)

	code, stderr, launches := runSeat(t, root)

	if code != 0 || launches != 1 {
		t.Errorf("exit=%d launches=%d, want a normal run when no hold percent is set (stderr=%s)", code, launches, stderr)
	}
}
