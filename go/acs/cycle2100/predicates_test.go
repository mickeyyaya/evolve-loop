//go:build acs

package cycle2100

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/citelemetry"
)

const (
	macJob      = "macOS job"
	ubuntuJob   = "ubuntu 1.27"
	pkgSuffix   = "internal/under"
	pkgFullPath = "example.com/m/go/" + pkgSuffix
	outsideBusy = 99
)

type sampleSpec struct {
	frac     float64
	busy     float64
	runqueue *int
	swapMB   float64
}

type scenario struct {
	job      string
	wall     float64
	user     float64
	system   float64
	serial   []float64
	parallel []float64
	samples  []sampleSpec
}

var baseTime = time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)

func at(offsetSeconds float64) string {
	return baseTime.Add(time.Duration(offsetSeconds * float64(time.Second))).Format(time.RFC3339Nano)
}

func ref(n int) *int { return &n }

func ndjson(t *testing.T, records []map[string]any) string {
	t.Helper()
	var out []byte
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		out = append(append(out, line...), '\n')
	}
	return string(out)
}

func (s scenario) stream(t *testing.T) string {
	t.Helper()
	events := []map[string]any{{"Time": at(0), "Action": "start", "Package": pkgFullPath}}
	mid := s.wall / 2
	for i, secs := range s.serial {
		events = append(events, map[string]any{"Time": at(mid), "Action": "pass", "Package": pkgFullPath, "Test": fmt.Sprintf("TestSerial%d", i), "Elapsed": secs})
	}
	for i, secs := range s.parallel {
		name := fmt.Sprintf("TestParallel%d", i)
		events = append(events,
			map[string]any{"Time": at(mid), "Action": "pause", "Package": pkgFullPath, "Test": name},
			map[string]any{"Time": at(mid), "Action": "pass", "Package": pkgFullPath, "Test": name, "Elapsed": secs})
	}
	events = append(events, map[string]any{"Time": at(s.wall), "Action": "pass", "Package": pkgFullPath, "Elapsed": s.wall})
	return ndjson(t, events)
}

func (s scenario) rusage(t *testing.T) string {
	return ndjson(t, []map[string]any{{
		"package": pkgSuffix, "wall_s": s.wall, "user_s": s.user, "system_s": s.system, "peak_rss_mb": 100, "exit_code": 0,
	}})
}

func (s scenario) sampleLines(t *testing.T) string {
	var records []map[string]any
	for _, x := range s.samples {
		r := map[string]any{"ts": at(x.frac * s.wall), "cpu_busy_pct": x.busy, "swap_used_mb": x.swapMB}
		if x.runqueue != nil {
			r["runqueue"] = *x.runqueue
		}
		records = append(records, r)
	}
	return ndjson(t, records)
}

type written struct {
	jobsFile string
	artDir   string
	binding  string
}

func (s scenario) write(t *testing.T) written {
	t.Helper()
	jobsDir, artDir := t.TempDir(), t.TempDir()
	jobs := fmt.Sprintf(`{"jobs":[{"name":%q,"started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:30:00Z","steps":[`+
		`{"name":"test","started_at":"2026-10-20T10:00:00Z","completed_at":"2026-10-20T10:29:00Z"}]}]}`, s.job)
	files := map[string]string{
		filepath.Join(jobsDir, "jobs.json"):            jobs,
		filepath.Join(artDir, "test-integration.json"): s.stream(t),
		filepath.Join(artDir, "test-rusage.ndjson"):    s.rusage(t),
		filepath.Join(artDir, "runner-samples.ndjson"): s.sampleLines(t),
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return written{jobsFile: filepath.Join(jobsDir, "jobs.json"), artDir: artDir, binding: s.job + "=" + artDir}
}

func (s scenario) report(t *testing.T) citelemetry.Report {
	t.Helper()
	w := s.write(t)
	rep, err := citelemetry.Run(w.jobsFile, []string{w.binding})
	if err != nil {
		t.Fatalf("citelemetry.Run: %v", err)
	}
	return rep
}

func (s scenario) limit(t *testing.T) string {
	t.Helper()
	rep := s.report(t)
	if len(rep.Packages) != 1 || rep.Packages[0].Package != pkgFullPath {
		t.Fatalf("packages = %+v, want the one package %s", rep.Packages, pkgFullPath)
	}
	return rep.Packages[0].Limit
}

func busyWindow(busy float64, runqueue *int) []sampleSpec {
	return []sampleSpec{{frac: 0.25, busy: busy, runqueue: runqueue}, {frac: 0.75, busy: busy, runqueue: runqueue}}
}

func withOutside(in []sampleSpec) []sampleSpec {
	out := []sampleSpec{{frac: -1, busy: outsideBusy}}
	out = append(out, in...)
	return append(out, sampleSpec{frac: 2, busy: outsideBusy})
}

func TestC2100_001_PackageWindowAt60PercentBusyIsNotCPUSaturation(t *testing.T) {
	base := scenario{job: macJob, wall: 10, user: 8, system: 1, serial: []float64{8}}
	cases := []struct {
		name    string
		samples []sampleSpec
		want    string
	}{
		{"window at 60 between two saturated samples outside the window", withOutside(busyWindow(60, nil)), "cpu"},
		{"window at 60 and 95 keeps the lowest sample", withOutside([]sampleSpec{{frac: 0.25, busy: 60}, {frac: 0.75, busy: 95}}), "cpu"},
		{"window at 95 throughout is saturated", withOutside(busyWindow(95, nil)), "cpu-saturation"},
		{"window at exactly 90 is not above the limit", busyWindow(90, nil), "cpu"},
	}
	for _, c := range cases {
		s := base
		s.samples = c.samples
		if got := s.limit(t); got != c.want {
			t.Errorf("%s: limit = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestC2100_002_BinaryWithTinyCPUIsWait(t *testing.T) {
	cases := []struct {
		name string
		s    scenario
		want string
	}{
		{"1.016 s wall and 0.011 s CPU with system above user", scenario{job: ubuntuJob, wall: 1.016, user: 0.001, system: 0.01, serial: []float64{0.001}}, "wait"},
		{"0.4 s of CPU in 0.6 s of wall is below the floor despite 0.67 efficiency", scenario{job: macJob, wall: 0.6, user: 0.3, system: 0.1, serial: []float64{0.5}}, "wait"},
		{"0.3 s of CPU in 30 s of wall over a 5 s floor is wait, not contention", scenario{job: macJob, wall: 30, user: 0.2, system: 0.1, serial: []float64{5}}, "wait"},
		{"0.4 s of system CPU above user CPU is wait, not subprocesses", scenario{job: ubuntuJob, wall: 0.6, user: 0.05, system: 0.35, serial: []float64{0.5}}, "wait"},
		{"exactly 0.5 s of CPU is not below the floor", scenario{job: macJob, wall: 0.6, user: 0.5, serial: []float64{0.5}}, "cpu"},
	}
	for _, c := range cases {
		if got := c.s.limit(t); got != c.want {
			t.Errorf("%s: limit = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestC2100_003_RunnerCPUCountIs3ForMacOSAnd4ForUbuntu(t *testing.T) {
	cases := []struct {
		name string
		job  string
		busy []sampleSpec
		want string
	}{
		{"macOS run queue 3 is not above 3 CPUs", macJob, busyWindow(95, ref(3)), "cpu"},
		{"macOS run queue 4 is above 3 CPUs", macJob, busyWindow(95, ref(4)), "cpu-saturation"},
		{"macOS has no run queue and saturates on busy alone", macJob, busyWindow(95, nil), "cpu-saturation"},
		{"ubuntu run queue 4 is not above 4 CPUs", ubuntuJob, busyWindow(95, ref(4)), "cpu"},
		{"ubuntu run queue 5 is above 4 CPUs", ubuntuJob, busyWindow(95, ref(5)), "cpu-saturation"},
		{"ubuntu run queue 3 is not above 4 CPUs", ubuntuJob, busyWindow(95, ref(3)), "cpu"},
		{"ubuntu without a run queue saturates on busy alone", ubuntuJob, busyWindow(95, nil), "cpu-saturation"},
		{"ubuntu run queue spike on one sample does not stay above the CPUs", ubuntuJob,
			[]sampleSpec{{frac: 0.25, busy: 95, runqueue: ref(9)}, {frac: 0.75, busy: 95, runqueue: ref(2)}}, "cpu"},
	}
	for _, c := range cases {
		s := scenario{job: c.job, wall: 10, user: 8, system: 1, serial: []float64{8}, samples: c.busy}
		if got := s.limit(t); got != c.want {
			t.Errorf("%s: limit = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestC2100_004_PackageAtFourTimesItsFloorIsContention(t *testing.T) {
	cases := []struct {
		name string
		s    scenario
		want string
	}{
		{"4 times the floor of 30 s", scenario{job: macJob, wall: 120, user: 80, system: 20, serial: []float64{20}, parallel: []float64{10}}, "contention"},
		{"core worked example: wall 362 s over a 92 s floor", scenario{job: macJob, wall: 362, user: 300, system: 20, serial: []float64{60}, parallel: []float64{32}}, "contention"},
		{"wall above twice the floor of serial sum plus longest parallel", scenario{job: macJob, wall: 190, user: 150, system: 10, serial: []float64{60}, parallel: []float64{32}}, "contention"},
		{"wall below twice the floor of serial sum plus longest parallel", scenario{job: macJob, wall: 180, user: 150, system: 10, serial: []float64{60}, parallel: []float64{32}}, "cpu"},
		{"wall exactly twice the floor is not more than twice", scenario{job: macJob, wall: 60, user: 50, system: 5, serial: []float64{30}}, "cpu"},
		{"contention comes before the system CPU rule", scenario{job: macJob, wall: 120, user: 20, system: 80, serial: []float64{30}}, "contention"},
		{"contention comes before the low efficiency rule", scenario{job: macJob, wall: 40, user: 6, system: 2, serial: []float64{10}}, "contention"},
		{"swap above the limit stays memory", scenario{job: macJob, wall: 120, user: 80, system: 20, serial: []float64{30}, samples: []sampleSpec{{frac: 0.5, busy: 50, swapMB: 2048}}}, "memory"},
		{"a saturated window stays cpu-saturation", scenario{job: macJob, wall: 120, user: 80, system: 20, serial: []float64{30}, samples: busyWindow(95, nil)}, "cpu-saturation"},
		{"a package without test timings has no floor and is not contention", scenario{job: macJob, wall: 70, user: 60, system: 5}, "cpu"},
	}
	for _, c := range cases {
		if got := c.s.limit(t); got != c.want {
			t.Errorf("%s: limit = %q, want %q", c.name, got, c.want)
		}
	}
}

func keysOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %s", raw)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func requireKeys(t *testing.T, where string, raw json.RawMessage, want ...string) {
	t.Helper()
	have := map[string]bool{}
	for _, k := range keysOf(t, raw) {
		have[k] = true
	}
	for _, k := range want {
		if !have[k] {
			t.Errorf("%s lacks %q: %s", where, k, raw)
		}
	}
}

func TestC2100_005_ReportJSONKeepsEveryCurrentField(t *testing.T) {
	s := scenario{job: macJob, wall: 10, user: 8, system: 1, serial: []float64{8}, samples: busyWindow(60, nil)}
	rep := s.report(t)
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	requireKeys(t, "report", raw, "g1", "critical_path", "packages", "incomplete")
	requireKeys(t, "g1", top["g1"], "macos_job_s", "ubuntu_test_step_s")
	var path []json.RawMessage
	if err := json.Unmarshal(top["critical_path"], &path); err != nil || len(path) == 0 {
		t.Fatalf("critical_path = %s err = %v", top["critical_path"], err)
	}
	for _, step := range path {
		requireKeys(t, "critical_path step", step, "level", "unit", "seconds")
	}
	var packages []map[string]json.RawMessage
	if err := json.Unmarshal(top["packages"], &packages); err != nil || len(packages) != 1 {
		t.Fatalf("packages = %s err = %v", top["packages"], err)
	}
	pkg := packages[0]
	pkgRaw, _ := json.Marshal(pkg)
	requireKeys(t, "package", pkgRaw, "package", "result", "wall_s", "limit", "rusage", "sampler")
	requireKeys(t, "rusage", pkg["rusage"], "wall_s", "user_s", "system_s", "peak_rss_mb", "exit_code")
	requireKeys(t, "sampler", pkg["sampler"], "samples", "cpu_busy_max_pct", "swap_used_max_mb")
	var limit string
	if err := json.Unmarshal(pkg["limit"], &limit); err != nil || limit != "cpu" {
		t.Errorf("limit = %s err = %v, want the string cpu", pkg["limit"], err)
	}
	if rep.Packages[0].Rusage.UserS != 8 || rep.Packages[0].Sampler.Samples != 2 || rep.Packages[0].Sampler.CPUBusyMaxPct != 60 {
		t.Errorf("joined values changed: rusage %+v sampler %+v", rep.Packages[0].Rusage, rep.Packages[0].Sampler)
	}
}

func digestTree(t *testing.T, dirs ...string) map[string][32]byte {
	t.Helper()
	sums := map[string][32]byte{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			sums[path] = sha256.Sum256(body)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return sums
}

func TestC2100_005_ReportNeverChangesTheRunItReads(t *testing.T) {
	s := scenario{job: ubuntuJob, wall: 120, user: 80, system: 20, serial: []float64{20}, parallel: []float64{10}, samples: busyWindow(95, ref(9))}
	w := s.write(t)
	dirs := []string{filepath.Dir(w.jobsFile), w.artDir}
	before := digestTree(t, dirs...)
	first, err := citelemetry.Run(w.jobsFile, []string{w.binding})
	if err != nil {
		t.Fatal(err)
	}
	second, err := citelemetry.Run(w.jobsFile, []string{w.binding})
	if err != nil {
		t.Fatal(err)
	}
	if after := digestTree(t, dirs...); !reflect.DeepEqual(before, after) {
		t.Errorf("Run changed the files it reads or added files:\nbefore %v\nafter  %v", before, after)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two reads of one run differ:\n%+v\n%+v", first, second)
	}
}
