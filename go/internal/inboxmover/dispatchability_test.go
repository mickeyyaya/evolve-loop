package inboxmover

import (
	"path/filepath"
	"testing"
)

func dispatchabilityFixture(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "dep-pending", nil)
	writeTask(t, filepath.Join(inbox, "processing", "cycle-9"), "dep-processing", nil)
	writeTask(t, filepath.Join(inbox, "retry"), "dep-retry", nil)
	writeTask(t, filepath.Join(inbox, "processed", "cycle-9"), "dep-done", nil)
	writeTask(t, inbox, "blocked-by-pending", []string{"dep-pending"})
	writeTask(t, inbox, "blocked-by-processing", []string{"dep-processing"})
	writeTask(t, inbox, "blocked-by-retry", []string{"dep-retry"})
	writeTask(t, inbox, "ready", []string{"dep-done"})
	writeTask(t, inbox, "ready-on-unfiled-dep", []string{"never-filed"})
	writeTask(t, filepath.Join(inbox, "rejected", "cycle-9"), "rejected-item", nil)
	writeTask(t, filepath.Join(inbox, "quarantine"), "quarantined-item", nil)
	writeTask(t, filepath.Join(inbox, "consumed"), "landed-item", nil)
	return Options{ProjectRoot: root}
}

func TestResolveDispatchability_IsTheOneRuleForWhetherALaneMayTakeAnID(t *testing.T) {
	opts := dispatchabilityFixture(t)
	cases := map[string]Dispatchability{
		"ready":                 {Dispatchable: true},
		"ready-on-unfiled-dep":  {Dispatchable: true},
		"never-filed":           {Dispatchable: true},
		"blocked-by-pending":    {Reason: "deps unmet: needs dep-pending"},
		"blocked-by-processing": {Reason: "deps unmet: needs dep-processing"},
		"blocked-by-retry":      {Reason: "deps unmet: needs dep-retry"},
		"dep-processing":        {Reason: "consumed: processing cycle-9"},
		"dep-retry":             {Reason: "consumed: retry"},
		"dep-done":              {Reason: "consumed: processed cycle-9"},
		"rejected-item":         {Reason: "consumed: rejected cycle-9"},
		"quarantined-item":      {Reason: "consumed: quarantine"},
		"landed-item":           {Reason: "consumed: consumed"},
	}
	for id, want := range cases {
		if got := ResolveDispatchability(opts, id); got != want {
			t.Errorf("ResolveDispatchability(%q) = %+v, want %+v", id, got, want)
		}
	}
}

func TestPendingDispatchability_NamesTheFirstUnmetDependency(t *testing.T) {
	opts := dispatchabilityFixture(t)
	cases := []struct {
		deps []string
		want Dispatchability
	}{
		{nil, Dispatchability{Dispatchable: true}},
		{[]string{"dep-done", "never-filed"}, Dispatchability{Dispatchable: true}},
		{[]string{"dep-done", "dep-retry", "dep-pending"}, Dispatchability{Reason: "deps unmet: needs dep-retry"}},
	}
	for _, c := range cases {
		if got := PendingDispatchability(opts, c.deps); got != c.want {
			t.Errorf("PendingDispatchability(%v) = %+v, want %+v", c.deps, got, c.want)
		}
	}
}
