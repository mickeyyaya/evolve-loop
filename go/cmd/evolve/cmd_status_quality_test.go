package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
)

func floatPtr(v float64) *float64 { return &v }

func TestWriteStatusQuality_RendersReportAndAllStages(t *testing.T) {
	t.Parallel()
	var empty bytes.Buffer
	writeStatusQuality(&empty, nil, "report")
	if empty.Len() != 0 {
		t.Fatalf("empty readings printed %q", empty.String())
	}

	readings := []dashboard.QualityReading{
		{Index: "CQ1", HasReading: false, Stage: "report"},
		{Index: "CQ2", HasReading: true, Value: floatPtr(2.5), Lo: floatPtr(2.0), Hi: floatPtr(3.0), N: 119, Window: "waves:82-140", Stage: "report", Status: "ok"},
		{Index: "CQ3", HasReading: true, Value: floatPtr(7.5), Lo: floatPtr(6.5), Hi: floatPtr(8.5), N: 40, Window: "tree:main", Stage: "shadow", Status: "ok"},
		{Index: "PQ1", HasReading: true, Value: floatPtr(0.424), Lo: floatPtr(0.312), Hi: floatPtr(0.544), N: 66, Window: "waves:131-140", Stage: "report", Status: "ok"},
		{Index: "JQ3", HasReading: true, Value: floatPtr(0.6), Lo: floatPtr(0.55), Hi: floatPtr(0.7), N: 98, Window: "reviews:all", Stage: "report", Status: "breach"},
		{Index: "OQ2", HasReading: true, Value: floatPtr(0.09), N: 274, Window: "commits:14-30d", Stage: "report", Status: "ok"},
		{Index: "ZZ9", HasReading: true, Value: floatPtr(1.0), N: 1, Window: "w", Stage: "report", Status: "ok"},
	}

	var reportOut bytes.Buffer
	writeStatusQuality(&reportOut, readings, "report")
	rep := reportOut.String()
	for _, want := range []string{
		"Quality",
		"CQ1", "no reading yet",
		"CQ2", "2.5 [2.0, 3.0]", "n=119", "waves:82-140",
		"PQ1", "42.4% [31.2, 54.4]", "n=66",
		"JQ3", "60.0% [55.0, 70.0]", "!",
		"OQ2", "9.0%", "n=274",
	} {
		if !strings.Contains(rep, want) {
			t.Errorf("report output lacks %q:\n%s", want, rep)
		}
	}
	if strings.Contains(rep, "CQ3") {
		t.Errorf("report mode printed shadow index CQ3:\n%s", rep)
	}

	var allOut bytes.Buffer
	writeStatusQuality(&allOut, readings, "all")
	if !strings.Contains(allOut.String(), "CQ3") || !strings.Contains(allOut.String(), "7.5 [6.5, 8.5]") || !strings.Contains(allOut.String(), "shadow") {
		t.Errorf("all mode lacks shadow CQ3:\n%s", allOut.String())
	}
}

func TestRunStatus_QualityFlagAndJSONSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if rc := runStatus([]string{"--quality", "bogus", "--project-root", root}, nil, &out, &errb); rc != cliExitUsage || out.Len() != 0 {
		t.Fatalf("bogus --quality: rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}

	errb.Reset()
	if rc := runStatus([]string{"--quality", "report", "--project-root", root}, nil, &out, &errb); rc != 0 {
		t.Fatalf("report --quality: rc=%d err=%q", rc, errb.String())
	}
	if !strings.Contains(out.String(), "Quality") || !strings.Contains(out.String(), "no reading yet") {
		t.Errorf("missing Quality block:\n%s", out.String())
	}

	out.Reset()
	errb.Reset()
	if rc := runStatus([]string{"--quality", "all", "--project-root", root}, nil, &out, &errb); rc != 0 {
		t.Fatalf("all --quality: rc=%d err=%q", rc, errb.String())
	}
	if !strings.Contains(out.String(), "CQ3") {
		t.Errorf("--quality all missing shadow CQ3:\n%s", out.String())
	}

	out.Reset()
	errb.Reset()
	if rc := runStatus([]string{"--json", "--project-root", root}, nil, &out, &errb); rc != 0 {
		t.Fatalf("--json: rc=%d err=%q", rc, errb.String())
	}
	var doc struct {
		Quality []dashboard.QualityReading `json:"quality"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Quality) == 0 {
		t.Fatalf("json quality: err=%v len=%d", err, len(doc.Quality))
	}
}
