package contextfillcorrelate

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
)

// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) along the one path a consumer takes:
// Load the real corpus shape off disk, Correlate it into a Report, read the
// Bucket / VerdictSplit tallies back out, round-trip the report through the
// Bucket JSON codec the CLI's --json projection depends on, and render Markdown.
func TestAPICoverNamedExports(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "knowledge-base", "cycles", "cycle-11.json"),
		`{"cycle":11,"final_verdict":"FAIL"}`)
	write(t, filepath.Join(root, ".evolve", "runs", "cycle-11", "phase-timing.json"),
		`[{"phase":"build","duration_ms":1,"verdict":"FAIL","attempt_count":1,"context_fill_ratio":0.94}]`)
	write(t, filepath.Join(root, "knowledge-base", "cycles", "cycle-12.json"),
		`{"cycle":12,"final_verdict":"PASS"}`)

	rows, err := Load(root)
	if err != nil {
		t.Fatalf("Load(%s) returned error %v, want nil", root, err)
	}
	if len(rows) != 2 {
		t.Fatalf("Load returned %d rows, want one CycleFill per dossier (2)", len(rows))
	}
	var hotRow CycleFill
	for _, r := range rows {
		if r.Cycle == 11 {
			hotRow = r
		}
	}
	if hotRow.Verdict != "FAIL" || len(hotRow.Fills) != 1 || hotRow.Fills[0] != 0.94 {
		t.Fatalf("CycleFill for cycle 11 = %+v, want Verdict FAIL and Fills [0.94]", hotRow)
	}

	rep := Correlate(rows)
	if rep.CyclesJoined != 1 {
		t.Errorf("Report.CyclesJoined = %d, want 1 (cycle 12 has no timing log)", rep.CyclesJoined)
	}
	if len(rep.NoData) != 1 || rep.NoData[0] != 12 {
		t.Errorf("Report.NoData = %v, want [12]", rep.NoData)
	}

	var hotSplit VerdictSplit = rep.Hot
	if hotSplit.Cycles != 1 || hotSplit.Fail != 1 || hotSplit.FailRate != 1 {
		t.Errorf("Report.Hot = %+v, want {Cycles:1 Fail:1 FailRate:1}", hotSplit)
	}
	if rep.Cold.Cycles != 0 || rep.Cold.FailRate != 0 {
		t.Errorf("Report.Cold = %+v, want an empty split with a finite 0 rate", rep.Cold)
	}

	top := rep.Buckets[len(rep.Buckets)-1]
	var bucket Bucket = top
	if bucket.Min != contextfill.HotThreshold || !math.IsInf(bucket.Max, 1) {
		t.Errorf("top Bucket = {Min:%v Max:%v}, want {HotThreshold +Inf}", bucket.Min, bucket.Max)
	}
	if bucket.Label == "" || bucket.Cycles != 1 || bucket.Fail != 1 || bucket.Pass != 0 || bucket.Warn != 0 || bucket.FailRate != 1 {
		t.Errorf("top Bucket = %+v, want the single hot FAIL cycle tallied", bucket)
	}

	// Bucket.MarshalJSON / Bucket.UnmarshalJSON: the open-ended Max survives the
	// round trip as +Inf via a JSON null.
	encoded, err := bucket.MarshalJSON()
	if err != nil {
		t.Fatalf("Bucket.MarshalJSON returned error %v, want nil", err)
	}
	if !strings.Contains(string(encoded), `"max":null`) {
		t.Errorf("Bucket.MarshalJSON = %s, want the open-ended Max encoded as null", encoded)
	}
	var decoded Bucket
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("Bucket.UnmarshalJSON returned error %v, want nil", err)
	}
	if !math.IsInf(decoded.Max, 1) || decoded.Fail != bucket.Fail {
		t.Errorf("Bucket.UnmarshalJSON = %+v, want Max +Inf and Fail %d", decoded, bucket.Fail)
	}

	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("json.Marshal(Report) returned error %v, want nil (+Inf must encode as null)", err)
	}
	var back Report
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("json.Unmarshal(Report) returned error %v, want nil", err)
	}
	if !math.IsInf(back.Buckets[len(back.Buckets)-1].Max, 1) {
		t.Errorf("round-tripped top Bucket.Max = %v, want +Inf", back.Buckets[len(back.Buckets)-1].Max)
	}

	md := Markdown(rep)
	if !strings.Contains(md, "fail rate") {
		t.Errorf("Markdown(rep) = %q, want a bucket table with a fail-rate column", md)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
