package fleet

import (
	"maps"
	"strings"
	"testing"
)

var deepAndTop = [][]string{{"agy", "agy-claude", "claude"}, {"agy-claude", "claude"}, {"agy-claude", "claude"}}

func TestQuotaAwareCount_ABenchWithAFallbackKeepsTheLane(t *testing.T) {
	t.Parallel()
	var log strings.Builder
	benched := map[string]string{"agy-claude": "rate_limit"}

	got := QuotaAwareCount(3, ChainAwareBenches(benched, [][]string{{"agy-claude", "claude"}}, &log), 2, &log)

	if got != 3 {
		t.Fatalf("count = %d, want 3: claude follows agy-claude in the chain, so the bench removes no capacity", got)
	}
	want := `[loop] INFO: fleet: quota bench on CLI family "agy-claude" (rate_limit) costs no lane: a lane walks to "claude"`
	if !strings.Contains(log.String(), want) || strings.Contains(log.String(), "WARN") {
		t.Errorf("log = %q, want only the INFO line that names the fallback", log.String())
	}
}

func TestQuotaAwareCount_ABenchThatEmptiesAChainCostsOneLane(t *testing.T) {
	t.Parallel()
	var log strings.Builder
	benched := map[string]string{"agy-claude": "rate_limit", "claude": "quota_exhausted"}

	costed := ChainAwareBenches(benched, [][]string{{"agy-claude", "claude"}}, &log)
	got := QuotaAwareCount(3, costed, 2, &log)

	if got != 2 {
		t.Fatalf("count = %d, want 2: the chain has no healthy family, so it costs one lane", got)
	}
	if !maps.Equal(costed, map[string]string{"claude": "quota_exhausted"}) {
		t.Errorf("costed = %v, want the last family of the empty chain only", costed)
	}
	if !strings.Contains(log.String(), `WARN: fleet: quota bench on CLI family "claude" (quota_exhausted): wave count 3 -> 2 (min 2)`) {
		t.Errorf("log = %q, want the WARN line for the costed bench", log.String())
	}
	if got := QuotaAwareCount(2, costed, 2, &log); got != 2 {
		t.Errorf("count = %d, want 2: the min_lanes floor holds", got)
	}
}

func TestChainAwareBenches_AFamilyInNoChainCostsNothing(t *testing.T) {
	t.Parallel()
	var log strings.Builder

	costed := ChainAwareBenches(map[string]string{"codex": "rate_limit"}, deepAndTop, &log)

	if len(costed) != 0 {
		t.Fatalf("costed = %v, want none: no chain of the wave uses codex", costed)
	}
	want := `[loop] INFO: fleet: quota bench on CLI family "codex" (rate_limit) costs no lane: no lane chain uses it`
	if !strings.Contains(log.String(), want) {
		t.Errorf("log = %q, want %q", log.String(), want)
	}
}

func TestChainAwareBenches_Edges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		benched map[string]string
		chains  [][]string
		want    map[string]string
		info    []string
	}{
		{
			name:    "no benches is nil",
			benched: nil,
			chains:  deepAndTop,
			want:    nil,
		},
		{
			name:    "unknown chains keep every bench",
			benched: map[string]string{"agy-claude": "rate_limit", "codex": "quota"},
			chains:  nil,
			want:    map[string]string{"agy-claude": "rate_limit", "codex": "quota"},
		},
		{
			name:    "the production benches of 2026-10-17 cost nothing",
			benched: map[string]string{"agy": "rate_limit", "agy-claude": "rate_limit"},
			chains:  deepAndTop,
			want:    map[string]string{},
			info:    []string{`"agy" (rate_limit) costs no lane: a lane walks to "claude"`, `"agy-claude" (rate_limit) costs no lane: a lane walks to "claude"`},
		},
		{
			name:    "two empty chains with one last family cost one lane",
			benched: map[string]string{"agy": "a", "agy-claude": "b", "claude": "c"},
			chains:  deepAndTop,
			want:    map[string]string{"claude": "c"},
			info:    []string{`"agy" (a) costs no lane: its chain is counted under "claude"`, `"agy-claude" (b) costs no lane: its chain is counted under "claude"`},
		},
		{
			name:    "two empty chains with two last families cost two lanes",
			benched: map[string]string{"codex": "a", "claude": "b"},
			chains:  [][]string{{"codex"}, {"claude"}, {"agy", "claude"}},
			want:    map[string]string{"codex": "a", "claude": "b"},
		},
		{
			name:    "a scoped bench matches by its family",
			benched: map[string]string{"claude@opus": "rate_limit"},
			chains:  [][]string{{"claude"}},
			want:    map[string]string{"claude@opus": "rate_limit"},
		},
		{
			name:    "an empty chain is skipped",
			benched: map[string]string{"claude": "rate_limit"},
			chains:  [][]string{{}, {"claude", "agy"}},
			want:    map[string]string{},
			info:    []string{`"claude" (rate_limit) costs no lane: a lane walks to "agy"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var log strings.Builder

			got := ChainAwareBenches(tc.benched, tc.chains, &log)

			if !maps.Equal(got, tc.want) || (got == nil) != (tc.want == nil) {
				t.Errorf("ChainAwareBenches = %v, want %v", got, tc.want)
			}
			for _, line := range tc.info {
				if !strings.Contains(log.String(), line) {
					t.Errorf("log = %q, want %q", log.String(), line)
				}
			}
			if len(tc.info) == 0 && log.Len() != 0 {
				t.Errorf("log = %q, want silence", log.String())
			}
		})
	}
}
