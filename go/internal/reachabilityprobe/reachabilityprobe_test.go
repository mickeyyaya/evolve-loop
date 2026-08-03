package reachabilityprobe

// reachabilityprobe_test.go — table-driven coverage for CheckCallSite,
// closing the inbox tdd-structural-test-reachability-probe (weight 0.92)
// acceptance criterion 3: "a reachable pinned call site passes the check
// unchanged (no false positives - table-driven)". The prior coverage in
// apicover_named_test.go exercised exactly one 2-node cycle and one acyclic
// case as hardcoded assertions, not a table; this file adds the
// multi-hop/edge shapes the package's own doc comment (cycle-644) promises.

import (
	"strings"
	"testing"
)

func TestCheckCallSite(t *testing.T) {
	cases := []struct {
		name      string
		graph     ImportGraph
		site      CallSite
		wantNil   bool
		cycleHead string // required first element of Violation.Cycle when wantNil is false
	}{
		{
			// cycle-644 shape, re-asserted here in table form per the AC.
			name: "direct_2node_cycle",
			graph: ImportGraph{
				"storage": {"core"},
				"core":    {},
			},
			site:      CallSite{PinningPackage: "core", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
			wantNil:   false,
			cycleHead: "storage",
		},
		{
			name: "transitive_3hop_cycle",
			graph: ImportGraph{
				"storage": {"mid"},
				"mid":     {"core"},
				"core":    {},
			},
			site:      CallSite{PinningPackage: "core", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
			wantNil:   false,
			cycleHead: "storage",
		},
		{
			// positive "safe to freeze" case: PinningPackage is present in the
			// graph but ReferencedPackage never reaches it.
			name: "acyclic_reachable_pin_no_false_positive",
			graph: ImportGraph{
				"storage": {"core"},
				"core":    {},
				"leaf":    {},
			},
			site:    CallSite{PinningPackage: "leaf", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
			wantNil: true,
		},
		{
			// PinningPackage absent from the graph entirely: absence of
			// evidence is not evidence of a cycle.
			name: "pinning_package_absent_from_graph",
			graph: ImportGraph{
				"storage": {"core"},
				"core":    {},
			},
			site:    CallSite{PinningPackage: "nowhere", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
			wantNil: true,
		},
		{
			// self-referential pin: a package "importing itself" is a trivial
			// non-violation (the Go compiler already forbids self-import
			// earlier in the pipeline).
			name: "self_referential_pin",
			graph: ImportGraph{
				"core": {},
			},
			site:      CallSite{PinningPackage: "core", ReferencedPackage: "core", Symbol: "Foo"},
			wantNil:   false,
			cycleHead: "core",
		},
		{
			// disjoint graph: two components sharing no edges must never be
			// treated as a cycle.
			name: "disjoint_components",
			graph: ImportGraph{
				"storage": {"core"},
				"core":    {},
				"other":   {"leaf"},
				"leaf":    {},
			},
			site:    CallSite{PinningPackage: "other", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
			wantNil: true,
		},
		{
			// empty graph: PinningPackage cannot be "known" in an empty graph.
			name:    "empty_graph",
			graph:   ImportGraph{},
			site:    CallSite{PinningPackage: "core", ReferencedPackage: "storage", Symbol: "UpdateStateMap"},
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckCallSite(tc.graph, tc.site)

			if tc.wantNil {
				if got != nil {
					t.Fatalf("CheckCallSite(%+v, %+v) = %+v, want nil", tc.graph, tc.site, got)
				}
				return
			}

			if got == nil {
				t.Fatalf("CheckCallSite(%+v, %+v) = nil, want a *Violation", tc.graph, tc.site)
			}
			if got.Site != tc.site {
				t.Errorf("Violation.Site = %+v, want %+v", got.Site, tc.site)
			}
			if len(got.Cycle) == 0 {
				t.Fatal("Violation.Cycle is empty, want a non-empty import chain")
			}
			if got.Cycle[0] != tc.cycleHead {
				t.Errorf("Violation.Cycle[0] = %q, want %q (chain must start at ReferencedPackage): %v",
					got.Cycle[0], tc.cycleHead, got.Cycle)
			}
			if !strings.Contains(got.Error(), tc.site.PinningPackage) {
				t.Errorf("Violation.Error() = %q, want it to mention PinningPackage %q", got.Error(), tc.site.PinningPackage)
			}
		})
	}
}
