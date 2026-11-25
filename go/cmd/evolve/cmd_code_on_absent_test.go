package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func gotypesTrailPolicy(t *testing.T, extra map[string]string) string {
	t.Helper()
	block := map[string]string{"backends": `["gotypes","trail"]`}
	for k, v := range extra {
		block[k] = v
	}
	return codePolicyRoot(t, block)
}

func callersRequest() codeindex.Request {
	return codeindex.Request{Query: codeindex.QueryCallers, Arg: "a", Direction: codeindex.DirectionIn}
}

func onAbsentIndexOf(root string, rec *runRecord, refuseStale bool, behind int) codeBackendsBuilder {
	return func(string) (codeIndexHandle, error) {
		set, err := buildCodeBackends(root, rec.run, []string{"PATH=/usr/bin"})
		fresh := func(context.Context) (codeIndexFreshness, error) {
			return codeIndexFreshness{Head: "1111111111111111", Main: "2222222222222222", Behind: behind}, nil
		}
		return codeIndexHandle{backends: set, freshness: fresh, refuseStale: refuseStale}, err
	}
}

func TestCodeOnAbsentWiring_TheNextPolicyRoutesAnAbsentGotypesIndexToTrail(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, map[string]string{"on_absent": `"next"`})
	rec := &runRecord{}
	set, err := buildCodeBackends(root, rec.run, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("buildCodeBackends: %v", err)
	}

	hits, served, err := set.SearchServed(context.Background(), callersRequest())

	if err != nil || served != "trail" || len(hits) == 0 || rec.name != "/abs/trail" {
		t.Errorf("hits=%v served=%q err=%v ran=%q; want trail to answer", hits, served, err, rec.name)
	}
}

func TestCodeOnAbsentWiring_TheDefaultPolicyRefusesAnAbsentGotypesIndex(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, nil)
	rec := &runRecord{}
	set, err := buildCodeBackends(root, rec.run, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("buildCodeBackends: %v", err)
	}

	hits, served, err := set.SearchServed(context.Background(), callersRequest())

	if !errors.Is(err, codeindex.ErrAbsent) || served != "gotypes" || len(hits) != 0 || rec.name != "" {
		t.Errorf("hits=%v served=%q err=%v ran=%q; want ErrAbsent from gotypes and trail never run", hits, served, err, rec.name)
	}
}

func TestCodeOnAbsentWiring_TheRefusePolicyWrittenOutRefusesLikeTheDefault(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, map[string]string{"on_absent": `"refuse"`})
	rec := &runRecord{}
	set, err := buildCodeBackends(root, rec.run, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("buildCodeBackends: %v", err)
	}

	_, served, err := set.SearchServed(context.Background(), callersRequest())

	if !errors.Is(err, codeindex.ErrAbsent) || served != "gotypes" || rec.name != "" {
		t.Errorf("served=%q err=%v ran=%q; want ErrAbsent from gotypes", served, err, rec.name)
	}
}

func TestCodeOnAbsentWiring_AnUnknownModeFailsTheBuildNamingTheKey(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, map[string]string{"on_absent": `"failover"`})

	_, err := buildCodeBackends(root, (&runRecord{}).run, nil)

	if !errors.Is(err, policy.ErrCodeIndexOnAbsent) {
		t.Errorf("err = %v, want policy.ErrCodeIndexOnAbsent", err)
	}
}

func TestCodeOnAbsentWiring_TheCommandAnswersFromTrailAndLogsTheBackendThatAnswered(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, map[string]string{"on_absent": `"next"`})
	sink, path := userSink(t, nil)

	rc, stdout, _ := runLogged(t, sink, onAbsentIndexOf(root, &runRecord{}, false, 0), "callers", "a", "--project-root", root)

	records := readLog(t, path)
	if rc != 0 || !strings.Contains(stdout, "b.go") || len(records) != 1 || records[0].Backend != "trail" {
		t.Errorf("rc=%d stdout=%q records=%+v; want exit 0 and backend trail", rc, stdout, records)
	}
}

func TestCodeOnAbsentWiring_TheDefaultCommandExitsTwoNamingGotypes(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, nil)
	sink, path := userSink(t, nil)

	rc, _, _ := runLogged(t, sink, onAbsentIndexOf(root, &runRecord{}, false, 0), "callers", "a", "--project-root", root)

	records := readLog(t, path)
	if rc != codeExitAbsent || len(records) != 1 || records[0].Backend != "gotypes" {
		t.Errorf("rc=%d records=%+v; want exit 2 and backend gotypes", rc, records)
	}
}

func TestCodeOnAbsentWiring_AStaleIndexUnderRefuseStillExitsTwentyOneWhenOnAbsentIsNext(t *testing.T) {
	t.Parallel()
	root := gotypesTrailPolicy(t, map[string]string{"on_absent": `"next"`, "stale": `"refuse"`})
	rec := &runRecord{}
	sink, _ := userSink(t, nil)

	rc, _, _ := runLogged(t, sink, onAbsentIndexOf(root, rec, true, 3), "callers", "a", "--project-root", root)

	if rc != codeExitStale || rec.name != "" {
		t.Errorf("rc=%d ran=%q; want exit 21 before any backend runs", rc, rec.name)
	}
}
