package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const mergedPolicyUnknownKey = "workflow.zz_merged_reader_key"

type mergedPolicyRig struct {
	repo, bare, index string
}

func remoteTrackedCommit(t *testing.T, bare, rel, content string) {
	t.Helper()
	clone := t.TempDir()
	smGit(t, t.TempDir(), "clone", "-q", bare, clone)
	smGit(t, clone, "config", "user.email", "ci@example.com")
	smGit(t, clone, "config", "user.name", "ci")
	path := filepath.Join(clone, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, clone, "add", "-f", rel)
	smGit(t, clone, "commit", "-q", "-m", "remote: "+rel)
	smGit(t, clone, "push", "-q", "origin", "main")
}

func mergedPolicyBody(index string, extra string) string {
	return fmt.Sprintf(`{%s"code_index":{"backends":["gotypes"],"index_dir":%q,"trail_bin":"/abs/trail"}}`, extra, index)
}

func newMergedPolicyRig(t *testing.T) mergedPolicyRig {
	t.Helper()
	repo, bare := smInitRepoWithRemote(t)
	index := filepath.Join(filepath.Dir(repo), "index")
	remoteTrackedCommit(t, bare, ".evolve/policy.json", mergedPolicyBody(index, ""))
	smGit(t, repo, "pull", "-q", "--ff-only", "origin", "main")
	smGit(t, repo, "worktree", "add", "-q", "--detach", index, "HEAD")
	remoteTrackedCommit(t, bare, ".evolve/policy.json", mergedPolicyBody(index, `"workflow":{"zz_merged_reader_key":true},`))
	return mergedPolicyRig{repo: repo, bare: bare, index: index}
}

type boundaryPlans map[string][]boundaryStep

func mergedPolicyPlans(root string) boundaryPlans {
	a := boundaryArgs{goalText: "goal"}
	return boundaryPlans{"wave next": waveSteps(a, root, nil), "boundary run": boundarySteps(a, root)}
}

func stepIndex(steps []boundaryStep, verb string, leading ...string) int {
	return slices.IndexFunc(steps, func(s boundaryStep) bool {
		return s.verb == verb && len(s.args) >= len(leading) && slices.Equal(s.args[:len(leading)], leading)
	})
}

type planeCall struct {
	name string
	args []string
}

type planeBinary struct {
	calls   []planeCall
	verdict func() (int, string)
}

func (p *planeBinary) run(_ context.Context, name, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
	p.calls = append(p.calls, planeCall{name: name, args: slices.Clone(args)})
	rc, msg := p.verdict()
	if msg != "" {
		fmt.Fprintln(stderr, msg)
	}
	return rc, nil
}

func planePath(root string) string {
	return filepath.Join(root, "go", "bin", "evolve")
}

type mergedBoundaryRun struct {
	rc     int
	verbs  []string
	stdout string
	stderr string
	plane  *planeBinary
}

var (
	strictPolicyVerbs  = []string{"inbox", "selfheal", "gc", "code"}
	sideEffectOnlyVerb = []string{"checkpoint", "loop-stop", "pr", boundaryRatchetVerb, queueApplyVerb, "quality", "gc", "reset-sha", "doctor", boundaryLogVerb, "loop"}
)

func runMergedPolicyBoundary(t *testing.T, rig mergedPolicyRig, rebuiltVerdict func(rebuilt bool) (int, string)) mergedBoundaryRun {
	t.Helper()
	rebuilt := false
	plane := &planeBinary{verdict: func() (int, string) { return rebuiltVerdict(rebuilt) }}
	waveDispatch := dispatchWaveVerb(plane.run)
	var verbs []string
	dispatch := func(verb string, args []string, stdout, stderr io.Writer) int {
		verbs = append(verbs, verb)
		switch {
		case verb == "sync-main":
			return runSyncMain(args, nil, stdout, stderr)
		case verb == waveBuildVerb:
			rebuilt = true
			return 0
		case slices.Contains(sideEffectOnlyVerb, verb):
			return 0
		}
		return waveDispatch(verb, args, stdout, stderr)
	}
	var stdout, stderr bytes.Buffer
	rc := runBoundarySteps(dispatch, waveSteps(boundaryArgs{goalText: "goal"}, rig.repo, nil), &stdout, &stderr)
	return mergedBoundaryRun{rc: rc, verbs: verbs, stdout: stdout.String(), stderr: stderr.String(), plane: plane}
}

func planeVerbs(p *planeBinary) []string {
	var verbs []string
	for _, c := range p.calls {
		verbs = append(verbs, c.args[0])
	}
	return verbs
}

func TestBoundaryMergedPolicy_TheSyncMainStepOfEachPlanCompletesWhenTheMergeBringsAKeyOnlyTheMergedCodeReads(t *testing.T) {
	t.Parallel()
	for name := range mergedPolicyPlans("/plane") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rig := newMergedPolicyRig(t)
			steps := mergedPolicyPlans(rig.repo)[name]
			i := stepIndex(steps, "sync-main")
			if i < 0 {
				t.Fatalf("the %s plan has no sync-main step", name)
			}
			indexBefore := smHead(t, rig.index)
			var stdout, stderr bytes.Buffer

			rc := runSyncMain(steps[i].args, nil, &stdout, &stderr)

			if rc != 0 || smHead(t, rig.repo) != smBareHead(t, rig.bare) {
				t.Fatalf("rc=%d repo=%s origin=%s, want the merge done and exit 0 with a policy key that only the merged code reads\nstderr=%s", rc, smHead(t, rig.repo), smBareHead(t, rig.bare), stderr.String())
			}
			if smHead(t, rig.index) != indexBefore {
				t.Errorf("the sync-main step moved the code index with the pre-merge binary; the refresh belongs to a later step")
			}
		})
	}
}

func TestBoundaryMergedPolicy_EachPlanRefreshesTheCodeIndexAfterSyncMainAndTheWavePlanAfterTheBuild(t *testing.T) {
	t.Parallel()
	const root = "/plane"
	for name, steps := range mergedPolicyPlans(root) {
		sync := stepIndex(steps, "sync-main")
		refresh := stepIndex(steps, "code", "refresh")
		if sync < 0 || refresh < 0 || refresh < sync {
			t.Errorf("%s: sync-main at %d, code refresh at %d, want a code refresh step after sync-main", name, sync, refresh)
			continue
		}
		if !slices.Contains(steps[refresh].args, root) {
			t.Errorf("%s: the refresh step %v does not name the project root", name, steps[refresh].args)
		}
	}
	wave := mergedPolicyPlans(root)["wave next"]
	build, refresh, launch := stepIndex(wave, waveBuildVerb), stepIndex(wave, "code", "refresh"), len(wave)-1
	if build < 0 || refresh < build || refresh >= launch || wave[launch].verb != "loop" {
		t.Errorf("wave next: build at %d, code refresh at %d, loop at %d, want the refresh after the build and before the loop starts", build, refresh, launch)
	}
}

func TestBoundaryMergedPolicy_TheRefreshStepRunsTheRebuiltPlaneBinaryAndKeepsItsExitCode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	steps := mergedPolicyPlans(root)["wave next"]
	i := stepIndex(steps, "code", "refresh")
	if i < 0 {
		t.Fatal("the wave next plan has no code refresh step")
	}
	for _, rc := range []int{0, 7} {
		plane := &planeBinary{verdict: func() (int, string) { return rc, "" }}
		var stdout, stderr bytes.Buffer

		got := dispatchWaveVerb(plane.run)(steps[i].verb, steps[i].args, &stdout, &stderr)

		want := []planeCall{{name: planePath(root), args: append([]string{steps[i].verb}, steps[i].args...)}}
		if got != rc || fmt.Sprint(plane.calls) != fmt.Sprint(want) {
			t.Errorf("plane rc %d: dispatch rc=%d calls=%v, want the plane binary run once as %v and its exit code kept\nstderr=%s", rc, got, plane.calls, want, stderr.String())
		}
	}
}

func TestBoundaryMergedPolicy_TheBoundaryRunDispatcherHandlesTheRefreshStepOfItsPlan(t *testing.T) {
	t.Parallel()
	steps := mergedPolicyPlans(t.TempDir())["boundary run"]
	i := stepIndex(steps, "code", "refresh")
	if i < 0 {
		t.Fatal("the boundary run plan has no code refresh step")
	}
	var stdout, stderr bytes.Buffer

	dispatchBoundaryVerb(steps[i].verb, steps[i].args, &stdout, &stderr)

	if strings.Contains(stderr.String(), "no handler for step") {
		t.Errorf("dispatchBoundaryVerb has no handler for the refresh step %v: %s", steps[i], stderr.String())
	}
}

func TestBoundaryMergedPolicy_TheBoundaryOfAMergedReaderAndItsKeyCompletesWithTheRebuiltBinary(t *testing.T) {
	t.Parallel()
	rig := newMergedPolicyRig(t)

	run := runMergedPolicyBoundary(t, rig, func(rebuilt bool) (int, string) {
		if !rebuilt {
			return 1, "the plane binary was run before the build: unknown policy key " + mergedPolicyUnknownKey
		}
		return 0, ""
	})

	if run.rc != 0 {
		t.Fatalf("boundary rc=%d verbs=%v, want a completed boundary\nstderr=%s", run.rc, run.verbs, run.stderr)
	}
	sync, build, launch := slices.Index(run.verbs, "sync-main"), slices.Index(run.verbs, waveBuildVerb), slices.Index(run.verbs, "loop")
	if !(0 <= sync && sync < build && build < launch) {
		t.Errorf("verbs=%v, want sync-main, then the build, then the loop", run.verbs)
	}
	for _, verb := range []string{"inbox", "selfheal", "code"} {
		if first := slices.Index(run.verbs, verb); first < build || !slices.Contains(planeVerbs(run.plane), verb) {
			t.Errorf("%s: first step at %d (build at %d), plane verbs=%v, want it after the build and run by the rebuilt binary", verb, first, build, planeVerbs(run.plane))
		}
	}
}

func TestBoundaryMergedPolicy_AKeyNoReaderOfTheMergedTreeKnowsStopsTheBoundaryAtTheFirstStepOfTheRebuiltBinaryNamingTheKey(t *testing.T) {
	t.Parallel()
	rig := newMergedPolicyRig(t)

	run := runMergedPolicyBoundary(t, rig, func(bool) (int, string) {
		return 1, fmt.Sprintf("unknown policy key %q: remove it or check its spelling", mergedPolicyUnknownKey)
	})

	if run.rc == 0 || !strings.Contains(run.stderr, mergedPolicyUnknownKey) {
		t.Fatalf("rc=%d, want a failed boundary whose stderr names %s\nstderr=%s", run.rc, mergedPolicyUnknownKey, run.stderr)
	}
	if len(run.plane.calls) != 1 || !strings.Contains(run.stderr, "the loop-stop brake stays engaged") {
		t.Errorf("plane verbs=%v, want the first step of the rebuilt binary to stop the boundary with the brake engaged\nstderr=%s", planeVerbs(run.plane), run.stderr)
	}
	if slices.Contains(run.verbs, "loop") || smHead(t, rig.repo) != smBareHead(t, rig.bare) {
		t.Errorf("verbs=%v repo=%s origin=%s, want the merge done, the loop never started", run.verbs, smHead(t, rig.repo), smBareHead(t, rig.bare))
	}
}

func TestBoundaryMergedPolicy_TheStepsThatReadThePolicyStrictlyRunAfterTheBuildInEachPlan(t *testing.T) {
	t.Parallel()
	for name, steps := range mergedPolicyPlans("/plane") {
		sync, build := stepIndex(steps, "sync-main"), stepIndex(steps, waveBuildVerb)
		if sync < 0 || build < sync {
			t.Errorf("%s: sync-main at %d, build at %d, want a build step after sync-main", name, sync, build)
			continue
		}
		for i, s := range steps {
			if slices.Contains(strictPolicyVerbs, s.verb) && i < build {
				t.Errorf("%s: step %d (%s) reads the policy strictly before the build at %d, so the pre-merge binary parses the merged policy", name, i, s, build)
			}
		}
	}
}

func TestBoundaryMergedPolicy_TheStrictLoaderOfTheRefreshVerbStillRefusesAnUnknownKeyByName(t *testing.T) {
	t.Parallel()
	rig := newMergedPolicyRig(t)
	smGit(t, rig.repo, "pull", "-q", "--ff-only", "origin", "main")
	var stdout, stderr bytes.Buffer

	rc := codeRefreshVerb([]string{"--project-root", rig.repo}, &stdout, &stderr, defaultCodeIndexHost(), func(string) string { return "" }, codeIndexRefreshTimeout)

	if rc == 0 || !strings.Contains(stderr.String(), mergedPolicyUnknownKey) {
		t.Errorf("rc=%d stderr=%q, want a refusal that names %s", rc, stderr.String(), mergedPolicyUnknownKey)
	}
}

func TestBoundaryMergedPolicy_TheOperationsDocRecordsTheOrderOfTheMergeTheBuildAndTheRefresh(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "operations", "wave-operations.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	if strings.Contains(doc, "It is not a separate step") {
		t.Errorf("the doc still says the code index refresh is not a separate step")
	}
	var recorded bool
	for _, block := range strings.Split(doc, "\n\n") {
		sync, build, refresh := strings.Index(block, "sync-main"), strings.Index(block, "build"), strings.Index(block, "code refresh")
		recorded = recorded || (0 <= sync && sync < build && build < refresh)
	}
	if !recorded {
		t.Errorf("no paragraph of wave-operations.md names sync-main, then the build, then code refresh in that order")
	}
}

const planeScript = `#!/bin/sh
here=$(dirname "$0")/..
if [ ! -f "$here/built.marker" ]; then
  echo "unknown policy key workflow.zz_merged_reader_key: this binary was not rebuilt" >&2
  exit 1
fi
echo "$@" >> "$here/plane.log"
`

func writeBoundaryRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]struct {
		body string
		mode os.FileMode
	}{
		filepath.Join(".evolve", "policy.json"): {`{"workflow":{"zz_merged_reader_key":true}}`, 0o644},
		filepath.Join("go", "Makefile"):         {"build:\n\t@touch built.marker\n", 0o644},
		filepath.Join("go", "bin", "evolve"):    {planeScript, 0o755},
	}
	for rel, f := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(f.body), f.mode); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBoundaryMergedPolicy_TheBoundaryRunPlanReadsTheMergedPolicyWithTheBinaryTheBuildLeavesOnDisk(t *testing.T) {
	root := writeBoundaryRoot(t)
	steps := boundarySteps(boundaryArgs{goalText: "goal"}, root)
	dispatch := func(verb string, args []string, stdout, stderr io.Writer) int {
		if slices.Contains([]string{"loop-stop", "pr", "sync-main", boundaryRatchetVerb, queueApplyVerb, "quality", boundaryLogVerb, "loop"}, verb) {
			return 0
		}
		return dispatchBoundaryVerb(verb, args, stdout, stderr)
	}
	var stdout, stderr bytes.Buffer

	rc := runBoundarySteps(dispatch, steps, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("boundary run rc=%d, want a completed boundary with a policy key only the rebuilt binary reads\nstderr=%s", rc, stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, "go", "plane.log"))
	if err != nil {
		t.Fatalf("the rebuilt binary was never run: %v", err)
	}
	for _, want := range []string{"inbox curate --apply", "selfheal incidents", "selfheal propose", "gc ", "code refresh"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the rebuilt binary never ran %q; the log is\n%s", want, raw)
		}
	}
}

func TestBoundaryMergedPolicy_TheOperationsDocRecordsThatTheStepsThatReadThePolicyRunAfterTheBuildInBothPlans(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "operations", "wave-operations.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	if strings.Contains(doc, "the boundary has no build step") {
		t.Errorf("the doc still says that boundary run has no build step")
	}
	var recorded bool
	for _, block := range strings.Split(doc, "\n\n") {
		build := strings.Index(block, "build")
		after := true
		for _, verb := range []string{"inbox curate", "selfheal", "gc"} {
			at := strings.Index(block, verb)
			after = after && build >= 0 && build < at
		}
		recorded = recorded || after
	}
	if !recorded {
		t.Errorf("no paragraph of wave-operations.md names the build before inbox curate, selfheal and gc")
	}
}
