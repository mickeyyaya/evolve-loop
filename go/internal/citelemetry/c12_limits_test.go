package citelemetry

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

const (
	c12MacJobs = `{"jobs":[
{"name":"macOS job","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:30:00Z","steps":[
 {"name":"test darwin","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:29:00Z"}]},
{"name":"ubuntu 1.23","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:10:00Z","steps":[
 {"name":"test race","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:09:00Z"}]}]}`

	c12MacStream = `{"Time":"2026-10-20T10:01:00Z","Action":"start","Package":"example.com/m/go/internal/core"}
{"Time":"2026-10-20T10:02:00Z","Action":"pass","Package":"example.com/m/go/internal/core","Test":"TestSerial","Elapsed":60}
{"Time":"2026-10-20T10:02:01Z","Action":"pause","Package":"example.com/m/go/internal/core","Test":"TestPar"}
{"Time":"2026-10-20T10:02:33Z","Action":"pass","Package":"example.com/m/go/internal/core","Test":"TestPar","Elapsed":32}
{"Time":"2026-10-20T10:07:02Z","Action":"pass","Package":"example.com/m/go/internal/core","Elapsed":362}
{"Time":"2026-10-20T10:08:00Z","Action":"start","Package":"example.com/m/go/cmd/small"}
{"Time":"2026-10-20T10:08:00Z","Action":"pass","Package":"example.com/m/go/cmd/small","Test":"TestSmall","Elapsed":0.5}
{"Time":"2026-10-20T10:08:01Z","Action":"pass","Package":"example.com/m/go/cmd/small","Elapsed":0.6}
`
	c12MacRusage = `{"package":"internal/core","wall_s":362,"user_s":300,"system_s":20,"peak_rss_mb":400,"exit_code":0}
{"package":"cmd/small","wall_s":0.6,"user_s":0.3,"system_s":0.1,"peak_rss_mb":20,"exit_code":0}
`
	c12MacSamples = `{"ts":"2026-10-20T10:03:00Z","cpu_busy_pct":70,"swap_used_mb":0}
{"ts":"2026-10-20T10:05:00Z","cpu_busy_pct":75,"swap_used_mb":0}
`

	c12UbuntuJobs = `{"jobs":[
{"name":"ubuntu 1.27","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:20:00Z","steps":[
 {"name":"test race","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:19:00Z"}]}]}`

	c12UbuntuStream = `{"Time":"2026-10-20T10:01:00Z","Action":"start","Package":"example.com/m/go/internal/busy"}
{"Time":"2026-10-20T10:01:05Z","Action":"pass","Package":"example.com/m/go/internal/busy","Test":"TestBusy","Elapsed":5}
{"Time":"2026-10-20T10:01:10Z","Action":"pass","Package":"example.com/m/go/internal/busy","Elapsed":10}
`
	c12UbuntuRusage = `{"package":"internal/busy","wall_s":10,"user_s":9,"system_s":0.5,"peak_rss_mb":100,"exit_code":0}
`
	c12UbuntuSamples = `{"ts":"2026-10-20T10:01:02Z","cpu_busy_pct":95,"runqueue":3,"swap_used_mb":0}
{"ts":"2026-10-20T10:01:08Z","cpu_busy_pct":96,"runqueue":3,"swap_used_mb":0}
`
)

func c12Report(t *testing.T, jobsDoc, binding, stream, usage, samples string) Report {
	t.Helper()
	jobs := writeDir(t, map[string]string{"jobs.json": jobsDoc})
	art := writeDir(t, map[string]string{testJSONFile: stream, rusageFile: usage, samplesFile: samples})
	rep, err := Run(filepath.Join(jobs, "jobs.json"), []string{binding + "=" + art})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return rep
}

func limitByPackage(rep Report) map[string]string {
	limits := map[string]string{}
	for _, p := range rep.Packages {
		limits[p.Package] = p.Limit
	}
	return limits
}

func TestRun_C12_WallAboveTwiceTheFloorIsContention(t *testing.T) {
	rep := c12Report(t, c12MacJobs, "macOS job", c12MacStream, c12MacRusage, c12MacSamples)
	if got := limitByPackage(rep)[mod+"internal/core"]; got != "contention" {
		t.Errorf("internal/core wall 362 s, floor 92 s: limit = %q, want contention", got)
	}
}

func TestRun_C12_CPUUnderHalfASecondIsWait(t *testing.T) {
	rep := c12Report(t, c12MacJobs, "macOS job", c12MacStream, c12MacRusage, c12MacSamples)
	if got := limitByPackage(rep)[mod+"cmd/small"]; got != "wait" {
		t.Errorf("cmd/small with 0.4 s of CPU: limit = %q, want wait", got)
	}
}

func TestRun_C12_UbuntuRunQueueUnderFourCPUsIsNotSaturation(t *testing.T) {
	rep := c12Report(t, c12UbuntuJobs, "ubuntu 1.27", c12UbuntuStream, c12UbuntuRusage, c12UbuntuSamples)
	if got := limitByPackage(rep)[mod+"internal/busy"]; got != "cpu" {
		t.Errorf("internal/busy at 95%% busy with runqueue 3 on 4 CPUs: limit = %q, want cpu", got)
	}
}

func TestRun_C12_PackageJSONKeepsEveryField(t *testing.T) {
	rep := c12Report(t, c12MacJobs, "macOS job", c12MacStream, c12MacRusage, c12MacSamples)
	raw, err := json.Marshal(rep.Packages[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"package", "result", "wall_s", "limit", "rusage", "sampler"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("package JSON lacks %q: %s", key, raw)
		}
	}
}
