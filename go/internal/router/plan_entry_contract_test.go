package router

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestPhasePlanEntry_JSONKeysAreTheAdvisorPlanContractKeys(t *testing.T) {
	var keys []string
	entry := reflect.TypeOf(PhasePlanEntry{})
	for i := range entry.NumField() {
		keys = append(keys, strings.Split(entry.Field(i).Tag.Get("json"), ",")[0])
	}
	for _, seat := range []string{phasecontract.FlowSeatPlan, phasecontract.FlowSeatRePlan} {
		allowed := slices.Clone(phasecontract.MustFlowDecision(seat).Allowed)
		slices.Sort(allowed)
		got := slices.Sorted(slices.Values(keys))
		if !slices.Equal(got, allowed) {
			t.Errorf("%s: the PhasePlanEntry keys %v differ from the contract keys %v: change both in one commit", seat, got, allowed)
		}
	}
}
