package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestBoundedTail(t *testing.T) {
	for name, tc := range map[string]struct {
		tail, want string
		maxBytes   int
	}{
		"short tail passes through":      {"ok\nFAIL x", "ok\nFAIL x", 512},
		"long tail keeps its end":        {strings.Repeat("a", 600) + "END", "…" + strings.Repeat("a", 508) + "END", 4096},
		"escaped runes count six bytes":  {"head <<<<END", "…<END", 12},
		"invalid utf-8 is made valid":    {"\xffEND", "�END", 512},
		"byte budget binds before runes": {strings.Repeat("b", 100) + "END", "…" + strings.Repeat("b", 7) + "END", 13},
	} {
		t.Run(name, func(t *testing.T) {
			got := boundedTail(tc.tail, tc.maxBytes)
			if got != tc.want {
				t.Errorf("boundedTail(%q, %d) = %q, want %q", tc.tail, tc.maxBytes, got, tc.want)
			}
			if n := utf8.RuneCountInString(got); n > signalFieldRunes {
				t.Errorf("boundedTail kept %d runes, over the %d-rune field bound", n, signalFieldRunes)
			}
		})
	}
}

func TestCompositionCarryForward_HTMLEscapedTailsOfAllFourGatesFitOneEventLine(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	outcomes := map[string]ciparity.GateOutcome{}
	for _, gate := range ciparity.RequiredComposedGates {
		lines := make([]string, 0, 20)
		for i := 0; i < 19; i++ {
			lines = append(lines, strings.Repeat("<&>", 40))
		}
		outcomes[gate] = ciparity.GateOutcome{Status: "fail", Tail: strings.Join(append(lines, fmt.Sprintf("%s_LAST <&>", gate)), "\n")}
	}
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	o := declineOrchestrator(t, center, diff, patchID, outcomes)

	if o.compositionCarryForward(context.Background(), 9, CycleState{ActiveWorktree: worktree, RunID: "run-escaped"}, "") {
		t.Fatal("red composed gates must not carry forward")
	}

	if len(events) != 1 {
		t.Fatalf("events = %+v, want exactly one decline event", events)
	}
	if n, dropped := events[0].Fields["truncated"]; dropped {
		t.Errorf("the Signal Center dropped %s tail field(s) to fit the line; every failing gate's tail must survive", n)
	}
	for _, gate := range ciparity.RequiredComposedGates {
		if !strings.HasSuffix(events[0].Fields["tail_"+gate], gate+"_LAST <&>") {
			t.Errorf("fields[tail_%s] = %q, want it to end with the gate's last line", gate, events[0].Fields["tail_"+gate])
		}
	}
}
