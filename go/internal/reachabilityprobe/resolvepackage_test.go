package reachabilityprobe

// resolvepackage_test.go covers resolvePackage's BASE-NAME FALLBACK — the branch
// every frozen pin written the ordinary way (`storage.UpdateStateMap(`, no
// import alias) flows through, and the one branch of the resolver that had zero
// tests before cycle-1248. The alias branch is covered from the outside in
// frozenpins_test.go; this file drives the resolver directly because the
// interesting inputs are graph SHAPES (two packages sharing a base name, two
// equidistant candidates) that would take a whole throwaway module each to
// stage through CheckFrozenPins.
//
// ImportGraph is a plain map[string][]string, so a graph literal here is the
// real input type `go list` produces — no fake, no seam.

import "testing"

// Fixture import paths. `storage` deliberately appears TWICE at different
// depths, which is the ambiguity resolvePackage exists to break.
const (
	resolveModule    = "example.com/fixture"
	corePkg          = resolveModule + "/internal/core"
	nearStoragePkg   = resolveModule + "/internal/storage"
	farStoragePkg    = resolveModule + "/vendorish/storage"
	leafutilPkg      = resolveModule + "/internal/leafutil"
	siblingAlphaPkg  = resolveModule + "/internal/alpha/shared"
	siblingBravoPkg  = resolveModule + "/internal/bravo/shared"
	outsideModulePkg = "third.party/elsewhere/storage"
)

// resolveGraph is the shared fixture graph. Edges are irrelevant to
// resolvePackage (it maps identifier -> path; CheckCallSite walks the edges), so
// they stay empty except where a reader would expect the cycle-644 shape.
func resolveGraph() ImportGraph {
	return ImportGraph{
		corePkg:         nil,
		nearStoragePkg:  {corePkg},
		farStoragePkg:   nil,
		leafutilPkg:     nil,
		siblingAlphaPkg: nil,
		siblingBravoPkg: nil,
	}
}

// TestResolvePackage_BaseNameFallback is the table for the unaliased path: an
// identifier is matched against every graph package's base name, and among the
// matches the NEAREST NEIGHBOUR — longest shared prefix with the pinning package
// — wins.
func TestResolvePackage_BaseNameFallback(t *testing.T) {
	graph := resolveGraph()

	for _, tc := range []struct {
		name    string
		ident   string
		pinning string
		want    string
		wantOK  bool
	}{
		{
			// The headline ambiguity: `storage` matches both internal/storage
			// and vendorish/storage. Pinned inside internal/core, the nearest
			// neighbour (shared prefix ".../internal/") must win — picking the
			// far one would report a cycle through the wrong package.
			name:    "multi_candidate_nearest_prefix_wins",
			ident:   "storage",
			pinning: corePkg,
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			// Same graph, same identifier, different pinning package: the
			// answer must MOVE with the neighbourhood. Without this the test
			// above is also satisfied by a resolver hard-coded to prefer
			// "internal/".
			name:    "multi_candidate_nearest_follows_pinning_package",
			ident:   "storage",
			pinning: resolveModule + "/vendorish/consumer",
			want:    farStoragePkg,
			wantOK:  true,
		},
		{
			name:    "single_candidate_resolves",
			ident:   "leafutil",
			pinning: corePkg,
			want:    leafutilPkg,
			wantOK:  true,
		},
		{
			// An identifier that IS a full import path present in the graph
			// short-circuits ahead of base-name matching.
			name:    "exact_import_path_short_circuits",
			ident:   nearStoragePkg,
			pinning: corePkg,
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			// NEGATIVE axis: nothing in the graph is named `nosuchpkg`, so the
			// resolver must return nothing rather than a best guess. This is
			// what makes CheckFrozenPins fail OPEN on an unknown identifier
			// instead of inventing a violation.
			name:    "no_candidate_fails_open",
			ident:   "nosuchpkg",
			pinning: corePkg,
			wantOK:  false,
		},
		{
			// Base-name matching is exact, not substring: `stor` must not
			// latch onto `storage`.
			name:    "partial_base_name_is_not_a_match_fails_open",
			ident:   "stor",
			pinning: corePkg,
			wantOK:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolvePackage(graph, tc.ident, tc.pinning, nil)
			if ok != tc.wantOK {
				t.Fatalf("resolvePackage(%q, pinning=%q) ok = %v, want %v (got %q)",
					tc.ident, tc.pinning, ok, tc.wantOK, got)
			}
			if got != tc.want {
				t.Errorf("resolvePackage(%q, pinning=%q) = %q, want %q",
					tc.ident, tc.pinning, got, tc.want)
			}
		})
	}
}

// TestResolvePackage_DeterministicLexicalTieBreak pins the determinism half.
// Go randomises map iteration order, so two candidates with an EQUAL prefix
// score are precisely where a nondeterministic verdict would hide: the gate
// would blame one package on one run and the other on the next. The tie must
// break lexically, and it must hold across repeated resolutions of the same
// graph.
func TestResolvePackage_DeterministicLexicalTieBreak(t *testing.T) {
	graph := resolveGraph()

	// Both candidates share exactly "example.com/fixture/internal/" with the
	// pinning package and neither shares more, so the scores tie.
	const pinning = resolveModule + "/internal/consumer"
	for i := 0; i < 50; i++ {
		got, ok := resolvePackage(graph, "shared", pinning, nil)
		if !ok {
			t.Fatalf("iteration %d: resolvePackage(\"shared\") did not resolve", i)
		}
		if got != siblingAlphaPkg {
			t.Fatalf("iteration %d: resolvePackage(\"shared\") = %q, want the"+
				" lexically smallest tied candidate %q (the other tied candidate"+
				" is %q); map iteration order must not change the verdict",
				i, got, siblingAlphaPkg, siblingBravoPkg)
		}
	}
}

// TestResolvePackage_AliasNeverSuppressesBaseNameMatch is the cycle-1248
// regression guard for the audit defect: resolvePackage consulted the frozen
// test file's alias map FIRST and unconditionally.
//
// The alias map comes from a test file, but the identifier is compiled in the
// PRODUCTION file's scope — so the alias is a hint, not an authority. Consulted
// first it could SUPPRESS: one import line in an agent-authored frozen test file
// rebinding `storage` to a benign package redirected a genuine cycle-644 pin
// away from the real internal/storage, and an alias pointing outside the module
// graph vetoed resolution outright. Both made the gate blind to the shape it
// exists to catch. Consulted last, the alias can only ADD reach.
func TestResolvePackage_AliasNeverSuppressesBaseNameMatch(t *testing.T) {
	graph := resolveGraph()

	for _, tc := range []struct {
		name    string
		ident   string
		aliases map[string]string
		want    string
		wantOK  bool
	}{
		{
			// Rebinding to a benign in-graph package must NOT redirect the pin.
			name:    "rebinding_alias_does_not_redirect_real_package",
			ident:   "storage",
			aliases: map[string]string{"storage": leafutilPkg},
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			// Rebinding to a package outside the graph must NOT veto the match.
			name:    "out_of_graph_alias_does_not_veto_real_package",
			ident:   "storage",
			aliases: map[string]string{"storage": outsideModulePkg},
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			// The alias still earns its keep where base names cannot reach:
			// `st` matches no package's base name, so without the alias this
			// pin fails open and the cycle goes unreported.
			name:    "alias_still_resolves_identifier_no_base_name_matches",
			ident:   "st",
			aliases: map[string]string{"st": nearStoragePkg},
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			// An alias binding a package absent from the graph proves nothing,
			// so it is the same fail-open verdict as no alias at all.
			name:    "alias_outside_graph_is_unprovable_and_fails_open",
			ident:   "st",
			aliases: map[string]string{"st": outsideModulePkg},
			wantOK:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolvePackage(graph, tc.ident, corePkg, tc.aliases)
			if ok != tc.wantOK {
				t.Fatalf("resolvePackage(%q, aliases=%v) ok = %v, want %v (got %q)",
					tc.ident, tc.aliases, ok, tc.wantOK, got)
			}
			if got != tc.want {
				t.Errorf("resolvePackage(%q, aliases=%v) = %q, want %q",
					tc.ident, tc.aliases, got, tc.want)
			}
		})
	}
}
