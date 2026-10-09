//go:build integration

package audit

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	importedParserFile = "go/internal/prompts/prompts.go"
	importedParserOld  = "		fm[key] = parseValue(val)\n"
	importedParserNew  = "		fm[key] = parseValue(val)\n\t\tif d, ok := fm[key].(string); ok && key == \"description\" {\n\t\t\tfm[key] = d + \" Imported change.\"\n\t\t}\n"
)

type generatorEdit func(t *testing.T, root string)

func rendererEdit(t *testing.T, root string) {
	t.Helper()
	editGenerator(t, root, rendererFile, laneMarkerOld, laneMarkerNew)
}

func importedParserEdit(t *testing.T, root string) {
	t.Helper()
	editSource(t, root, importedParserFile, importedParserOld, importedParserNew)
}

func peerShipsTheChange(t *testing.T, edit generatorEdit, regenerate bool) (*gittest.Repo, string) {
	t.Helper()
	repo := generatorLaneRepo(t)
	edit(t, repo.Dir)
	if regenerate {
		regenerateWithWorktreeGenerator(t, repo.Dir)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "peer: change the generator output")
	fixtures.MustWrite(t, filepath.Join(repo.Dir, "docs", "lane-notes.md"), "the lane's own change\n")
	return repo, repo.Git("rev-parse", "HEAD")
}

func TestSkillsDriftGate_Cycle1841ReplayBaseNewerThanTheHostPasses(t *testing.T) {
	repo, base := peerShipsTheChange(t, rendererEdit, true)
	stubs := generatedStubs(t, repo.Dir)
	if host, _ := skillcheck.Check(repo.Dir); !reflect.DeepEqual(sortedCopy(host), stubs) {
		t.Fatalf("precondition: the host generator must report every stub as drift, as in cycle 1841, got %v", host)
	}

	diags := skillsGateDiagnostics(t, skillsDriftCheckDefault, core.PhaseRequest{Cycle: 1841, Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: base})

	want := "PASS|warning|skills-drift: the worktree generator and the host generator disagree on " + strconv.Itoa(len(stubs)) + " artifact(s)"
	if len(diags) != 1 || !strings.HasPrefix(diags[0], want) {
		t.Fatalf("a lane on a base newer than the host generator must keep its PASS with one disagreement warning:\ngot  %q\nwant prefix %q", diags, want)
	}
}

func TestSkillsDriftGate_BaseGeneratorChangeWithoutRegenerationFails(t *testing.T) {
	repo, base := peerShipsTheChange(t, rendererEdit, false)
	if host, _ := skillcheck.Check(repo.Dir); len(host) != 0 {
		t.Fatalf("precondition: the host generator must see the old stubs as clean, got %v", host)
	}

	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: base})

	if err != nil {
		t.Fatalf("gate error: %v", err)
	}
	if want := generatedStubs(t, repo.Dir); !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("a base generator change without regeneration must fail on every stub:\ngot  %v\nwant %v", got, want)
	}
}

func TestSkillsDriftGate_ImportedPackageChangeIsGradedByTheWorktreeGenerator(t *testing.T) {
	cases := []struct {
		name       string
		regenerate bool
		wantFail   bool
	}{
		{"with consistent regeneration it passes", true, false},
		{"without regeneration it fails", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, base := peerShipsTheChange(t, importedParserEdit, tc.regenerate)
			stubs := generatedStubs(t, repo.Dir)
			host, _ := skillcheck.Check(repo.Dir)
			if tc.regenerate != (len(host) > 0) {
				t.Fatalf("precondition: the host generator must see drift only after the regeneration, got %v", host)
			}

			got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: base})

			if tc.wantFail {
				if err != nil || !reflect.DeepEqual(sortedCopy(got), stubs) {
					t.Fatalf("an imported-package change without regeneration = (%v, %v), want every stub as an offender %v", got, err, stubs)
				}
				return
			}
			if len(got) != 0 {
				t.Fatalf("an imported-package change with consistent regeneration must pass: got %d offender(s): %v", len(got), got)
			}
			requireDisagreement(t, err, len(host))
		})
	}
}
