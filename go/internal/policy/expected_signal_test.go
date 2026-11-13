package policy

import (
	"strings"
	"testing"
	"time"
)

func TestExpectedSignalParse_AcceptsTheBlockAndKeepsTheValues(t *testing.T) {
	t.Parallel()
	p, err := Parse([]byte(`{"expected_signal":{"wave_seconds":10,"phase_seconds":60}}`))
	if err != nil || p.ExpectedSignal == nil {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
	if *p.ExpectedSignal != (ExpectedSignalPolicy{WaveSeconds: 10, PhaseSeconds: 60}) {
		t.Errorf("block = %+v", *p.ExpectedSignal)
	}
	if _, err := Parse([]byte(`{"expected_signal":{"phase_seconds":60}}`)); err != nil {
		t.Errorf("a partial block parses: %v", err)
	}
}

func TestExpectedSignalParse_RejectsBadValues(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{"expected_signal":{"wave_seconds":-1}}`:   "wave_seconds",
		`{"expected_signal":{"phase_seconds":-1}}`:  "phase_seconds",
		`{"expected_signal":{"stage":"shadow"}}`:    "stage",
		`{"expected_signal":{"phase_seconds":"x"}}`: "expected_signal",
	}
	for body, want := range cases {
		if _, err := Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want mention of %q", body, err, want)
		}
	}
}

func TestExpectedSignalDeadline_DefaultsApplyToAMissingBlockAndOmittedKeys(t *testing.T) {
	t.Parallel()
	var none *ExpectedSignalPolicy
	partial := &ExpectedSignalPolicy{PhaseSeconds: 60}
	cases := []struct {
		p    *ExpectedSignalPolicy
		key  string
		want time.Duration
	}{
		{none, "expected_signal.wave_seconds", 57600 * time.Second},
		{none, "expected_signal.phase_seconds", 5400 * time.Second},
		{partial, "expected_signal.phase_seconds", 60 * time.Second},
		{partial, "expected_signal.wave_seconds", 57600 * time.Second},
		{&ExpectedSignalPolicy{WaveSeconds: 5}, "expected_signal.wave_seconds", 5 * time.Second},
	}
	for _, c := range cases {
		if got, ok := c.p.Deadline(c.key); !ok || got != c.want {
			t.Errorf("%+v %s = %v %v, want %v", c.p, c.key, got, ok, c.want)
		}
	}
	if _, ok := none.Deadline("expected_signal.ship_seconds"); ok {
		t.Error("the ship key is gone")
	}
	if _, ok := none.Deadline("nope"); ok {
		t.Error("an unknown key has no deadline")
	}
}
