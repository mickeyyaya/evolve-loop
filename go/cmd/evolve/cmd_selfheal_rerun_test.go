package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

const rerunFixtureRun = int64(40269836920)

func rerunIncident(t *testing.T, env selfhealEnv) (selfhealEnvelope, string) {
	t.Helper()
	rc, e, stderr := runSelfhealJSON(t, env, selfhealProject(t, `{}`))
	if rc != 0 || !e.OK {
		t.Fatalf("rc=%d envelope=%+v stderr=%s", rc, e, stderr)
	}
	return e, stderr
}

func flakyIncidents(e selfhealEnvelope) []selfheal.Incident {
	var out []selfheal.Incident
	for _, inc := range e.Data.Incidents {
		if inc.Class == selfheal.ClassFlakyTest {
			out = append(out, inc)
		}
	}
	return out
}

func TestSelfhealCIRuns_ARerunGreenRunNamesTheFailingTestOnceAndTheProducerAsksOnlyForIt(t *testing.T) {
	t.Parallel()
	var asked []int64
	env := selfhealTestEnv(fixtureGH)
	env.earlier = func(_ context.Context, _ string, run selfheal.CIRun) ([]selfheal.CIFailure, error) {
		asked = append(asked, run.ID)
		return []selfheal.CIFailure{{Jobs: []string{"go-test"}, Package: "go/internal/cli/inboxcmd", Test: "TestRunInboxLint", Repro: "go/internal/cli/inboxcmd/lint_test.go:TestRunInboxLint"}}, nil
	}
	e, _ := rerunIncident(t, env)
	flaky := flakyIncidents(e)
	if len(flaky) != 1 || flaky[0].Fingerprint != "flaky-test:rerun-green:go/internal/cli/inboxcmd.TestRunInboxLint" || flaky[0].ReproTest != "go/internal/cli/inboxcmd/lint_test.go:TestRunInboxLint" {
		t.Fatalf("flaky incidents %+v", flaky)
	}
	if !slices.Equal(asked, []int64{rerunFixtureRun}) {
		t.Fatalf("the port was asked for runs %v, want only the rerun-green run", asked)
	}
}

func TestSelfhealCIRuns_AFailedEarlierTryFetchIsOneWarningAndNoFlakyIncident(t *testing.T) {
	t.Parallel()
	env := selfhealTestEnv(fixtureGH)
	env.earlier = func(context.Context, string, selfheal.CIRun) ([]selfheal.CIFailure, error) {
		return nil, errors.New("HTTP 502")
	}
	e, _ := rerunIncident(t, env)
	if flaky := flakyIncidents(e); len(flaky) != 0 {
		t.Fatalf("a transport error must not mint an identity: %+v", flaky)
	}
	if len(e.Warnings) != 1 {
		t.Fatalf("want one warning, got %v", e.Warnings)
	}
}

func TestSelfhealCIRuns_AReadThatFoundNoFailingTestNamesTheRunInTheSubject(t *testing.T) {
	t.Parallel()
	e, _ := rerunIncident(t, selfhealTestEnv(fixtureGH))
	flaky := flakyIncidents(e)
	if len(flaky) != 1 || flaky[0].Fingerprint != "flaky-test:rerun-green:run-40269836920" || flaky[0].ReproTest != "" {
		t.Fatalf("flaky incidents %+v", flaky)
	}
}

func TestSelfhealCIRuns_TheEarlierTryReadsOfAPassAreBoundedAndTheNewestRunsComeFirst(t *testing.T) {
	t.Parallel()
	total := selfhealEarlierReadLimit + 5
	var list []ghCIRun
	for i := 0; i < total; i++ {
		created := selfhealNow.Add(-time.Duration(total-i) * time.Minute)
		list = append(list, ghCIRun{DatabaseID: int64(1000 + i), HeadBranch: "f", Status: "completed", Conclusion: "success", Attempt: 2, CreatedAt: created, UpdatedAt: created})
	}
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	var asked []int64
	env := selfhealTestEnv(func(context.Context, string, ...string) (string, error) { return string(raw), nil })
	env.earlier = func(_ context.Context, _ string, run selfheal.CIRun) ([]selfheal.CIFailure, error) {
		asked = append(asked, run.ID)
		return nil, nil
	}
	runs, warnings, err := selfhealCIRuns(env, ".", selfhealNow.Add(-24*time.Hour))
	if err != nil || len(asked) != selfhealEarlierReadLimit || len(warnings) != 1 || len(runs) != total {
		t.Fatalf("asked %d warnings %v runs %d err %v", len(asked), warnings, len(runs), err)
	}
	if asked[0] != int64(1000+total-1) {
		t.Fatalf("the newest run was not read first: %v", asked)
	}
	unread := 0
	for _, r := range runs {
		if r.Unread {
			unread++
		}
	}
	if unread != 5 {
		t.Fatalf("want 5 unread runs, got %d", unread)
	}
}

func TestGhEarlierFailuresVia_OneFailureForEachFailingTestWithItsJobs(t *testing.T) {
	t.Parallel()
	var gotRun int64
	var gotAttempt int
	port := ghEarlierFailuresVia(func(_ context.Context, _ string, id int64, attempt int) ([]ciwatch.FailingTest, error) {
		gotRun, gotAttempt = id, attempt
		return []ciwatch.FailingTest{
			{Package: "p", Test: "TestA", Jobs: []string{"j1", "j2"}},
			{Step: "make lint"},
		}, nil
	})
	got, err := port(context.Background(), t.TempDir(), selfheal.CIRun{ID: 9, Attempt: 3})
	want := []selfheal.CIFailure{{Jobs: []string{"j1", "j2"}, Package: "p", Test: "TestA"}, {Step: "make lint"}}
	if err != nil || !reflect.DeepEqual(got, want) || gotRun != 9 || gotAttempt != 3 {
		t.Fatalf("got %+v err %v run %d attempt %d", got, err, gotRun, gotAttempt)
	}
	failing := ghEarlierFailuresVia(func(context.Context, string, int64, int) ([]ciwatch.FailingTest, error) {
		return nil, errors.New("boom")
	})
	if got, err := failing(context.Background(), ".", selfheal.CIRun{}); err == nil || len(got) != 0 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestGhEarlierFailuresVia_TheReproTestIsAFileTheVerifierAccepts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "go", "internal", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"0.go": "package x\nfunc TestFlaky(\n", "a_test.go": "package x\nfunc TestOther(t *testing.T) {}\n", "b_test.go": "package x\nfunc TestFlaky(t *testing.T) {}\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	leaf := filepath.Join(root, "x")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leaf, "a_test.go"), []byte("package x\nfunc TestFlaky(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := ghEarlierFailuresVia(func(context.Context, string, int64, int) ([]ciwatch.FailingTest, error) {
		return []ciwatch.FailingTest{
			{Package: "github.com/mickeyyaya/evolve-loop/go/internal/x", Test: "TestFlaky/case_one"},
			{Package: "github.com/mickeyyaya/evolve-loop/go/internal/x", Test: "TestMissing"},
			{Package: "github.com/mickeyyaya/evolve-loop/go/internal/nodir", Test: "TestFlaky"},
			{Package: "github.com/mickeyyaya/evolve-loop/go/internal/x"},
		}, nil
	})
	got, err := port(context.Background(), root, selfheal.CIRun{ID: 1, Attempt: 2})
	if err != nil || len(got) != 4 {
		t.Fatalf("got %+v err %v", got, err)
	}
	if got[0].Repro != "go/internal/x/b_test.go:TestFlaky" {
		t.Fatalf("repro = %q", got[0].Repro)
	}
	if got[1].Repro != "" || got[2].Repro != "" || got[3].Repro != "" {
		t.Fatalf("an unresolved test must have no repro: %+v", got)
	}
	if !(selfheal.TreeVerifier{Root: root, MainGreen: true}).ReproGreen(selfheal.Incident{ReproTest: got[0].Repro}) {
		t.Fatalf("TreeVerifier rejected %q", got[0].Repro)
	}
	if strings.Contains(fmt.Sprint(got[0].Repro), "/case_one") {
		t.Fatalf("a subtest leaked into the repro: %q", got[0].Repro)
	}
}
