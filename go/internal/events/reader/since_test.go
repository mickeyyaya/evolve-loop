package reader

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
)

func TestParseSince_ReadsEachForm(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 9, 17, 46, 2, 114000000, time.UTC)
	tests := []struct {
		in   string
		want Since
	}{
		{"", Since{mode: sinceNew}},
		{"new", Since{mode: sinceNew}},
		{"all", Since{mode: sinceAll}},
		{"last", Since{mode: sinceLast}},
		{"2026-10-09T17:46:02.114Z", Since{mode: sinceTime, at: at}},
		{"loop:0", Since{mode: sinceCursors, cursors: map[string]int64{"loop": 0}}},
		{"loop:184467,ci.required:12", Since{mode: sinceCursors, cursors: map[string]int64{"loop": 184467, "ci.required": 12}}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseSince(tc.in)
			if err != nil || !reflect.DeepEqual(got, tc.want) || !got.at.Equal(tc.want.at) {
				t.Errorf("ParseSince(%q) = %+v, %v, want %+v", tc.in, got, err, tc.want)
			}
		})
	}
}

func TestParseSince_RefusesAMalformedValueAsAUsageError(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"loop", "loop:", "loop:-1", "loop:x", "Loop:1", "loop:1,loop:2", "loop:1,", "2026-10-09"} {
		t.Run(in, func(t *testing.T) {
			got, err := ParseSince(in)
			if !errors.Is(err, filter.ErrUsage) || !reflect.DeepEqual(got, Since{}) {
				t.Errorf("ParseSince(%q) = %+v, %v, want a usage error", in, got, err)
			}
		})
	}
}
