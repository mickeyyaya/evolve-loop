package reviewclass

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	seatSimplifier   = "code-simplifier"
	seatGoOrCode     = "go-reviewer|code-reviewer"
	seatArchitecture = "architecture-reviewer"
	seatDocs         = "docs-review"
	seatCode         = "code-reviewer"
	seatGoTests      = "go-reviewer[lens=golang-test-review]"
	seatSecurity     = "security"
	seatAdversarial  = "adversarial-review[tier=deep,other-family]"
)

var (
	goCheckIDs  = []string{"lanes", "cover-changed", "apicover"}
	t0CheckIDs  = append(slices.Clone(goCheckIDs), "gate-flags", "guard-mutation")
	docCheckIDs = []string{"ste-lint", "docs-index-check", "committed-doc-guard"}
	goSeatNames = []string{seatSimplifier, seatGoOrCode}
	t1SeatNames = []string{seatSimplifier, seatGoOrCode, seatArchitecture}
	t0SeatNames = []string{seatSimplifier, seatGoOrCode, seatArchitecture, seatAdversarial}
)

type classCase struct {
	name    string
	diff    Diff
	classes []ID
	seats   []string
	checks  []string
}

func paths(p ...string) Diff { return Diff{Paths: p} }

var fakeGuarantees = map[string]string{
	"go/internal/guards/":                 "guard",
	"go/internal/bridge/sandbox_paths.go": "sandbox-rule",
	".claude/settings.json":               "guard",
	".evolve/policy.json":                 "policy-parser",
	".github/workflows/":                  "ship-or-commit-gate",
}

func fakeGuaranteesOf(p string) []string {
	var out []string
	for prefix, class := range fakeGuarantees {
		if strings.HasPrefix(p, prefix) {
			out = append(out, class)
		}
	}
	return out
}

var fakeSurface = Surface{
	Protected:  func(p string) bool { return len(fakeGuaranteesOf(p)) > 0 },
	Guarantees: fakeGuaranteesOf,
}

func classify(d Diff) Requirements { return Classify(fakeSurface, d) }

var classCases = []classCase{
	{"C0 a proven comment-only Go file", Diff{Paths: []string{"go/internal/guards/role.go"}, CommentOnly: []string{"go/internal/guards/role.go"}}, []ID{C0}, nil, []string{"comment-equivalence"}},
	{"C1 a document", paths("docs/guide.md"), []ID{C1}, []string{seatDocs}, docCheckIDs},
	{"C2 inbox data", paths(".evolve/inbox/pending/x.json"), []ID{C2}, nil, []string{"inbox-lint", "inbox-integrity"}},
	{"C3 a whole generated file", paths(".codex-plugin/plugin.json"), []ID{C3}, nil, []string{"derived-check:skill-projections"}},
	{"C4 a gate input", paths("go/.cover-strict"), []ID{C4}, []string{seatCode}, []string{"floor-list-regression"}},
	{"C5 a Go test", paths("go/internal/widget/widget_test.go"), []ID{C5}, []string{seatGoTests}, []string{"go-checks", "test-flags"}},
	{"C5 a testdata fixture", paths("go/internal/widget/testdata/in.json"), []ID{C5}, []string{seatGoTests}, []string{"go-checks", "test-flags"}},
	{"C6 ordinary Go code", paths("go/internal/widget/widget.go"), []ID{C6}, goSeatNames, goCheckIDs},
	{"C6 a guard test outside the manifest", paths("go/internal/widget/widget_guard_test.go"), []ID{C6}, goSeatNames, goCheckIDs},
	{"C7 Go T1 code", paths("go/internal/core/widget.go"), []ID{C7}, t1SeatNames, goCheckIDs},
	{"C7 a data file of a T1 package", paths("go/internal/bridge/recipe/recipes/plugin-install.json"), []ID{C7}, t1SeatNames, goCheckIDs},
	{"C5 needs a Go testdata directory", paths("agents/testdata/x.md"), []ID{C8}, []string{seatArchitecture, seatDocs}, []string{"persona-strip", "skills-check", "guidance-check", "ste-lint"}},
	{"C3 a whole generated document", paths("commands/audit.md"), []ID{C3}, nil, []string{"derived-check:skill-projections"}},
	{"C8 a phase prompt", paths(".evolve/phases/scout/agent.md"), []ID{C8}, []string{seatArchitecture, seatDocs}, []string{"persona-strip", "skills-check", "guidance-check", "ste-lint"}},
	{"C6 a data file of a T2 package", paths("go/internal/policy/routing_defaults.json"), []ID{C6}, goSeatNames, goCheckIDs},
	{"C13 a file under agents/ that is not a persona", paths("agents/x.json"), []ID{C13}, goSeatNames, goCheckIDs},
	{"C8 a persona", paths("agents/evolve-scout.md"), []ID{C8}, []string{seatArchitecture, seatDocs}, []string{"persona-strip", "skills-check", "guidance-check", "ste-lint"}},
	{"C8 a skill file", paths("skills/widget/reference.md"), []ID{C8}, []string{seatArchitecture, seatDocs}, []string{"persona-strip", "skills-check", "guidance-check", "ste-lint"}},
	{"C9 a profile", paths(".evolve/profiles/scout.json"), []ID{C9}, []string{seatCode, seatSecurity}, []string{"profile-load", "skills-check", "permission-widening"}},
	{"C9 a phase spec", paths(".evolve/phases/scout/phase.json"), []ID{C9}, []string{seatCode, seatSecurity}, []string{"profile-load", "skills-check", "permission-widening"}},
	{"C9 a CLI manifest, which is T1 data too", paths("go/internal/bridge/manifests/claude.json"), []ID{C7, C9}, append(slices.Clone(t1SeatNames), seatCode, seatSecurity), append(slices.Clone(goCheckIDs), "profile-load", "skills-check", "permission-widening")},
	{"C10 a workflow, which is protected too", paths(".github/workflows/go.yml"), []ID{C10, C11}, []string{seatCode, seatArchitecture, seatSimplifier, seatGoOrCode, seatAdversarial}, []string{"required-needs", "ci-routes-generated", "lanes", "cover-changed", "apicover", "gate-flags", "guard-mutation"}},
	{"C11 a guard", paths("go/internal/guards/role.go"), []ID{C6, C11}, t0SeatNames, t0CheckIDs},
	{"C11 a sandbox rule adds the security seat", paths("go/internal/bridge/sandbox_paths.go"), []ID{C7, C11}, append(slices.Clone(t0SeatNames), seatSecurity), t0CheckIDs},
	{"C11 a protected non-Go file", paths(".claude/settings.json"), []ID{C11}, t0SeatNames, t0CheckIDs},
	{"C12 a stage flip", Diff{Paths: []string{".evolve/policy.json"}, StageFlips: []string{".evolve/policy.json"}}, []ID{C11, C12}, append(slices.Clone(t0SeatNames), seatSecurity), append(slices.Clone(t0CheckIDs), "operator-approved")},
	{"C13 a path that matches no class", paths("Makefile"), []ID{C13}, goSeatNames, goCheckIDs},
	{"C13 a module file", paths("go/go.mod"), []ID{C13}, goSeatNames, goCheckIDs},
	{"a generated region in a document is C1 and C3", paths("docs/architecture/control-flags.md"), []ID{C1, C3}, []string{seatDocs}, append(slices.Clone(docCheckIDs), "derived-check:flag-index")},
	{"R1 a mixed diff gets the union", paths(".evolve/inbox/pending/x.json", "go/internal/guards/role.go"), []ID{C2, C6, C11}, t0SeatNames, append([]string{"inbox-lint", "inbox-integrity"}, t0CheckIDs...)},
	{"an empty diff needs nothing", Diff{}, nil, nil, nil},
}

func seatStrings(seats []Seat) []string {
	var out []string
	for _, s := range seats {
		out = append(out, s.String())
	}
	return out
}

func checkIDs(checks []Check) []string {
	var out []string
	for _, c := range checks {
		out = append(out, c.ID)
	}
	return out
}

func hitIDs(hits []Hit) []ID {
	var out []ID
	for _, h := range hits {
		out = append(out, h.Class)
	}
	return out
}

func TestClassify_EachClassGetsItsSeatsAndChecks(t *testing.T) {
	t.Parallel()
	for _, tc := range classCases {
		t.Run(tc.name, func(t *testing.T) {
			got := classify(tc.diff)
			if ids := hitIDs(got.Classes); !slices.Equal(ids, tc.classes) {
				t.Errorf("classes = %v, want %v", ids, tc.classes)
			}
			if seats := seatStrings(got.Seats); !slices.Equal(seats, tc.seats) {
				t.Errorf("seats = %v, want %v", seats, tc.seats)
			}
			if checks := checkIDs(got.Checks); !slices.Equal(checks, tc.checks) {
				t.Errorf("checks = %v, want %v", checks, tc.checks)
			}
		})
	}
}

func TestClassify_EachHitNamesItsClassAndItsSortedPaths(t *testing.T) {
	t.Parallel()
	got := classify(paths("go/internal/widget/b.go", "go/internal/widget/a.go", "docs/x.md"))
	want := []Hit{
		{Class: C1, Name: "Docs", Paths: []string{"docs/x.md"}},
		{Class: C6, Name: "Go T2", Paths: []string{"go/internal/widget/a.go", "go/internal/widget/b.go"}},
	}
	if b, w := mustJSON(t, got.Classes), mustJSON(t, want); b != w {
		t.Errorf("classes = %s, want %s", b, w)
	}
}

func TestClassify_AnEmptyDiffPrintsEmptyListsNotNull(t *testing.T) {
	t.Parallel()
	if got, want := classify(Diff{}), (Requirements{Classes: []Hit{}, Seats: []Seat{}, Checks: []Check{}}); !reflect.DeepEqual(got, want) {
		t.Errorf("classify(Diff{}) = %+v, want %+v", got, want)
	}
	if got, want := mustJSON(t, classify(Diff{})), `{"classes":[],"seats":[],"checks":[]}`; got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestClassify_AGeneratedCheckNamesTheVerbOfItsEntry(t *testing.T) {
	t.Parallel()
	got := classify(paths(".codex-plugin/plugin.json")).Checks
	if len(got) != 1 || got[0].Rule != "evolve skills check" {
		t.Errorf("checks = %+v, want one check with the rule %q", got, "evolve skills check")
	}
}

func TestClassify_APlannedSeatAndCheckNameTheirComponent(t *testing.T) {
	t.Parallel()
	got := classify(paths("go/internal/bridge/sandbox_paths.go"))
	planned := map[string]string{}
	for _, s := range got.Seats {
		planned[s.String()] = s.Planned
	}
	for _, c := range got.Checks {
		planned[c.ID] = c.Planned
	}
	for key, want := range map[string]string{seatSecurity: "S6", seatAdversarial: "S6", "guard-mutation": "S11", seatSimplifier: "", "lanes": ""} {
		if planned[key] != want {
			t.Errorf("%s planned = %q, want %q", key, planned[key], want)
		}
	}
}

func TestMergeSeat_TheHighestTierWinsAndOtherFamilyStays(t *testing.T) {
	t.Parallel()
	seats := mergeSeat(nil, Seat{Any: []string{"x"}})
	seats = mergeSeat(seats, Seat{Any: []string{"x"}, Tier: TierDeep, OtherFamily: true})
	seats = mergeSeat(seats, Seat{Any: []string{"x"}})
	seats = mergeSeat(seats, Seat{Any: []string{"y"}})
	if got, want := seatStrings(seats), []string{"x[tier=deep,other-family]", "y"}; !slices.Equal(got, want) {
		t.Errorf("seats = %v, want %v", got, want)
	}
}

func TestSeat_StringNamesTheAlternativesTheLensTheTierAndTheFamily(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		seat Seat
		want string
	}{
		{Seat{Any: []string{"a"}}, "a"},
		{Seat{Any: []string{"a", "b"}}, "a|b"},
		{Seat{Any: []string{"a"}, Lens: "l"}, "a[lens=l]"},
		{Seat{Any: []string{"a"}, Lens: "l", Tier: "deep", OtherFamily: true}, "a[lens=l,tier=deep,other-family]"},
	} {
		if got := tc.seat.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.seat, got, tc.want)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestClassify_ASurfaceWithNoPredicatesFailsClosed(t *testing.T) {
	t.Parallel()
	got := Classify(Surface{}, paths("docs/guide.md"))
	if ids := hitIDs(got.Classes); !slices.Equal(ids, []ID{C1, C11}) {
		t.Errorf("classes = %v, want [C1 C11]", ids)
	}
	if seats := seatStrings(got.Seats); !slices.Contains(seats, seatSecurity) {
		t.Errorf("seats = %v, want the security seat", seats)
	}
}

func TestSurface_TheGuaranteesOfAPathDecideTheSecuritySeat(t *testing.T) {
	t.Parallel()
	s := Surface{Protected: func(string) bool { return true }, Guarantees: func(string) []string { return []string{"guard"} }}
	if seats := seatStrings(Classify(s, paths("x.go")).Seats); slices.Contains(seats, seatSecurity) {
		t.Errorf("seats = %v: a guard path needs no security seat", seats)
	}
}
