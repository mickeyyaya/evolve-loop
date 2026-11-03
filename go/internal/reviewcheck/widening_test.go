package reviewcheck_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reviewcheck"
)

const profilePath = ".evolve/profiles/diff-code-reviewer.json"

func profileChange() reviewcheck.FileChange {
	return reviewcheck.FileChange{
		Path: profilePath,
		Base: &reviewcheck.Grants{AllowedTools: []string{"Read"}, WriteSubpaths: []string{".evolve/runs/cycle-*"}},
		Head: &reviewcheck.Grants{AllowedTools: []string{"Read", "Edit"}, WriteSubpaths: []string{".evolve/runs/cycle-*", "go/"}},
	}
}

func tracked(paths ...string) func(string) bool {
	return func(p string) bool { return slices.Contains(paths, p) }
}

func subjects(refusals []reviewcheck.Refusal) []string {
	var out []string
	for _, r := range refusals {
		out = append(out, r.Subject)
	}
	return out
}

func TestPermissionWidening_RefusesEachWiderEntryThatNamesNoConsumer(t *testing.T) {
	got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{profileChange()}, nil, tracked())
	want := []string{profilePath + " allowed_tools Edit", profilePath + " sandbox.write_subpaths go/"}
	if !slices.Equal(subjects(got), want) {
		t.Fatalf("subjects = %q, want %q", subjects(got), want)
	}
	if got[0].Check != reviewcheck.CheckPermissionWidening || !strings.Contains(got[0].Reason, "Widening-consumer:") {
		t.Errorf("refusal = %+v, want a permission-widening refusal that names the trailer", got[0])
	}
}

func TestPermissionWidening_ATrailerThatNamesATrackedConsumerAccountsForTheEntry(t *testing.T) {
	bodies := []string{"feat: widen\n\nWidening-consumer: " + profilePath + " allowed_tools agents/evolve-diff-code-reviewer.md Edit\nwidening-consumer: " + profilePath + " sandbox.write_subpaths go/cmd/evolve/main.go go/\n"}
	got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{profileChange()}, bodies, tracked("agents/evolve-diff-code-reviewer.md", "go/cmd/evolve/main.go"))
	if len(got) != 0 {
		t.Errorf("refusals = %+v, want none", got)
	}
}

func TestPermissionWidening_ATrailerForAnotherFieldOrEntryAccountsForNothing(t *testing.T) {
	bodies := []string{"Widening-consumer: " + profilePath + " allowed_tools go/a.go Bash\nWidening-consumer: " + profilePath + " env_passthrough go/a.go Edit\nWidening-consumer: other.json allowed_tools go/a.go Edit\n"}
	got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{profileChange()}, bodies, tracked("go/a.go"))
	if len(got) != 2 {
		t.Errorf("refusals = %+v, want both widenings refused", got)
	}
}

func TestPermissionWidening_AConsumerThatIsNotInTheTreeIsRefused(t *testing.T) {
	bodies := []string{"Widening-consumer: " + profilePath + " allowed_tools agents/gone.md Edit\nWidening-consumer: short\nWidening-consumer: a b c\n"}
	change := profileChange()
	change.Head = &reviewcheck.Grants{AllowedTools: []string{"Read", "Edit"}}
	got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{change}, bodies, tracked())
	if len(got) != 1 || !strings.Contains(got[0].Reason, "agents/gone.md") {
		t.Errorf("refusals = %+v, want one refusal that names agents/gone.md", got)
	}
}

func TestPermissionWidening_ANewFileWidensEveryEnvPassthroughEntry(t *testing.T) {
	change := reviewcheck.FileChange{Path: "go/internal/bridge/manifests/new.json", Head: &reviewcheck.Grants{EnvPassthrough: []string{"HOME"}}}
	got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{change}, nil, tracked())
	if !slices.Equal(subjects(got), []string{"go/internal/bridge/manifests/new.json env_passthrough HOME"}) {
		t.Errorf("refusals = %+v, want the one new entry", got)
	}
}

func TestPermissionWidening_ANarrowingOrADeletedFileIsNotRefused(t *testing.T) {
	narrowed := profileChange()
	narrowed.Base, narrowed.Head = narrowed.Head, narrowed.Base
	deleted := profileChange()
	deleted.Head = nil
	if got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{narrowed, deleted}, nil, tracked()); len(got) != 0 {
		t.Errorf("refusals = %+v, want none", got)
	}
}

func TestPermissionWidening_ATrailerWithTabsOrDoubledSpacesKeepsAnEntryWithSpaces(t *testing.T) {
	change := reviewcheck.FileChange{Path: profilePath, Base: &reviewcheck.Grants{}, Head: &reviewcheck.Grants{AllowedTools: []string{"Bash(git commit:*)"}}}
	bodies := []string{"Widening-consumer:\t" + profilePath + "  allowed_tools\tagents/a.md   Bash(git commit:*)\n"}
	if got := reviewcheck.PermissionWidening([]reviewcheck.FileChange{change}, bodies, tracked("agents/a.md")); len(got) != 0 {
		t.Errorf("refusals = %+v, want none", got)
	}
}
