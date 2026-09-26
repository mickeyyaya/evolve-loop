package reachabilityprobe

import "testing"

// storage appears twice, at different depths, so resolvePackage must pick the nearer one.
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

// Edges do not affect resolvePackage; the storage -> core edge only mirrors the real cycle.
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
			name:    "multi_candidate_nearest_prefix_wins",
			ident:   "storage",
			pinning: corePkg,
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
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
			name:    "exact_import_path_short_circuits",
			ident:   nearStoragePkg,
			pinning: corePkg,
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			name:    "no_candidate_fails_open",
			ident:   "nosuchpkg",
			pinning: corePkg,
			wantOK:  false,
		},
		{
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

func TestResolvePackage_DeterministicLexicalTieBreak(t *testing.T) {
	graph := resolveGraph()

	// Both shared candidates tie on prefix with pinning; repeating the call exercises
	// Go's randomized map iteration order.
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
			name:    "rebinding_alias_does_not_redirect_real_package",
			ident:   "storage",
			aliases: map[string]string{"storage": leafutilPkg},
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			name:    "out_of_graph_alias_does_not_veto_real_package",
			ident:   "storage",
			aliases: map[string]string{"storage": outsideModulePkg},
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
			name:    "alias_still_resolves_identifier_no_base_name_matches",
			ident:   "st",
			aliases: map[string]string{"st": nearStoragePkg},
			want:    nearStoragePkg,
			wantOK:  true,
		},
		{
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
