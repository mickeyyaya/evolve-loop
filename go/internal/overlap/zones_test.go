package overlap

import "testing"

func TestProof_ABuildZonePathIsT3(t *testing.T) {
	for _, p := range []string{"go/go.sum", "go/go.mod", ".evolve/policy.json", ".gitattributes", "docs/.gitignore"} {
		t.Run(p, func(t *testing.T) {
			got := Prove(baseInput([]string{src("b")}, []string{p}))

			assertTier(t, got, T3, RuleBuildZone)
			assertStrings(t, "build_zone", got.Evidence.BuildZone, p)
			assertStrings(t, "unknown", got.Evidence.Unknown)
		})
	}
}

func TestProof_AGateZonePathWidensTheTestsWithoutRaisingTheTier(t *testing.T) {
	got := Prove(baseInput([]string{src("b"), ".github/workflows/go.yml"}, []string{"go/Makefile"}))

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "gate_zone", got.Evidence.GateZone, ".github/workflows/go.yml", "go/Makefile")
	assertStrings(t, "unknown", got.Evidence.Unknown)
}

func TestProof_AVendoredFileIsInTheBuildZone(t *testing.T) {
	vendored := "go/vendor/gopkg.in/yaml.v3/yaml.go"

	got := Prove(baseInput([]string{src("b")}, []string{vendored}))

	assertTier(t, got, T3, RuleBuildZone)
	assertStrings(t, "build_zone", got.Evidence.BuildZone, vendored)
	assertStrings(t, "unknown", got.Evidence.Unknown)
}

func TestProof_AnEmbeddedFileMapsToItsPackage(t *testing.T) {
	got := Prove(baseInput([]string{src("top")}, []string{"go/internal/a/assets/x.txt"}))

	assertTier(t, got, T3, RulePackageEdge)
	assertStrings(t, "unknown", got.Evidence.Unknown)
	assertStrings(t, "peer packages", got.Selection.PeerPackages, ip("a"))
}

func TestProof_ATestdataFileMapsToItsPackage(t *testing.T) {
	got := Prove(baseInput([]string{src("b")}, []string{"go/internal/a/testdata/golden/x.json"}))

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "unknown", got.Evidence.Unknown)
	assertStrings(t, "peer packages", got.Selection.PeerPackages, ip("a"))
}

func TestProof_AnUnownedPathUnderGoIsUnknown(t *testing.T) {
	for _, p := range []string{"go/internal/a/notes.txt", "go/internal/zz/testdata/x.json", "go/README.md"} {
		t.Run(p, func(t *testing.T) {
			got := Prove(baseInput([]string{src("b")}, []string{p}))

			assertTier(t, got, T3, RuleUnknown)
			assertStrings(t, "unknown", got.Evidence.Unknown, p)
			assertStrings(t, "peer packages", got.Selection.PeerPackages)
		})
	}
}

func TestProof_ASkillBodyChangeIsUnknown(t *testing.T) {
	got := Prove(baseInput([]string{"skills/build/SKILL.md"}, []string{src("b")}))

	assertTier(t, got, T3, RuleUnknown)
	assertStrings(t, "unknown", got.Evidence.Unknown, "skills/build/SKILL.md")
}

func TestProof_AReadRootIsUnknown(t *testing.T) {
	for _, p := range []string{"docs/conventions/ste100-writing.md", "docs/research/x.md", "docs/reference/y/z.md"} {
		t.Run(p, func(t *testing.T) {
			got := Prove(baseInput([]string{p}, []string{src("b")}))

			assertTier(t, got, T3, RuleUnknown)
			assertStrings(t, "unknown", got.Evidence.Unknown, p)
		})
	}
}

func TestProof_AProseDocIsNotUnknown(t *testing.T) {
	prose := []string{"docs/architecture/x.md", "solutions/deal/plan.md", "docs/conventions/other.md", "docs/researcher.md"}

	got := Prove(baseInput(prose, []string{src("b")}))

	assertTier(t, got, T1, RuleDisjoint)
	assertStrings(t, "unknown", got.Evidence.Unknown)
}

func TestProof_AnythingElseIsUnknown(t *testing.T) {
	for _, p := range []string{"docs/diagram.svg", "README.md", "agents/x.md", ".evolve/profiles/a.json", "landing/index.md"} {
		t.Run(p, func(t *testing.T) {
			got := Prove(baseInput([]string{p}, []string{src("b")}))

			assertTier(t, got, T3, RuleUnknown)
			assertStrings(t, "unknown", got.Evidence.Unknown, p)
		})
	}
}

func TestProof_ANonPlainPathIsUnknownAndT3(t *testing.T) {
	for _, p := range []string{"docs/café.md", "docs/a:b.md", "docs/x./y.md", ".evolve/inbox/a~1.json"} {
		t.Run(p, func(t *testing.T) {
			got := Prove(baseInput([]string{src("b")}, []string{p}))

			assertTier(t, got, T3, RuleUnknown)
			assertStrings(t, "unknown", got.Evidence.Unknown, p)
		})
	}
}

func TestProof_AGraphFailureIsUnknownAndT3(t *testing.T) {
	in := baseInput([]string{src("b")}, []string{src("c")})
	in.Failures = []string{"go list -tags integration: exit status 1"}

	got := Prove(in)

	assertTier(t, got, T3, RuleUnknown)
	assertStrings(t, "unknown", got.Evidence.Unknown, "go list -tags integration: exit status 1")
}

func TestProof_TheStrictestRuleWinsAndEveryRuleIsRecorded(t *testing.T) {
	in := baseInput(
		[]string{src("top"), "docs/guide.md", "skills/x/SKILL.md"},
		[]string{src("a"), "docs/guide.md", "go/go.sum", "docs/architecture/control-flags.md"},
	)
	in.Catalogs = fakeCatalogs{
		derived: map[Side]map[string]bool{SidePeer: {"docs/architecture/control-flags.md": true}},
		fired:   []string{"flag-index"},
	}

	got := Prove(in)

	assertTier(t, got, T3, RuleSharedPath, RulePackageEdge, RuleBuildZone, RuleUnknown, RuleDerived)
}
