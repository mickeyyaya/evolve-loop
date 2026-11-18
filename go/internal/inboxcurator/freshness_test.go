package inboxcurator

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func verdictItem(reason string, files ...string) inboxbatch.Item {
	raw, _ := json.Marshal(reason)
	return inboxbatch.Item{ID: "v", Files: files, RoutedReason: raw}
}

func TestFreshness_StaleNamesAnAbsentPremiseAndAnAlreadyShippedCommitOnlyWhileTheTreeProvesIt(t *testing.T) {
	absent := func(string) bool { return false }
	present := func(p string) bool { return p == "go/internal/ship/main_red_ci.go" }
	onMain := func(sha string, paths []string) bool { return sha == "c6462ce03" && len(paths) == 1 }

	cases := map[string]struct {
		f     Freshness
		it    inboxbatch.Item
		stale bool
	}{
		"an absent directory premise":        {Freshness{PathExists: absent}, verdictItem("lane triage (cycle 9) dropped: stale: go/internal/gaterun/ absent at HEAD; only on branches", "go/internal/gaterun/"), true},
		"a premise that landed since":        {Freshness{PathExists: func(string) bool { return true }}, verdictItem("dropped: stale: go/internal/gaterun/ absent at HEAD", "go/internal/gaterun/"), false},
		"basenames resolved by declared":     {Freshness{PathExists: absent}, verdictItem("dropped: stale: gate_step_runner.go and main_red_ci.go absent at HEAD; PR open", "go/internal/ship/gate_step_runner.go", "go/internal/ship/main_red_ci.go"), true},
		"one named file present":             {Freshness{PathExists: present}, verdictItem("dropped: stale: gate_step_runner.go and main_red_ci.go absent at HEAD", "go/internal/ship/gate_step_runner.go", "go/internal/ship/main_red_ci.go"), false},
		"a token that no declared path owns": {Freshness{PathExists: absent}, verdictItem("dropped: stale: nothing absent at HEAD", "go/a.go"), false},
		"an undeclared path token":           {Freshness{PathExists: absent}, verdictItem("dropped: stale: go/internal/other/x.go absent at HEAD", "go/a.go"), true},
		"a shipped commit on main":           {Freshness{OnMain: onMain}, verdictItem("dropped: already-shipped: c6462ce03", "go/a.go"), true},
		"a shipped commit off main":          {Freshness{OnMain: func(string, []string) bool { return false }}, verdictItem("dropped: already-shipped: c6462ce03", "go/a.go"), false},
		"no verdict":                         {Freshness{PathExists: absent, OnMain: onMain}, verdictItem("dropped: protected-surface: go/a.go", "go/a.go"), false},
		"no reason":                          {Freshness{PathExists: absent, OnMain: onMain}, inboxbatch.Item{ID: "x"}, false},
	}
	for name, tc := range cases {
		if got := tc.f.Stale(tc.it) != ""; got != tc.stale {
			t.Errorf("%s: stale = %v, want %v (%q)", name, got, tc.stale, tc.f.Stale(tc.it))
		}
	}
}
