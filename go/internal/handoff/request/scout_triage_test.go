package request_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
)

func selectionAsk(slug string) handoff.Ask {
	return handoff.Ask{ID: slug, Text: slug, Source: "scout:selected-tasks", EvidenceKind: "disposition", Required: true}
}

func TestScoutTriage_Table(t *testing.T) {
	cases := []struct {
		name  string
		slugs []string
		want  []handoff.Ask
	}{
		{name: "no selection gives no asks", slugs: nil, want: nil},
		{
			name:  "each selected slug is one ask in selection order",
			slugs: []string{"handoff-h2-request-builders", "alpha"},
			want:  []handoff.Ask{selectionAsk("handoff-h2-request-builders"), selectionAsk("alpha")},
		},
		{
			name:  "blank slugs are dropped and padding is trimmed",
			slugs: []string{"  alpha ", "", "   "},
			want:  []handoff.Ask{selectionAsk("alpha")},
		},
		{
			name:  "a repeated slug is asked once",
			slugs: []string{"alpha", "beta", "alpha"},
			want:  []handoff.Ask{selectionAsk("alpha"), selectionAsk("beta")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := request.ScoutTriage(tc.slugs)
			if got.Pair != "scout-triage" {
				t.Errorf("pair = %q, want scout-triage", got.Pair)
			}
			if !reflect.DeepEqual(got.Asks, tc.want) {
				t.Errorf("asks mismatch\n got: %+v\nwant: %+v", got.Asks, tc.want)
			}
		})
	}
}

func TestPairNames_MatchTheSpecTable(t *testing.T) {
	got := []string{request.PairScoutTriage, request.PairTriageTDD, request.PairTDDBuild}
	want := []string{"scout-triage", "triage-tdd", "tdd-build"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pair names = %v, want %v", got, want)
	}
}
