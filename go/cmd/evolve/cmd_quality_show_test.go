package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func showOnlyQualityEnv(t *testing.T, readers ...qualityReader) qualityEnv {
	t.Helper()
	return qualityEnv{scopes: map[string][]qualityReader{"wave": readers, "tree": nil}, newCenter: func(string, string, io.Writer) (*signalcenter.Center, func()) {
		t.Fatal("quality show opened a Signal Center")
		return nil, nil
	}}
}

func TestQualityShow_PrintsTheReadingsAndPublishesNothing(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	before := treeDigest(t, root)
	reading := qualitypanel.Reading{Index: "PQ1", Interval: qualitypanel.Wilson(3, 5), Method: qualitypanel.MethodWilson95, N: 5, Window: "waves:140"}
	env := showOnlyQualityEnv(t, readerOf("PQ1", []qualitypanel.Reading{reading}, nil), readerOf("CQ1", nil, errors.New("bad tree")))
	var stdout, stderr bytes.Buffer
	if rc := runQualityWith(env, []string{"show", "--scope", "wave", "--json", "--project-root", root}, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	var got qualityShowResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not one result: %v\n%s", err, stdout.String())
	}
	if got.Scope != "wave" || got.Readings != 1 || got.Warnings != 1 || len(got.Events) != 2 || got.Events[0].Fields["value"] != "0.600" {
		t.Errorf("result = %+v, want one reading at 0.600 and one source warning", got)
	}
	if after := treeDigest(t, root); after != before {
		t.Error("quality show changed the tree of the project root")
	}
	if events := channelEvents(t, root, "quality.reading"); len(events) != 0 {
		t.Errorf("the reading channel holds %d events, want none", len(events))
	}
}

func TestQualityShow_TheTextFormPrintsOneLineForEachEvent(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	reading := qualitypanel.Reading{Index: "PQ1", Interval: qualitypanel.Wilson(3, 5), Method: qualitypanel.MethodWilson95, N: 5}
	var stdout, stderr bytes.Buffer
	if rc := runQualityWith(showOnlyQualityEnv(t, readerOf("PQ1", []qualitypanel.Reading{reading}, nil)), []string{"show", "--scope", "wave", "--project-root", root}, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	if out := stdout.String(); !strings.Contains(out, string(signalcenter.KindQualityReading)) || !strings.Contains(out, "quality show: scope wave: 1 reading(s), 0 breach(es), 0 warning(s)") {
		t.Errorf("stdout = %q", out)
	}
}

func TestQualityShow_AWrongCallIsAUsageError(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	for _, args := range [][]string{
		{"show"},
		{"show", "--scope", "bogus", "--project-root", root},
		{"show", "--scope", "wave"},
		{"show", "--scope", "wave", "extra", "--project-root", root},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runQualityWith(showOnlyQualityEnv(t), args, &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), "quality show") {
			t.Errorf("%v: rc = %d stderr = %q, want 10 and the usage", args, rc, stderr.String())
		}
	}
}

func TestQualityReport_JSONPrintsTheTallyOfThePublishedEvents(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	reading := qualitypanel.Reading{Index: "PQ1", Interval: qualitypanel.Wilson(3, 5), Method: qualitypanel.MethodWilson95, N: 5}
	var stdout, stderr bytes.Buffer
	if rc := runQualityWith(fakeQualityEnv(readerOf("PQ1", []qualitypanel.Reading{reading}, nil)), []string{"report", "--scope", "wave", "--json", "--project-root", root}, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	var got qualityReportResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got != (qualityReportResult{Scope: "wave", Readings: 1}) {
		t.Errorf("result = %+v (%v), want one reading of scope wave", got, err)
	}
	if events := channelEvents(t, root, "quality.reading"); len(events) != 1 {
		t.Errorf("the reading channel holds %d events, want the one published reading", len(events))
	}
}

func TestQualityCompare_JSONPrintsTheComparisonAndPublishesItsReading(t *testing.T) {
	t.Parallel()
	root := pipelineProject(t)
	var stdout, stderr bytes.Buffer
	if rc := runQualityWith(fakeQualityEnv(), []string{"compare", "--index", "PQ1", "--a", "wave:1", "--b", "wave:2", "--json", "--project-root", root}, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	var got qualityCompareResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got.Index != "PQ1" || got.A.Window != "wave:1" || got.Paired != nil {
		t.Errorf("result = %+v (%v), want the PQ1 comparison of wave:1 and wave:2", got, err)
	}
	if events := channelEvents(t, root, "quality.reading"); len(events) != 1 {
		t.Errorf("the reading channel holds %d events, want the one PQ3 reading", len(events))
	}
}
