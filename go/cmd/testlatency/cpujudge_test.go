package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func baselines(goos string, cpus map[string]float64) *ratchetFile {
	rf := &ratchetFile{Packages: map[string]record{}}
	for name, cpu := range cpus {
		rf.Packages[name] = record{Class: "small", P90: 1, CPU: map[string]float64{goos: cpu}}
	}
	return rf
}

func TestJudgeCPU_FailsOnlyPastBothTheRatioAndTheFloor(t *testing.T) {
	refs := map[string]float64{"r0": 20, "r1": 20, "r2": 20}
	cases := []struct {
		name       string
		base, run  float64
		wantFailed bool
	}{
		{"under the ratio", 100, 134, false},
		{"over the ratio and the floor", 100, 136, true},
		{"over the ratio, under the floor", 10, 24, false},
		{"over the floor, under the ratio", 100, 114, false},
	}
	for _, c := range cases {
		base := map[string]float64{"t": c.base}
		usage := map[string]float64{"t": c.run, "r0": 20, "r1": 20, "r2": 20}
		for k, v := range refs {
			base[k] = v
		}
		var got bool
		for _, v := range baselines("linux", base).judgeCPU(usage, "linux") {
			if v.Pkg == "t" {
				got = v.failed()
			}
		}
		if got != c.wantFailed {
			t.Errorf("%s: failed = %v, want %v", c.name, got, c.wantFailed)
		}
	}
}

func TestJudgeCPU_FewerThanThreeReferencesAreReportedOnly(t *testing.T) {
	rf := baselines("linux", map[string]float64{"t": 100, "r0": 20, "r1": 20})
	var out strings.Builder
	verdicts := rf.judgeCPU(map[string]float64{"t": 900, "r0": 20, "r1": 20}, "linux")
	writeCPUSection(&out, verdicts)
	if len(cpuFailures(verdicts)) != 0 || !strings.Contains(out.String(), "reported only: 2 reference packages") {
		t.Errorf("want a report with no failure, got failures %v and\n%s", cpuFailures(verdicts), out.String())
	}
}

func TestJudgeCPU_IgnoresABaselineOfAnotherOS(t *testing.T) {
	rf := baselines("darwin", map[string]float64{"t": 100})
	if got := rf.judgeCPU(map[string]float64{"t": 900}, "linux"); len(got) != 0 {
		t.Errorf("verdicts = %v, want none", got)
	}
}

func TestLoadRusage_SumsAPackageAndRejectsABrokenLine(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.ndjson")
	body := `{"package":"github.com/mickeyyaya/evolve-loop/go/internal/a","user_s":1,"system_s":2}` + "\n\n" + `{"package":"internal/a","user_s":0.5,"system_s":0.5}` + "\n"
	if err := os.WriteFile(good, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cpu, err := loadRusage(good)
	if err != nil || cpu["internal/a"] != 4 {
		t.Errorf("cpu = %v err = %v, want internal/a = 4", cpu, err)
	}
	bad := filepath.Join(dir, "bad.ndjson")
	if err := os.WriteFile(bad, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRusage(bad); err == nil || !strings.Contains(err.Error(), "bad.ndjson: line 1") {
		t.Errorf("err = %v, want one naming bad.ndjson line 1", err)
	}
}

func TestTighten_SeedsAndLowersCPUNeverRaisesAndSkipsUnlisted(t *testing.T) {
	rf := baselines(runtime.GOOS, map[string]float64{"low": 50})
	rf.Packages["fresh"] = record{Class: "small", P90: 1}
	usage := map[string]float64{"low": 40, "fresh": 7, "ghost": 3}
	if !rf.tighten(&Report{}, usage, runtime.GOOS) {
		t.Fatal("tighten reported no change")
	}
	if rf.Packages["low"].CPU[runtime.GOOS] != 40 || rf.Packages["fresh"].CPU[runtime.GOOS] != 7 {
		t.Errorf("packages = %+v", rf.Packages)
	}
	if _, ok := rf.Packages["ghost"]; ok {
		t.Error("tighten added a package the file does not list")
	}
	if rf.tighten(&Report{}, map[string]float64{"low": 90}, runtime.GOOS) {
		t.Error("a slower run raised or changed a baseline")
	}
}

func TestTighten_BadRusageExitsTwo(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"p":{"class":"small","p90":50}}}`)
	code, _, errOut := runWith("", "tighten", "-ratchet", path, "-rusage", filepath.Join(t.TempDir(), "absent.ndjson"))
	if code != 2 || !strings.Contains(errOut, "absent.ndjson") {
		t.Errorf("exit %d stderr %q, want 2 naming absent.ndjson", code, errOut)
	}
}

func TestMedian_AveragesTheTwoMiddleValuesOfAnEvenCount(t *testing.T) {
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median = %v, want 2.5", got)
	}
	if got := median([]float64{5, 1, 3}); got != 3 {
		t.Errorf("median = %v, want 3", got)
	}
}

func TestCPUSection_NamesAFailureAndCountsWhenNothingFailed(t *testing.T) {
	base := map[string]float64{"t": 100, "r0": 20, "r1": 20, "r2": 20}
	rf := baselines("linux", base)
	failing := rf.judgeCPU(map[string]float64{"t": 200, "r0": 20, "r1": 20, "r2": 20}, "linux")
	var out strings.Builder
	writeCPUSection(&out, failing)
	failures := cpuFailures(failing)
	if len(failures) != 1 || !strings.Contains(failures[0], "t used 200.00s of CPU") || !strings.Contains(out.String(), "| t | 100.00 | 200.00 | 1.00 | FAIL: 200.00s after the index |") {
		t.Errorf("failures %v\n%s", failures, out.String())
	}
	out.Reset()
	writeCPUSection(&out, rf.judgeCPU(map[string]float64{"t": 100, "r0": 20, "r1": 20, "r2": 20}, "linux"))
	if !strings.Contains(out.String(), "_(none)_ of 4 packages with a baseline") {
		t.Errorf("section:\n%s", out.String())
	}
}

func TestLoadRusage_ReportsAReadError(t *testing.T) {
	if _, err := loadRusage(t.TempDir()); err == nil {
		t.Error("loadRusage read a directory without an error")
	}
}
