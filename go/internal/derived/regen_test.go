package derived

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type recorder struct {
	evolve  [][]string
	git     [][]string
	failOn  map[string]int
	gitOut  string
	gitExit int
	gitErr  error
}

func (r *recorder) run(_ context.Context, args ...string) error {
	r.evolve = append(r.evolve, args)
	key := strings.Join(args, " ")
	if r.failOn[key] > 0 {
		r.failOn[key]--
		return errors.New(key + " failed")
	}
	return nil
}

func (r *recorder) gitRun(_ context.Context, args ...string) (string, int, error) {
	r.git = append(r.git, args)
	return r.gitOut, r.gitExit, r.gitErr
}

func (r *recorder) worktree(base string) Worktree {
	return Worktree{Run: Generator(r.run), Git: Git(r.gitRun), Base: base}
}

func mustLookup(t *testing.T, name string) Entry {
	t.Helper()
	e, ok := Lookup(name)
	if !ok {
		t.Fatalf("Lookup(%q) found no entry", name)
	}
	return e
}

func TestRegen_RefusesALeftoverConflictMarker(t *testing.T) {
	r := &recorder{gitOut: "commands/build.md:4: leftover conflict marker\n", gitExit: 2}
	e := mustLookup(t, "skill-projections")

	err := r.worktree("main").Regenerate(context.Background(), e)

	if err == nil || !strings.Contains(err.Error(), "leftover conflict marker") {
		t.Fatalf("Regenerate = %v, want a refusal that names the leftover conflict marker", err)
	}
	want := append([]string{"diff", "--check", "main", "--"}, e.Pathspecs()...)
	if len(r.git) != 1 || !reflect.DeepEqual(r.git[0], want) {
		t.Fatalf("git calls = %v, want one %v", r.git, want)
	}
}

func TestRegen_GeneratesThenChecksThenRefusesNothingOnACleanDiff(t *testing.T) {
	r := &recorder{gitOut: "docs/architecture/control-flags.md:3: trailing whitespace.\n", gitExit: 2}

	err := r.worktree("HEAD").Regenerate(context.Background(), mustLookup(t, "flag-index"))

	if err != nil {
		t.Fatalf("Regenerate = %v, want nil: a whitespace report is no leftover conflict marker", err)
	}
	want := [][]string{{"flags", "generate"}, {"flags", "check"}}
	if !reflect.DeepEqual(r.evolve, want) {
		t.Fatalf("generator calls = %v, want %v", r.evolve, want)
	}
}

func TestRegen_EachFailedStepIsAnError(t *testing.T) {
	cases := []struct {
		name string
		r    *recorder
		want string
	}{
		{"the generator fails", &recorder{failOn: map[string]int{"signals codes generate": 1}}, "regenerate signal-codes via `evolve signals codes generate`"},
		{"the check after it fails", &recorder{failOn: map[string]int{"signals codes check": 1}}, "check signal-codes after regeneration"},
		{"git cannot run", &recorder{gitErr: errors.New("no git")}, "diff --check signal-codes against HEAD: no git"},
	}
	for _, c := range cases {
		err := c.r.worktree("HEAD").Regenerate(context.Background(), mustLookup(t, "signal-codes"))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Regenerate = %v, want an error containing %q", c.name, err, c.want)
		}
	}
}

func TestRefresh_ACleanCheckDoesNotRegenerate(t *testing.T) {
	r := &recorder{}

	if err := r.worktree("HEAD").Refresh(context.Background(), mustLookup(t, "flag-index")); err != nil {
		t.Fatalf("Refresh = %v, want nil", err)
	}
	if want := [][]string{{"flags", "check"}}; !reflect.DeepEqual(r.evolve, want) || len(r.git) != 0 {
		t.Fatalf("calls = evolve %v git %v, want only %v", r.evolve, r.git, want)
	}
}

func TestRefresh_DriftRegenerates(t *testing.T) {
	r := &recorder{failOn: map[string]int{"flags check": 1}}

	if err := r.worktree("HEAD").Refresh(context.Background(), mustLookup(t, "flag-index")); err != nil {
		t.Fatalf("Refresh = %v, want nil", err)
	}
	want := [][]string{{"flags", "check"}, {"flags", "generate"}, {"flags", "check"}}
	if !reflect.DeepEqual(r.evolve, want) || len(r.git) != 1 {
		t.Fatalf("calls = evolve %v git %v, want %v and one diff --check", r.evolve, r.git, want)
	}
}

func TestRegen_AGitErrorFromTheLeftoverCheckIsARefusal(t *testing.T) {
	cases := []struct {
		name string
		out  string
		exit int
	}{
		{"a bad pathspec", "", 128},
		{"a usage error", "", 129},
		{"an unknown exit", "", 1},
	}
	for _, c := range cases {
		r := &recorder{gitOut: c.out, gitExit: c.exit}

		err := r.worktree("HEAD").Regenerate(context.Background(), mustLookup(t, "skill-projections"))

		if err == nil || !strings.Contains(err.Error(), "exit") {
			t.Errorf("%s: Regenerate = %v, want a refusal that names the git exit", c.name, err)
		}
	}
}
