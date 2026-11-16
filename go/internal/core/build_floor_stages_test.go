package core

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type stageFailures struct {
	static, tests []string
}

func stagedCheck(name string, calls *[]string, f stageFailures) BuildFloorCheck {
	return BuildFloorCheck{
		Name: name,
		Run: func(context.Context, ReviewInput) []string {
			*calls = append(*calls, name+":static")
			return f.static
		},
		Tests: func(context.Context, ReviewInput) []string {
			*calls = append(*calls, name+":tests")
			return f.tests
		},
	}
}

func TestBuildFloor_StageAReportsAllStaticFailuresTogether(t *testing.T) {
	t.Parallel()

	var calls []string
	floor := BuildHandoffFloor{
		stagedCheck("explanation", &calls, stageFailures{[]string{"explanation: red"}, []string{"explanation tests: red"}}),
		stagedCheck("production", &calls, stageFailures{[]string{"lint: red", "size: red"}, []string{"cover: red"}}),
		recordingCheck("solution", &calls, "solution: red"),
	}

	got := floor.Failures(context.Background(), ReviewInput{Phase: string(PhaseBuild)})

	want := []string{"explanation: red", "lint: red", "size: red", "solution: red"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stage A reports each static failure of each check in one round: got %q, want %q", got, want)
	}
}

func TestBuildFloor_AStageARedSkipsTheTestRun(t *testing.T) {
	t.Parallel()

	var calls []string
	floor := BuildHandoffFloor{
		stagedCheck("production", &calls, stageFailures{[]string{"lint: red"}, []string{"cover: red"}}),
	}

	_ = floor.Failures(context.Background(), ReviewInput{Phase: string(PhaseBuild)})

	for _, c := range calls {
		if strings.HasSuffix(c, ":tests") {
			t.Fatalf("a Stage A red must skip the long tests; calls = %q", calls)
		}
	}
}

func TestBuildFloor_AGreenStageARunsTheTestsAfterEveryStaticCheck(t *testing.T) {
	t.Parallel()

	var calls []string
	floor := BuildHandoffFloor{
		stagedCheck("first", &calls, stageFailures{nil, []string{"first tests: red"}}),
		stagedCheck("second", &calls, stageFailures{nil, nil}),
		recordingCheck("plain", &calls),
	}

	got := floor.Failures(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: "/wt"})

	if want := []string{"first tests: red"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a green Stage A runs Stage B and reports its failures: got %q, want %q", got, want)
	}
	wantCalls := []string{"first:static", "second:static", "plain@/wt", "first:tests", "second:tests"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("every static check runs before the first test check; calls = %q, want %q", calls, wantCalls)
	}
}

func TestBuildHandoffFloor_StaticFailuresNeverRunTheTests(t *testing.T) {
	t.Parallel()

	var calls []string
	floor := BuildHandoffFloor{
		stagedCheck("production", &calls, stageFailures{[]string{"lint: red"}, []string{"cover: red"}}),
		recordingCheck("plain", &calls, "plain: red"),
	}

	got := floor.StaticFailures(context.Background(), ReviewInput{Phase: string(PhaseBuild)})

	if want := []string{"lint: red", "plain: red"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("StaticFailures = %q, want %q", got, want)
	}
	for _, c := range calls {
		if strings.HasSuffix(c, ":tests") {
			t.Fatalf("StaticFailures never runs a test check; calls = %q", calls)
		}
	}
}

func TestMandatoryExplanationReviewer_AnExplanationGapSkipsTheComposedTests(t *testing.T) {
	t.Parallel()

	gap := "Explanation Documentation: the build is not sealed"
	cases := []struct {
		name   string
		staged stageFailures
		want   []string
	}{
		{"a red static stage", stageFailures{[]string{"[size] red"}, []string{"[cover] red"}}, []string{gap, "[size] red"}},
		{"a green static stage", stageFailures{nil, []string{"[cover] red"}}, []string{gap}},
	}
	for _, tc := range cases {
		var calls []string
		r := mandatoryExplanationReviewer{
			floor: BuildHandoffFloor{recordingCheck("explanation", &calls, gap)},
			next:  ChainReviewers(BuildHandoffFloor{stagedCheck("production", &calls, tc.staged)}),
		}

		got := r.Review(context.Background(), ReviewInput{Phase: string(PhaseBuild), ExplanationDocumentationVersion: 1, Worktree: "/wt"})

		if got.Approve || !reflect.DeepEqual(got.Failures, tc.want) {
			t.Fatalf("%s: an explanation gap reports the static failures of the composed floor only: got %+v, want %q", tc.name, got, tc.want)
		}
		for _, c := range calls {
			if strings.HasSuffix(c, ":tests") {
				t.Fatalf("%s: an explanation gap must skip the composed tests; calls = %q", tc.name, calls)
			}
		}
	}
}

func TestBuildFloorStages_TheDefaultCheckRunsTheTestsOnlyAfterAGreenStaticStage(t *testing.T) {
	t.Parallel()

	repo := gittest.Fixture(t)
	root := repo.Dir
	writeFile(t, filepath.Join(root, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(root, "go", ".apicover-enforce"), "./p\n")
	writeFile(t, filepath.Join(root, "go", "p", "a.go"), "package p\n\nfunc F() int { return 1 }\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(root, "go", ".apicover-enforce"), "./q\n")
	writeFile(t, filepath.Join(root, "go", "p", "a_test.go"), "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tt.Fatal(\"red\")\n}\n")
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "build-report.md"), "# Build Report\n\nresult: FULLSUITE_PLACEHOLDER\n")
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: root, WorktreeBaseSHA: base, Workspace: ws}

	got := DefaultBuildFloorChecks(context.Background(), in)

	if len(got) != 2 || !strings.Contains(got[0], "FULLSUITE_PLACEHOLDER") || !strings.Contains(got[1], "drops ./p") {
		t.Fatalf("a static red reports every static failure and never the failing test of the changed package: got %q", got)
	}
	if got := StaticBuildFloorChecks(context.Background(), ReviewInput{}); len(got) != 0 {
		t.Fatalf("no worktree, no static failure; got %q", got)
	}
	if got := SuiteBuildFloorChecks(context.Background(), ReviewInput{}); len(got) != 0 {
		t.Fatalf("no worktree, no test failure; got %q", got)
	}
}
