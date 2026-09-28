package config

import "testing"

func TestRoutingBlock_ATriggerWithoutAHintIsTheWholeAdmissionRule(t *testing.T) {
	trigger := []Condition{{Field: "scout.goal_type", Op: "==", Value: "bugfix"}}
	for _, tc := range []struct {
		name  string
		block RoutingBlock
		want  bool
	}{
		{"a trigger alone", RoutingBlock{InsertWhen: trigger}, true},
		{"a trigger beside a hint", RoutingBlock{InsertWhen: trigger, RubricHint: []string{"also warrants it"}}, false},
		{"a hint alone", RoutingBlock{RubricHint: []string{"also warrants it"}}, false},
		{"a skip rule alone", RoutingBlock{SkipWhen: trigger}, false},
		{"no rule", RoutingBlock{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.block.TriggerIsTheWholeRule(); got != tc.want {
				t.Errorf("TriggerIsTheWholeRule() = %v, want %v", got, tc.want)
			}
		})
	}
}
