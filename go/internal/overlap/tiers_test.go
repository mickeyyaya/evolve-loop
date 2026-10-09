package overlap

import "testing"

func TestProof_DisjointPathsAndClosuresAreT1(t *testing.T) {
	got := Prove(baseInput([]string{src("b")}, []string{src("a")}))

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "shared_paths", got.Evidence.SharedPaths)
	assertEdges(t, "edges_lane_to_peer", got.Evidence.EdgesLaneToPeer)
	assertEdges(t, "edges_peer_to_lane", got.Evidence.EdgesPeerToLane)
	assertStrings(t, "unknown", got.Evidence.Unknown)
	assertStrings(t, "lane packages", got.Selection.LanePackages, ip("b"))
	assertStrings(t, "peer packages", got.Selection.PeerPackages, ip("a"))
}

func TestProof_ASharedPathIsT3(t *testing.T) {
	got := Prove(baseInput([]string{"docs/guide.md"}, []string{"docs/guide.md"}))

	assertTier(t, got, T3, RuleSharedPath)
	assertStrings(t, "shared_paths", got.Evidence.SharedPaths, "docs/guide.md")
}

func TestProof_APeerChangeInTheLaneClosureIsT3(t *testing.T) {
	got := Prove(baseInput([]string{src("top")}, []string{src("a")}))

	assertTier(t, got, T3, RulePackageEdge)
	assertEdges(t, "edges_lane_to_peer", got.Evidence.EdgesLaneToPeer, Edge{From: ip("top"), To: ip("a")})
	assertEdges(t, "edges_peer_to_lane", got.Evidence.EdgesPeerToLane)
}

func TestProof_ALaneChangeInThePeerClosureIsT3(t *testing.T) {
	got := Prove(baseInput([]string{src("a")}, []string{src("top")}))

	assertTier(t, got, T3, RulePackageEdge)
	assertEdges(t, "edges_lane_to_peer", got.Evidence.EdgesLaneToPeer)
	assertEdges(t, "edges_peer_to_lane", got.Evidence.EdgesPeerToLane, Edge{From: ip("top"), To: ip("a")})
}

func TestProof_ASamePackageIsAnEdgeBothWays(t *testing.T) {
	got := Prove(baseInput([]string{src("a")}, []string{"go/internal/a/a_test.go"}))

	assertTier(t, got, T3, RulePackageEdge)
	assertEdges(t, "edges_lane_to_peer", got.Evidence.EdgesLaneToPeer, Edge{From: ip("a"), To: ip("a")})
	assertEdges(t, "edges_peer_to_lane", got.Evidence.EdgesPeerToLane, Edge{From: ip("a"), To: ip("a")})
}

func TestProof_ASharedLeafImportIsT1(t *testing.T) {
	got := Prove(baseInput([]string{src("b")}, []string{src("c")}))

	assertTier(t, got, T1, RuleDisjoint)
}

func TestProof_ADerivedEntryAloneIsT2(t *testing.T) {
	in := baseInput([]string{src("flagregistry")}, []string{"docs/architecture/control-flags.md"})
	in.Catalogs = fakeCatalogs{
		derived: map[Side]map[string]bool{SidePeer: {"docs/architecture/control-flags.md": true}},
		fired:   []string{"flag-index"},
	}

	got := Prove(in)

	assertTier(t, got, T2, RuleDerived)
	assertStrings(t, "derived", got.Evidence.Derived, "flag-index")
	assertStrings(t, "unknown", got.Evidence.Unknown)
}

func TestProof_ASharedDerivedOutputIsNotASharedPath(t *testing.T) {
	out := "docs/architecture/control-flags.md"
	in := baseInput([]string{out}, []string{out})
	in.Catalogs = fakeCatalogs{
		derived: map[Side]map[string]bool{SideLane: {out: true}, SidePeer: {out: true}},
		fired:   []string{"flag-index"},
	}

	got := Prove(in)

	assertTier(t, got, T2, RuleDerived)
	assertStrings(t, "shared_paths", got.Evidence.SharedPaths)
}

func TestProof_ADerivedOutputOnOneSideOnlyStaysAShareCandidate(t *testing.T) {
	out := "docs/architecture/control-flags.md"
	in := baseInput([]string{out}, []string{out})
	in.Catalogs = fakeCatalogs{derived: map[Side]map[string]bool{SidePeer: {out: true}}}

	got := Prove(in)

	assertTier(t, got, T3, RuleSharedPath)
	assertStrings(t, "shared_paths", got.Evidence.SharedPaths, out)
}

func TestProof_ADerivedEntryAndAPackageEdgeIsT3(t *testing.T) {
	in := baseInput([]string{src("top")}, []string{src("a"), "docs/architecture/control-flags.md"})
	in.Catalogs = fakeCatalogs{
		derived: map[Side]map[string]bool{SidePeer: {"docs/architecture/control-flags.md": true}},
		fired:   []string{"flag-index"},
	}

	got := Prove(in)

	assertTier(t, got, T3, RulePackageEdge, RuleDerived)
}

func TestProof_ABookkeepingOnlyPeerIsT1BeforeStepThree(t *testing.T) {
	in := baseInput(
		[]string{"skills/x/SKILL.md", "go/go.mod", src("a")},
		[]string{".evolve/inbox/x.json", "knowledge-base/cycles/cycle-9.json", "go/acs/cycle9/x_test.go"},
	)
	in.CompileRed = true
	in.Failures = []string{"go list: exit 1"}

	got := Prove(in)

	assertTier(t, got, T1, RuleBookkeepingPeer)
}

func TestProof_AnEmptyPeerIsT1ByItsOwnRule(t *testing.T) {
	got := Prove(baseInput([]string{src("a"), "skills/x/SKILL.md"}, nil))

	assertTier(t, got, T1, RuleEmptyPeer)
}

func TestProof_BookkeepingNeverRaisesTheTier(t *testing.T) {
	inbox := ".evolve/inbox/a.json"
	in := baseInput(
		[]string{src("b"), inbox, "go/acs/cycle12/p_test.go"},
		[]string{src("c"), inbox, "go/acs/cycle12/p_test.go", "knowledge-base/cycles/cycle-12.md"},
	)

	got := Prove(in)

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "shared_paths", got.Evidence.SharedPaths)
	assertStrings(t, "unknown", got.Evidence.Unknown)
	assertStrings(t, "lane packages", got.Selection.LanePackages, ip("b"))
}

func TestProof_AGenuineConflictIsT4(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Input)
		rules []Rule
	}{
		{"genuine conflict", func(in *Input) { in.Merge = MergeGenuineConflict }, []Rule{RuleConflict}},
		{"base not an ancestor", func(in *Input) { in.BaseNotAncestor = true }, []Rule{RuleBaseNotAncestor}},
		{"audited tree missing", func(in *Input) { in.AuditedTreeMissing = true }, []Rule{RuleAuditedTreeMissing}},
		{"all three", func(in *Input) {
			in.Merge = MergeGenuineConflict
			in.BaseNotAncestor = true
			in.AuditedTreeMissing = true
		}, []Rule{RuleConflict, RuleBaseNotAncestor, RuleAuditedTreeMissing}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInput([]string{src("b")}, []string{".evolve/inbox/x.json"})
			tc.setup(&in)

			got := Prove(in)

			assertTier(t, got, T4, tc.rules...)
			if got.EvidenceDigest != "" {
				t.Fatalf("a step-1 ejection has no evidence digest, got %q", got.EvidenceDigest)
			}
		})
	}
}

func TestProof_ADerivedConflictIsNotStepOne(t *testing.T) {
	in := baseInput([]string{src("b")}, []string{src("c")})
	in.Merge = MergeDerivedConflict

	got := Prove(in)

	assertTier(t, got, T1, RuleDisjoint)
}

func TestProof_ACompileRedIsT4BeforeTheRestOfStepThree(t *testing.T) {
	in := baseInput([]string{src("top")}, []string{src("a")})
	in.CompileRed = true

	got := Prove(in)

	assertTier(t, got, T4, RuleCompile)
}

func TestProof_ADeletedFileCountsByItsDirectory(t *testing.T) {
	in := baseInput([]string{src("top")}, []string{"go/internal/a/gone.go", "go/internal/z/gone.go"})
	in.DeletedAtC = []string{"go/internal/a/gone.go", "go/internal/z/gone.go"}

	got := Prove(in)

	assertTier(t, got, T3, RulePackageEdge)
	assertStrings(t, "unknown", got.Evidence.Unknown)
	assertStrings(t, "peer packages", got.Selection.PeerPackages, ip("a"), ip("z"))
}

func TestProof_ADeletedFileAtTheModuleRootCountsAsTheModule(t *testing.T) {
	in := baseInput([]string{"go/main.go"}, []string{src("b")})
	in.DeletedAtC = []string{"go/main.go"}

	got := Prove(in)

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "lane packages", got.Selection.LanePackages, testModule)
}

func TestProof_DataReadsAreEvidenceThatDoesNotRaiseTheTier(t *testing.T) {
	in := baseInput([]string{"docs/guide.md", ".evolve/inbox/x.json"}, []string{src("b")})
	in.Catalogs = fakeCatalogs{dataReads: map[string]bool{"docs/guide.md": true, ".evolve/inbox/x.json": true}}

	got := Prove(in)

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "data_edges", got.Evidence.DataEdges, "docs/guide.md")
}

func TestProof_ADeletedFileWithNoModuleIsUnknown(t *testing.T) {
	in := baseInput([]string{src("b")}, []string{"go/internal/z/gone.go"})
	in.Module = Module{}
	in.DeletedAtC = []string{"go/internal/z/gone.go"}
	in.Failures = []string{"go list: exit status 1"}

	got := Prove(in)

	assertTier(t, got, T3, RuleUnknown)
	assertStrings(t, "unknown", got.Evidence.Unknown, "go list: exit status 1", "go/internal/b/b.go", "go/internal/z/gone.go")
	assertStrings(t, "peer packages", got.Selection.PeerPackages)
}

func TestProof_TheCatalogSeesTheConflictedPathsOfADerivedConflict(t *testing.T) {
	out := "docs/architecture/control-flags.md"
	in := baseInput([]string{src("flagregistry"), out}, []string{out, src("b")})
	in.Merge = MergeDerivedConflict
	in.Conflicted = []string{out, out}
	in.Catalogs = fakeCatalogs{
		derived:    map[Side]map[string]bool{SideLane: {out: true}, SidePeer: {out: true}},
		fired:      []string{"flag-index"},
		onConflict: []string{out},
	}

	got := Prove(in)

	assertTier(t, got, T2, RuleDerived)
	assertStrings(t, "derived", got.Evidence.Derived, "flag-index")
}

func TestMergeClass_OnlyTheGenuineClassEjectsAtStepOne(t *testing.T) {
	want := map[MergeClass]Tier{MergeClean: T1, MergeDerivedConflict: T1, MergeGenuineConflict: T4}
	for class, tier := range want {
		in := baseInput([]string{src("b")}, []string{src("c")})
		in.Merge = class

		if got := Prove(in); got.Tier != tier {
			t.Errorf("merge class %s: tier = %s, want %s", class, got.Tier, tier)
		}
	}
}

func TestProof_ADeletedNonGoFileUnderGoIsUnknown(t *testing.T) {
	gone := []string{"go/internal/x/fixtures/a.json", "go/internal/x/notes.txt"}
	in := baseInput([]string{src("b")}, gone)
	in.DeletedAtC = gone

	got := Prove(in)

	assertTier(t, got, T3, RuleUnknown)
	assertStrings(t, "unknown", got.Evidence.Unknown, gone...)
	assertStrings(t, "peer packages", got.Selection.PeerPackages)
}

func TestProof_ADeletedGoFileOwnsItsDirectoryPackage(t *testing.T) {
	in := baseInput([]string{src("b")}, []string{"go/internal/x/x.go"})
	in.DeletedAtC = []string{"go/internal/x/x.go"}

	got := Prove(in)

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "peer packages", got.Selection.PeerPackages, ip("x"))
}

func TestProof_ARenameListedWithBothPathsMapsBothSides(t *testing.T) {
	in := baseInput([]string{"go/internal/a/old.go", "go/internal/b/new.go"}, []string{src("top")})
	in.DeletedAtC = []string{"go/internal/a/old.go"}
	in.Module.Packages[1].Files = append(in.Module.Packages[1].Files, "go/internal/b/new.go")

	got := Prove(in)

	assertTier(t, got, T3, RulePackageEdge)
	assertStrings(t, "lane packages", got.Selection.LanePackages, ip("a"), ip("b"))
	assertEdges(t, "edges_peer_to_lane", got.Evidence.EdgesPeerToLane, Edge{From: ip("top"), To: ip("a")})
	assertStrings(t, "unknown", got.Evidence.Unknown)
}

func TestStricter_RanksTheTiersAndNotTheirSpelling(t *testing.T) {
	cases := []struct{ a, b, want Tier }{
		{T1, T2, T2}, {T3, T2, T3}, {T4, T1, T4}, {T2, T2, T2}, {T3, Tier("Tz"), T3},
	}
	for _, tc := range cases {
		if got := stricter(tc.a, tc.b); got != tc.want {
			t.Errorf("stricter(%s, %s) = %s, want %s", tc.a, tc.b, got, tc.want)
		}
	}
}
