package policy

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
)

func TestAttemptPostmortemConfig_AnAbsentBlockIsTheCompiledDefault(t *testing.T) {
	p, err := Parse([]byte(`{}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := p.AttemptPostmortemConfig(); got != attemptpostmortem.DefaultConfig() {
		t.Fatalf("AttemptPostmortemConfig() = %+v, want the compiled default", got)
	}
}

func TestAttemptPostmortemConfig_EachKeyOverridesItsCap(t *testing.T) {
	p, err := Parse([]byte(`{"attempt_postmortem":{"max_commands":5,"max_command_runes":77,"max_pane_tail_runes":901,"max_delta_runes":333,"suspect_window_s":45,"max_records":7}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := attemptpostmortem.Config{MaxCommands: 5, MaxCommandRunes: 77, MaxPaneTailRunes: 901, MaxDeltaRunes: 333, SuspectWindow: 45 * time.Second, MaxRecords: 7}
	if got := p.AttemptPostmortemConfig(); got != want {
		t.Fatalf("AttemptPostmortemConfig() = %+v, want %+v", got, want)
	}
	var block *AttemptPostmortemPolicy = p.AttemptPostmortem
	if block == nil || block.MaxRecords == nil || *block.MaxRecords != 7 || block.SuspectWindowS == nil || *block.SuspectWindowS != 45 {
		t.Fatalf("AttemptPostmortemPolicy = %+v, want the decoded keys", block)
	}
	partial, err := Parse([]byte(`{"attempt_postmortem":{"max_records":2}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	wantPartial := attemptpostmortem.DefaultConfig()
	wantPartial.MaxRecords = 2
	if got := partial.AttemptPostmortemConfig(); got != wantPartial {
		t.Fatalf("a partial block = %+v, want the defaults with max_records 2", got)
	}
}

func TestAttemptPostmortemPolicy_RefusesAnUnknownKeyAndANonPositiveCap(t *testing.T) {
	cases := map[string]string{
		"unknown key":   `{"attempt_postmortem":{"max_comands":5}}`,
		"zero cap":      `{"attempt_postmortem":{"max_records":0}}`,
		"negative cap":  `{"attempt_postmortem":{"suspect_window_s":-1}}`,
		"wrong type":    `{"attempt_postmortem":{"max_commands":"8"}}`,
		"not an object": `{"attempt_postmortem":[]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), "attempt_postmortem") {
				t.Fatalf("Parse(%s) = %v, want an attempt_postmortem error", body, err)
			}
		})
	}
}
