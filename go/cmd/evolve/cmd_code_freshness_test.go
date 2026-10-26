package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
)

func staleIndexOf(behind int, refuse bool, backends ...codeindex.Backend) codeBackendsBuilder {
	return func(string) (codeIndexHandle, error) {
		fresh := func(context.Context) (codeIndexFreshness, error) {
			return codeIndexFreshness{Head: "1111111111111111", Main: "2222222222222222", Behind: behind}, nil
		}
		return codeIndexHandle{backends: codeindex.NewBackends(backends...), freshness: fresh, refuseStale: refuse}, nil
	}
}

func indexWithFreshnessError(err error, backends ...codeindex.Backend) codeBackendsBuilder {
	return func(string) (codeIndexHandle, error) {
		fresh := func(context.Context) (codeIndexFreshness, error) { return codeIndexFreshness{}, err }
		return codeIndexHandle{backends: codeindex.NewBackends(backends...), freshness: fresh}, nil
	}
}

func findFake() codeindex.Backend {
	return codeindextest.NewFake("fake", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{
		{Query: codeindex.QueryFind, Arg: "q"}: {{File: "a.go", Line: 7, Symbol: "s", Text: "t"}},
	})
}

func TestCodeVerb_AStaleIndexWarnsOnStderrWithTheCountAndStillAnswers(t *testing.T) {
	t.Parallel()

	rc, stdout, stderr := runCode(t, staleIndexOf(114, false, findFake()), "find", "q")

	want := "evolve code: WARN: the code index is 114 commit(s) behind origin/main (index 111111111111, origin/main 222222222222); an answer can miss newer code; evolve code refresh moves the index\n"
	if rc != 0 || stdout != "a.go:7\ts\tt\n" || stderr != want {
		t.Errorf("rc=%d stdout=%q stderr=%q, want the hit and %q", rc, stdout, stderr, want)
	}
}

func TestCodeVerb_AFreshIndexPrintsNoWarning(t *testing.T) {
	t.Parallel()

	rc, _, stderr := runCode(t, codeIndexOf(findFake()), "find", "q")

	if rc != 0 || stderr != "" {
		t.Errorf("rc=%d stderr=%q, want no warning", rc, stderr)
	}
}

func TestCodeVerb_AStaleIndexUnderTheRefusePolicyExitsTwentyOneBeforeTheQuery(t *testing.T) {
	t.Parallel()

	rc, stdout, stderr := runCode(t, staleIndexOf(3, true, findFake()), "find", "q", "--json")

	e := decodeCodeEnvelope(t, stdout)
	failure, _ := e["error"].(map[string]any)
	if rc != codeExitStale || e["data"] != nil || failure["kind"] != "stale" || failure["code"] != 21.0 {
		t.Errorf("rc=%d envelope=%v, want exit 21 and no hits", rc, e)
	}
	if !strings.Contains(stderr, "evolve code find: refused: the code index is 3 commit(s) behind origin/main") || !strings.Contains(stderr, "code_index.stale is refuse") || strings.Contains(stderr, "WARN") {
		t.Errorf("stderr = %q, want one refusal line", stderr)
	}
}

func TestCodeVerb_JSONCarriesTheHitsAndTheFreshness(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		build        codeBackendsBuilder
		stale        bool
		behind       float64
		wantWarnings int
	}{
		"fresh": {codeIndexOf(findFake()), false, 0, 0},
		"stale": {staleIndexOf(5, false, findFake()), true, 5, 1},
	} {
		rc, stdout, _ := runCode(t, tc.build, "find", "q", "--json")

		e := decodeCodeEnvelope(t, stdout)
		data, _ := e["data"].(map[string]any)
		hits, _ := data["hits"].([]any)
		warnings, _ := e["warnings"].([]any)
		if rc != 0 || e["schema"] != "evolve.code.find/1" || e["ok"] != true || data["stale"] != tc.stale || data["behind"] != tc.behind || len(hits) != 1 || len(warnings) != tc.wantWarnings {
			t.Errorf("%s: rc=%d envelope=%v", name, rc, e)
		}
		if hit, _ := hits[0].(map[string]any); hit["file"] != "a.go" || hit["line"] != 7.0 || hit["symbol"] != "s" || hit["text"] != "t" {
			t.Errorf("%s: hit = %v", name, hits[0])
		}
	}
}

func TestCodeVerb_AnUnknownFreshnessWarnsAndLeavesTheFieldsOut(t *testing.T) {
	t.Parallel()

	rc, stdout, stderr := runCode(t, indexWithFreshnessError(errors.New("git rev-parse: exit 128"), findFake()), "find", "q", "--json")

	data, _ := decodeCodeEnvelope(t, stdout)["data"].(map[string]any)
	_, hasStale := data["stale"]
	_, hasBehind := data["behind"]
	if rc != 0 || hasStale || hasBehind || !strings.Contains(stderr, "WARN: the freshness of the code index is unknown: git rev-parse: exit 128") {
		t.Errorf("rc=%d data=%v stderr=%q", rc, data, stderr)
	}
}

func TestCodeVerb_AnAbsentIndexGivesNoFreshnessWarning(t *testing.T) {
	t.Parallel()
	absent := fmt.Errorf("%w: /abs/index does not exist", errCodeIndexAbsent)

	rc, _, stderr := runCode(t, indexWithFreshnessError(absent, findFake()), "find", "q")

	if rc != 0 || stderr != "" {
		t.Errorf("rc=%d stderr=%q, want the backend alone to speak for an absent index", rc, stderr)
	}
}

func TestCodeVerb_JSONGivesAnErrorEnvelopeForEachFailure(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		build codeBackendsBuilder
		args  []string
		rc    int
		kind  string
	}{
		"no result":   {codeIndexOf(findFake()), []string{"find", "other", "--json"}, codeExitNotFound, "not_found"},
		"usage":       {codeIndexOf(findFake()), []string{"api", "--json"}, codeExitUsage, "usage"},
		"no verb":     {codeIndexOf(findFake()), []string{"--json"}, codeExitUsage, "usage"},
		"build fails": {func(string) (codeIndexHandle, error) { return codeIndexHandle{}, errors.New("bad policy") }, []string{"map", "--json"}, codeExitFailed, "failed"},
	} {
		rc, stdout, stderr := runCode(t, tc.build, tc.args...)

		e := decodeCodeEnvelope(t, stdout)
		failure, _ := e["error"].(map[string]any)
		if rc != tc.rc || e["ok"] != false || failure["kind"] != tc.kind || failure["code"] != float64(tc.rc) || stderr == "" || strings.Contains(failure["message"].(string), "\n") {
			t.Errorf("%s: rc=%d envelope=%v stderr=%q", name, rc, e, stderr)
		}
	}
}

func TestCodeVerb_TheUsageEnvelopeOfAnUnknownVerbNamesTheQuerySchema(t *testing.T) {
	t.Parallel()

	_, stdout, _ := runCode(t, codeIndexOf(findFake()), "teleport", "--json")

	if e := decodeCodeEnvelope(t, stdout); e["schema"] != "evolve.code.query/1" {
		t.Errorf("schema = %v", e["schema"])
	}
}

func TestCodeEnvelopeWriter_AWriteFailureIsReportedOnStderr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	w := codeEnvelopeWriter{schema: "s/1", json: true, stdout: &failingWriter{}, stderr: &stderr}

	rc := w.ok(nil)

	if rc != 0 || !strings.Contains(stderr.String(), "write the JSON envelope: ") {
		t.Errorf("rc=%d stderr=%q", rc, stderr.String())
	}
}

func TestCodeIndexHostFreshness_ComparesTheIndexHeadWithTheLocalOriginMainRef(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		runner    *refreshRunner
		want      codeIndexFreshness
		wantCalls []string
	}{
		"at main": {&refreshRunner{heads: "abc\nabc\n"}, codeIndexFreshness{Head: "abc", Main: "abc"}, []string{"git rev-parse HEAD origin/main"}},
		"behind":  {&refreshRunner{behind: "114\n"}, codeIndexFreshness{Head: "1111111111111111", Main: "2222222222222222", Behind: 114}, []string{"git rev-parse HEAD origin/main", "git rev-list --count HEAD..origin/main"}},
	} {
		got, err := refreshHost(tc.runner).freshness(context.Background(), "/abs/index")

		if err != nil || got != tc.want || !slices.Equal(tc.runner.argvs(), tc.wantCalls) {
			t.Errorf("%s: got=%+v err=%v argvs=%q, want %+v after %q", name, got, err, tc.runner.argvs(), tc.want, tc.wantCalls)
		}
		if got.stale() == (name == "at main") {
			t.Errorf("%s: stale() = %v", name, got.stale())
		}
	}
}

func TestCodeIndexHostFreshness_AnAbsentIndexRunsNoGit(t *testing.T) {
	t.Parallel()
	rr := &refreshRunner{}
	host := refreshHost(rr)
	host.stat = absentStat

	_, err := host.freshness(context.Background(), "/abs/index")

	if !errors.Is(err, errCodeIndexAbsent) || len(rr.calls) != 0 {
		t.Errorf("err=%v calls=%v", err, rr.argvs())
	}
}

func TestBuildCodeIndex_ReadsTheStalePolicyAndProbesTheConfiguredIndex(t *testing.T) {
	t.Parallel()
	for mode, refuse := range map[string]bool{"warn": false, "refuse": true} {
		root := codePolicyRoot(t, map[string]string{"stale": fmt.Sprintf("%q", mode)})
		rr := &refreshRunner{}

		idx, err := buildCodeIndex(root, refreshHost(rr))
		if err != nil {
			t.Fatalf("%s: buildCodeIndex: %v", mode, err)
		}
		f, err := idx.freshness(context.Background())

		if idx.refuseStale != refuse || err != nil || f.Behind != 3 || rr.calls[0].dir != "/abs/index" {
			t.Errorf("%s: refuse=%v f=%+v err=%v calls=%v", mode, idx.refuseStale, f, err, rr.calls)
		}
	}
}

func TestBuildCodeIndex_AnUnknownBackendFailsBeforeAnyProbe(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, map[string]string{"backends": `["nope"]`})
	rr := &refreshRunner{}

	_, err := buildCodeIndex(root, refreshHost(rr))

	if err == nil || !strings.Contains(err.Error(), `unknown backend "nope"`) || len(rr.calls) != 0 {
		t.Errorf("err=%v calls=%v", err, rr.argvs())
	}
}

func TestCodeVerb_AnIndexAheadOfMainWarnsWithoutACountOfMissingCommits(t *testing.T) {
	t.Parallel()

	rc, _, stderr := runCode(t, staleIndexOf(0, false, findFake()), "find", "q")

	if rc != 0 || !strings.Contains(stderr, "WARN: the code index is not at origin/main but misses none of its commits") || strings.Contains(stderr, "0 commit(s)") {
		t.Errorf("rc=%d stderr=%q", rc, stderr)
	}
}
