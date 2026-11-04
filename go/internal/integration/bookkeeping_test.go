package integration

import "testing"

func TestSkipsL2_OnlyABatchWithNoCodeSkips(t *testing.T) {
	t.Parallel()
	if !SkipsL2(false) {
		t.Error("a bookkeeping batch must skip L2")
	}
	if SkipsL2(true) {
		t.Error("a batch that holds code must not skip L2")
	}
}

func TestFlushDue_FollowsEveryCodeFlushAndNeedsPendingEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		pending int
		trigger FlushTrigger
		want    bool
	}{
		{"nothing pending", 0, TriggerWaveEnd, false},
		{"no code flush", 2, TriggerNone, false},
		{"wave end", 1, TriggerWaveEnd, true},
		{"cap reached", 1, TriggerCapReached, true},
		{"risky alone", 3, TriggerRiskyAlone, true},
	}
	for _, c := range cases {
		if got := FlushDue(c.pending, c.trigger); got != c.want {
			t.Errorf("%s: FlushDue = %v, want %v", c.name, got, c.want)
		}
	}
}
