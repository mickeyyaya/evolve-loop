package triagecap

import (
	"reflect"
	"testing"
)

// R9.3: the deferred/dropped floor vocabulary — packages whose coverage
// floors triage explicitly pushed OUT of this cycle. TDD predicates binding
// these floors is the cycle-280 failure mode (builder starved the committed
// task while clearing deferred-task gates).

func TestDeferredFloorPackages_Cycle281Replay(t *testing.T) {
	// cycle-281 deferred floor items name cmd/evolve ("evolve" is the package
	// basename); the bridge item's only package reference is inside hyphenated
	// slug compounds, which are single tokens — not mentions.
	pkgs := append([]string{"evolve"}, knownPkgsFixture...)
	got := DeferredFloorPackages(readFixture(t, "triage-cycle281.md"), pkgs)
	want := []string{"evolve"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeferredFloorPackages(cycle-281) = %v, want %v", got, want)
	}
}

func TestDeferredFloorPackages_Table(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		want     []string
	}{
		{
			name:     "no deferred section",
			artifact: "## top_n\n- coverage-x: bridge coverage ≥98%\n",
			want:     nil,
		},
		{
			name: "deferred floor item names packages",
			artifact: "## top_n\n- coverage-x: bridge coverage ≥98%\n\n" +
				"## deferred (carry to NEXT cycle's carryoverTodos)\n" +
				"- coverage-rest: recovery, interaction coverage to ≥98%\n",
			want: []string{"interaction", "recovery"},
		},
		{
			name: "dropped floor item counts too",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## dropped (rejected with reason)\n" +
				"- coverage-no: evalgate to 95% coverage — reason=descoped\n",
			want: []string{"evalgate"},
		},
		{
			name: "non-floor deferred items contribute nothing",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n- refactor-later: tidy the recovery package\n",
			want: nil,
		},
		{
			// Pins the metadata-strip semantics on deferred items: the
			// contract fields' own vocabulary (evidence/scout) never counts,
			// the first word of defer_reason= is consumed with the key, and
			// later free-form words naming a package DO count (a reason
			// naming bridge is about bridge).
			name: "deferred metadata stripped, free-form defer_reason tail still matches",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n" +
				"- coverage-rest: push swarm coverage to ≥98% — priority=M, evidence=scout-report.md#x, defer_reason=budget consumed by bridge work, source=scout\n",
			want: []string{"bridge", "swarm"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeferredFloorPackages(tt.artifact, knownPkgsFixture)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeferredFloorPackages = %v, want %v", got, tt.want)
			}
		})
	}
}
