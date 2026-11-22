package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wavehealth"
)

const checkinFixtures = "testdata/wavecheckin"

var checkinGreenCI = wavehealth.CIState{Available: true, Status: "completed", Conclusion: "success", RunID: "9001"}

type checkinFixture struct {
	name     string
	pid      int
	digestAt map[string]string
}

var (
	wave185Fixture = checkinFixture{name: "wave185", pid: 13795, digestAt: map[string]string{"2583": "2026-11-21T12:25:21Z"}}
	wave186Fixture = checkinFixture{name: "wave186", pid: 51538}
	wave184Fixture = checkinFixture{name: "wave184", pid: 26667, digestAt: map[string]string{"2576": "2026-11-21T04:00:28Z", "2577": "2026-11-21T02:56:05Z", "2579": "2026-11-20T23:38:24Z"}}
)

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

type checkinPlane struct {
	wavePlane
	fixture checkinFixture
	harness *waveHarness
}

func newCheckinPlane(t *testing.T, f checkinFixture) checkinPlane {
	t.Helper()
	p := checkinPlane{wavePlane: wavePlane{t: t, root: t.TempDir()}, fixture: f, harness: newWaveHarness()}
	copyTree(t, filepath.Join(checkinFixtures, f.name), p.evolveDir())
	records, err := p.store().Records()
	if err != nil || len(records) != 1 {
		t.Fatalf("fixture records = %v, %v", records, err)
	}
	rec := records[0]
	rec.LogPath = p.loopLog(rec.RunID, f.pid)
	if err := p.store().Save(rec); err != nil {
		t.Fatal(err)
	}
	for cycle, at := range f.digestAt {
		when, _ := time.Parse(time.RFC3339, at)
		path := filepath.Join(p.evolveDir(), "runs", "cycle-"+cycle, "failure-digest.json")
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	p.harness.alive[f.pid] = true
	return p
}

type checkinRun struct {
	rc     int
	stdout string
	stderr string
}

func (p checkinPlane) run(at time.Time, ci wavehealth.CIState, args ...string) checkinRun {
	env := p.harness.env(p.root)
	env.now = func() time.Time { return at }
	env.mainCI = func(string) wavehealth.CIState { return ci }
	var out, errb bytes.Buffer
	rc := runWaveWith(env, append([]string{"checkin", "--project-root", p.root}, args...), &out, &errb)
	return checkinRun{rc: rc, stdout: out.String(), stderr: errb.String()}
}

func (r checkinRun) report(t *testing.T) wavehealth.Report {
	t.Helper()
	var rep wavehealth.Report
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("decode the JSON report: %v\n%s", err, r.stdout)
	}
	return rep
}

func at(t *testing.T, stamp string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func rowResult(stdout, id string) string {
	for _, line := range strings.Split(stdout, "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[0] == id {
			return f[1]
		}
	}
	return ""
}

func brakingSteps(rep wavehealth.Report) []string {
	var ids []string
	for _, s := range rep.Steps {
		if s.Result == wavehealth.ResultBrake {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

func firstBrake(t *testing.T, p checkinPlane, from, until time.Time) (time.Time, wavehealth.Report) {
	t.Helper()
	for now := from; now.Before(until); now = now.Add(time.Minute) {
		run := p.run(now, checkinGreenCI, "--json")
		switch run.rc {
		case checkinExitBrake:
			return now, run.report(t)
		case 0:
		default:
			t.Fatalf("at %s the verb exited %d: %s %s", now.Format(time.RFC3339), run.rc, run.stdout, run.stderr)
		}
	}
	t.Fatalf("the verb never gave BRAKE between %s and %s", from.Format(time.RFC3339), until.Format(time.RFC3339))
	return time.Time{}, wavehealth.Report{}
}

func TestWaveCheckin_Wave185BrakesBefore1000Z(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	start, deadline := at(t, "2026-11-21T08:17:37Z"), at(t, "2026-11-21T10:00:00Z")
	first, rep := firstBrake(t, p, start.Add(time.Minute), deadline)
	if got := brakingSteps(rep); strings.Join(got, ",") != "A4" {
		t.Fatalf("the first BRAKE at %s came from %v, want only A4: the load of the host never stayed at 3.5 per core for 15 minutes before 09:33Z", first.Format(time.RFC3339), got)
	}
	t.Logf("wave 185 with host samples: first BRAKE at %s from %v", first.Format(time.RFC3339), brakingSteps(rep))
}

func TestWaveCheckin_Wave185TheLockFaultAloneBrakesBefore1000Z(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	if err := os.RemoveAll(hostSamplesPath(loopConfig{EvolveDir: p.evolveDir()})); err != nil {
		t.Fatal(err)
	}
	start, deadline := at(t, "2026-11-21T08:17:37Z"), at(t, "2026-11-21T10:00:00Z")
	first, rep := firstBrake(t, p, start.Add(time.Minute), deadline)
	if got := brakingSteps(rep); strings.Join(got, ",") != "A4" {
		t.Fatalf("the first BRAKE at %s came from %v, want only A4", first.Format(time.RFC3339), got)
	}
	if first.Before(at(t, "2026-11-21T09:33:15Z")) || first.After(at(t, "2026-11-21T09:34:15Z")) {
		t.Fatalf("the first BRAKE at %s, want within a minute of the 15th lock event at 09:33:15Z", first.Format(time.RFC3339))
	}
	stalls := p.run(at(t, "2026-11-21T09:43:00Z"), checkinGreenCI, "--json").report(t)
	if got := strings.Join(brakingSteps(stalls), ","); got != "A4,A5" {
		t.Fatalf("at 09:43Z the braking steps are %q, want A4,A5 (the third stall line is at 09:42:30Z)", got)
	}
	t.Logf("wave 185 without host samples: first BRAKE at %s from %v", first.Format(time.RFC3339), brakingSteps(rep))
}

func TestWaveCheckin_Wave185TextAtTheSOPReadingNamesStallAndSharedFault(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	run := p.run(at(t, "2026-11-21T10:20:00Z"), checkinGreenCI)
	if run.rc != checkinExitBrake {
		t.Fatalf("exit = %d, want %d\n%s%s", run.rc, checkinExitBrake, run.stdout, run.stderr)
	}
	lines := strings.Split(strings.TrimSpace(run.stdout), "\n")
	if last := lines[len(lines)-1]; last != "outcome: BRAKE" {
		t.Errorf("last line = %q, want %q", last, "outcome: BRAKE")
	}
	for _, id := range []string{"A2", "A4", "A5"} {
		if got := rowResult(run.stdout, id); got != "BRAKE" {
			t.Errorf("row %s = %q, want BRAKE:\n%s", id, got, run.stdout)
		}
	}
	for _, want := range []string{"; STALL", "SHARED FAULT", "verify-single-flight-unavailable", "live 8"} {
		if !strings.Contains(run.stdout, want) {
			t.Errorf("the text lacks %q:\n%s", want, run.stdout)
		}
	}
}

func TestWaveCheckin_Wave184ContinuesAtEachClockTime(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	for _, stamp := range []string{"2026-11-20T23:00:00Z", "2026-11-21T01:00:00Z", "2026-11-21T03:00:00Z", "2026-11-21T04:30:00Z"} {
		run := p.run(at(t, stamp), checkinGreenCI)
		if run.rc != 0 || !strings.HasSuffix(run.stdout, "outcome: CONTINUE\n") {
			t.Errorf("at %s exit = %d, want CONTINUE\n%s%s", stamp, run.rc, run.stdout, run.stderr)
		}
	}
	last := p.run(at(t, "2026-11-21T04:30:00Z"), checkinGreenCI, "--json").report(t)
	for _, s := range last.Steps {
		if s.ID == "A3" && !strings.Contains(s.Reading, "3 failed cycle(s)") {
			t.Errorf("A3 must read the 3 failed cycles of wave 184: %q", s.Reading)
		}
	}
}

func TestWaveCheckin_Wave186StallsBrakeWhileTheLanesAreSilentAndContinueOnceTheyRecovered(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave186Fixture)
	silent := p.run(at(t, "2026-11-21T22:40:00Z"), checkinGreenCI)
	if got := rowResult(silent.stdout, "A5"); got != "BRAKE" {
		t.Errorf("A5 at 22:40Z with the 3 lanes silent = %q, want BRAKE\n%s", got, silent.stdout)
	}
	recovered := p.run(at(t, "2026-11-21T23:39:00Z"), checkinGreenCI)
	if got := rowResult(recovered.stdout, "A5"); got != "OK" {
		t.Errorf("A5 at 23:39Z after the 3 lanes signalled = %q, want OK\n%s", got, recovered.stdout)
	}
	if reading := stepReading(t, p.run(at(t, "2026-11-21T23:39:00Z"), checkinGreenCI, "--json"), "A5"); !strings.Contains(reading, "3 line(s) recovered") {
		t.Errorf("A5 reading must name the recovered stalls: %q", reading)
	}
}

func TestWaveCheckin_AFailedMainCIExitsFixLocalAndNamesTheClassifyCommand(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	red := wavehealth.CIState{Available: true, Status: "completed", Conclusion: "failure", RunID: "424242"}
	run := p.run(at(t, "2026-11-21T03:00:00Z"), red)
	if run.rc != checkinExitFixLocal || !strings.Contains(run.stdout, "evolve ci classify 424242 --json") || !strings.HasSuffix(run.stdout, "outcome: FIX-LOCAL\n") {
		t.Fatalf("exit = %d\n%s", run.rc, run.stdout)
	}
}

func TestWaveCheckin_AnUnmappedCIReadingExitsBrake(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	odd := wavehealth.CIState{Available: true, Status: "completed", Conclusion: "cancelled", RunID: "5"}
	run := p.run(at(t, "2026-11-21T03:00:00Z"), odd)
	if run.rc != checkinExitBrake || !strings.Contains(run.stdout, "unmapped reading") {
		t.Fatalf("exit = %d, want BRAKE with an unmapped reading\n%s", run.rc, run.stdout)
	}
}

func TestWaveCheckin_ALoopThatIsNotRunningEndsAtA1(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	p.harness.alive[wave184Fixture.pid] = false
	env := p.harness.env(p.root)
	env.now = func() time.Time { return at(t, "2026-11-21T03:00:00Z") }
	ciCalls := 0
	env.mainCI = func(string) wavehealth.CIState { ciCalls++; return checkinGreenCI }
	var out, errb bytes.Buffer
	rc := runWaveWith(env, []string{"checkin", "--project-root", p.root}, &out, &errb)
	if rc != 0 || rowResult(out.String(), "A1") != "OK" || rowResult(out.String(), "A5") != "SKIP" || strings.Contains(out.String(), "  BRAKE ") || ciCalls != 0 {
		t.Fatalf("exit = %d, CI calls = %d\n%s", rc, ciCalls, out.String())
	}
}

func TestWaveCheckin_AHeldBrakeKeepsTheOutcomeAtContinue(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	p.write(".evolve/loop-stop", "stop\n")
	run := p.run(at(t, "2026-11-21T10:20:00Z"), checkinGreenCI)
	if run.rc != 0 || rowResult(run.stdout, "A5") != "HELD" || !strings.Contains(run.stdout, "brake held (BRAKE-HELD)") {
		t.Fatalf("exit = %d, want CONTINUE with HELD rows\n%s", run.rc, run.stdout)
	}
}

func TestWaveCheckin_EachThresholdOfThePolicyChangesTheRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, policy, step string
		before, after      wavehealth.Result
	}{
		{"stall_min_age_minutes", `{"wave":{"checkin":{"stall_min_age_minutes":600}}}`, "A2", wavehealth.ResultBrake, wavehealth.ResultOK},
		{"stall_min_live_lanes", `{"wave":{"checkin":{"stall_min_live_lanes":9}}}`, "A2", wavehealth.ResultBrake, wavehealth.ResultOK},
		{"abnormal_min_events", `{"wave":{"checkin":{"abnormal_min_events":1000}}}`, "A4", wavehealth.ResultBrake, wavehealth.ResultOK},
		{"abnormal_min_cycles", `{"wave":{"checkin":{"abnormal_min_cycles":50}}}`, "A4", wavehealth.ResultBrake, wavehealth.ResultOK},
		{"stall_lines", `{"wave":{"checkin":{"stall_lines":50}}}`, "A5", wavehealth.ResultBrake, wavehealth.ResultOK},
		{"load_sustained_minutes", `{"wave":{"checkin":{"load_sustained_minutes":1}}}`, "A8-load", wavehealth.ResultOK, wavehealth.ResultGap},
		{"load_per_core_limit", `{"wave":{"checkin":{"load_per_core_limit":0.5}}}`, "A8-load", wavehealth.ResultOK, wavehealth.ResultBrake},
		{"stall_window_minutes", `{"wave":{"checkin":{"stall_window_minutes":1}}}`, "A5", wavehealth.ResultBrake, wavehealth.ResultOK},
		{"gc.disk_floor_gb", `{"gc":{"disk_floor_gb":2000}}`, "A8-disk", wavehealth.ResultOK, wavehealth.ResultFixLocal},
	}
	now := at(t, "2026-11-21T10:20:00Z")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := newCheckinPlane(t, wave185Fixture)
			result := func() wavehealth.Result {
				for _, s := range p.run(now, checkinGreenCI, "--json").report(t).Steps {
					if s.ID == c.step {
						return s.Result
					}
				}
				t.Fatalf("no step %s", c.step)
				return ""
			}
			if got := result(); got != c.before {
				t.Fatalf("%s before the policy = %s, want %s", c.step, got, c.before)
			}
			p.write(".evolve/policy.json", c.policy)
			if got := result(); got != c.after {
				t.Errorf("%s with %s = %s, want %s", c.step, c.policy, got, c.after)
			}
		})
	}
}

func checkinTreeDigest(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			lines = append(lines, "d "+rel)
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		lines = append(lines, fmt.Sprintf("f %s %x %d", rel, sha256.Sum256(raw), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestWaveCheckin_TheVerbWritesNothing(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	before := checkinTreeDigest(t, p.root)
	for _, args := range [][]string{nil, {"--json"}} {
		p.run(at(t, "2026-11-21T10:20:00Z"), checkinGreenCI, args...)
	}
	if after := checkinTreeDigest(t, p.root); after != before {
		t.Fatal("the verb changed the project tree")
	}
}

func TestWaveCheckin_JSONHasTheStepsAndTheOutcome(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	run := p.run(at(t, "2026-11-21T10:20:00Z"), checkinGreenCI, "--json")
	rep := run.report(t)
	if rep.Outcome != wavehealth.OutcomeBrake || len(rep.Steps) != 12 || run.rc != checkinExitBrake {
		t.Fatalf("outcome %s with %d steps, exit %d", rep.Outcome, len(rep.Steps), run.rc)
	}
}

func TestWaveCheckin_ErrorsHaveTheirOwnExitCodes(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-11-21T10:20:00Z")
	t.Run("an unexpected argument is a usage error", func(t *testing.T) {
		t.Parallel()
		p := newCheckinPlane(t, wave185Fixture)
		if run := p.run(now, checkinGreenCI, "extra"); run.rc != exitUsage || !strings.Contains(run.stderr, "usage: evolve wave next") {
			t.Fatalf("exit = %d\n%s", run.rc, run.stderr)
		}
	})
	t.Run("a corrupt abnormal-events line is a read failure", func(t *testing.T) {
		t.Parallel()
		p := newCheckinPlane(t, wave185Fixture)
		p.write(".evolve/runs/cycle-2582/abnormal-events.jsonl", "{broken\n")
		if run := p.run(now, checkinGreenCI); run.rc != exitIO || !strings.Contains(run.stderr, "line 1") {
			t.Fatalf("exit = %d\n%s", run.rc, run.stderr)
		}
	})
	t.Run("a corrupt host sample is a read failure", func(t *testing.T) {
		t.Parallel()
		p := newCheckinPlane(t, wave185Fixture)
		p.write(".evolve/logs/host-pressure.ndjson", "{broken\n")
		if run := p.run(now, checkinGreenCI); run.rc != exitIO || !strings.Contains(run.stderr, "host-pressure.ndjson") {
			t.Fatalf("exit = %d\n%s", run.rc, run.stderr)
		}
	})
	t.Run("a project with no .evolve is refused", func(t *testing.T) {
		t.Parallel()
		p := newCheckinPlane(t, wave185Fixture)
		if run := p.run(now, checkinGreenCI, "--project-root", t.TempDir()); run.rc == 0 {
			t.Fatalf("exit = %d, want a refusal", run.rc)
		}
	})
}

func TestWaveCheckin_NoWaveRecordedAndNoLoopEndsAtA1(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	if err := os.RemoveAll(filepath.Join(p.evolveDir(), "waves")); err != nil {
		t.Fatal(err)
	}
	run := p.run(at(t, "2026-11-21T10:20:00Z"), checkinGreenCI)
	if run.rc != 0 || rowResult(run.stdout, "A1") != "OK" || rowResult(run.stdout, "A2") != "SKIP" {
		t.Fatalf("exit = %d\n%s", run.rc, run.stdout)
	}
}

func TestWaveCheckin_MissingSamplesAndAFailedDiskReadAreGaps(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	p.harness.diskFree = func(string) (uint64, error) { return 0, io.ErrUnexpectedEOF }
	run := p.run(at(t, "2026-11-21T03:00:00Z"), checkinGreenCI, "--json")
	steps := map[string]wavehealth.Result{}
	for _, s := range run.report(t).Steps {
		steps[s.ID] = s.Result
	}
	if steps["A8-load"] != wavehealth.ResultGap || steps["A8-disk"] != wavehealth.ResultGap || run.rc != 0 {
		t.Fatalf("A8 rows = %v, exit %d", steps, run.rc)
	}
	if !strings.Contains(run.stderr, "free disk") {
		t.Errorf("a failed disk read must warn on stderr: %q", run.stderr)
	}
}

func TestWaveCheckin_ThePlaneHelperKeepsTheRecordOfTheFixture(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	records, err := p.store().Records()
	if err != nil || len(records) != 1 || records[0].Number != 185 || records[0].RunID != "20261121T081737Z" || records[0].CycleFloor != 2580 {
		t.Fatalf("records = %+v, %v", records, err)
	}
}

func TestWaveCheckin_AFailedJSONWriteIsAnIOError(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave185Fixture)
	env := p.harness.env(p.root)
	env.now = func() time.Time { return at(t, "2026-11-21T10:20:00Z") }
	env.mainCI = func(string) wavehealth.CIState { return checkinGreenCI }
	var errb bytes.Buffer
	if rc := runWaveWith(env, []string{"checkin", "--json", "--project-root", p.root}, &failingWriter{}, &errb); rc != exitIO || !strings.Contains(errb.String(), "encode") {
		t.Fatalf("rc = %d, stderr %q", rc, errb.String())
	}
}

func TestReadHostSamples(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "good.ndjson")
	if err := os.WriteFile(good, []byte(`{"ts":"2026-11-21T08:26:06Z","load1":14.7,"cpus":14,"load_per_core":1.05}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readHostSamples(good)
	if err != nil || !got.Known || len(got.Value) != 1 || got.Value[0].PerCore != 1.05 || !got.Value[0].At.Equal(at(t, "2026-11-21T08:26:06Z")) {
		t.Fatalf("readHostSamples(good) = %+v, %v", got, err)
	}
	if got, err := readHostSamples(filepath.Join(dir, "absent")); err != nil || got.Known {
		t.Errorf("an absent file must be unknown, got %+v, %v", got, err)
	}
	for name, path := range map[string]string{"a directory": dir, "a path under a file": filepath.Join(good, "child")} {
		if _, err := readHostSamples(path); err == nil {
			t.Errorf("readHostSamples(%s) must fail", name)
		}
	}
	bad := filepath.Join(dir, "bad.ndjson")
	const good1 = `{"ts":"2026-11-21T08:26:06Z","load_per_core":1}` + "\n"
	for name, body := range map[string]string{"json": "{broken\n", "time": `{"ts":"yesterday","load_per_core":1}` + "\n"} {
		if err := os.WriteFile(bad, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, atLine1 := readHostSamples(bad)
		if err := os.WriteFile(bad, []byte(good1+body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, atLine2 := readHostSamples(bad)
		if atLine1 == nil || atLine2 == nil {
			t.Fatalf("a bad %s line must fail, got %v and %v", name, atLine1, atLine2)
		}
		if strings.ReplaceAll(atLine1.Error(), bad, "") == strings.ReplaceAll(atLine2.Error(), bad, "") {
			t.Errorf("a bad %s line 1 and line 2 must report different positions, both gave %v", name, atLine1)
		}
	}
}

func TestCIStateOf(t *testing.T) {
	t.Parallel()
	run := statusCIRun{Workflow: "required.yml", Branch: "main", Status: "completed", Conclusion: "failure", URL: "https://github.com/o/r/actions/runs/424242"}
	cases := []struct {
		name string
		in   statusRemote
		want wavehealth.CIState
	}{
		{"a run", statusRemote{Available: true, Items: []statusCIRun{run}}, wavehealth.CIState{Available: true, Status: "completed", Conclusion: "failure", RunID: "424242"}},
		{"a run with no url", statusRemote{Available: true, Items: []statusCIRun{{Status: "queued"}}}, wavehealth.CIState{Available: true, Status: "queued"}},
		{"unavailable", statusRemote{Error: "gh run list: boom"}, wavehealth.CIState{}},
		{"no run", statusRemote{Available: true, Items: []statusCIRun{}}, wavehealth.CIState{}},
		{"a foreign item type", statusRemote{Available: true, Items: "x"}, wavehealth.CIState{}},
	}
	for _, c := range cases {
		if got := ciStateOf(c.in); got != c.want {
			t.Errorf("%s: ciStateOf = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestMainCIFrom_ConvertsTheFetchedRunAndBoundsItsContext(t *testing.T) {
	t.Parallel()
	var gotRoot string
	var hadDeadline bool
	fetch := func(ctx context.Context, root string) statusRemote {
		gotRoot = root
		_, hadDeadline = ctx.Deadline()
		return statusRemote{Available: true, Items: []statusCIRun{{Status: "in_progress", URL: "https://x/runs/77"}}}
	}
	got := mainCIFrom(fetch)("/project")
	want := wavehealth.CIState{Available: true, Status: "in_progress", RunID: "77"}
	if got != want || gotRoot != "/project" || !hadDeadline {
		t.Errorf("mainCIFrom = %+v root %q deadline %t, want %+v with the root and a deadline", got, gotRoot, hadDeadline, want)
	}
}

func TestFileModTime(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	when := at(t, "2026-11-20T01:02:03Z")
	if err := os.Chtimes(file, when, when); err != nil {
		t.Fatal(err)
	}
	if got := fileModTime(file); !got.Equal(when) {
		t.Errorf("fileModTime = %s, want %s", got, when)
	}
	if got := fileModTime(file + ".absent"); !got.IsZero() {
		t.Errorf("fileModTime(absent) = %s, want the zero time", got)
	}
}

func TestCheckinExitCode_EachOutcomeHasItsOwnCode(t *testing.T) {
	t.Parallel()
	got := map[wavehealth.Outcome]int{}
	for _, o := range []wavehealth.Outcome{wavehealth.OutcomeContinue, wavehealth.OutcomeBrake, wavehealth.OutcomeFixLocal} {
		got[o] = checkinExitCode(o)
	}
	want := map[wavehealth.Outcome]int{wavehealth.OutcomeContinue: 0, wavehealth.OutcomeBrake: 3, wavehealth.OutcomeFixLocal: 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exit codes = %v, want %v (documented in wave-operations.md)", got, want)
	}
}

func stepReading(t *testing.T, run checkinRun, id string) string {
	t.Helper()
	for _, s := range run.report(t).Steps {
		if s.ID == id {
			return s.Reading
		}
	}
	t.Fatalf("no step %s", id)
	return ""
}

func TestWaveCheckin_ADigestOfACycleThatEndsAfterNowIsNotCounted(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	if got := stepReading(t, p.run(at(t, "2026-11-21T03:00:00Z"), checkinGreenCI, "--json"), "A3"); !strings.Contains(got, "2 failed cycle(s)") {
		t.Errorf("at 03:00Z the digest of cycle 2576 (04:00Z) must not count: %q", got)
	}
	if got := stepReading(t, p.run(at(t, "2026-11-21T04:30:00Z"), checkinGreenCI, "--json"), "A3"); !strings.Contains(got, "3 failed cycle(s)") {
		t.Errorf("at 04:30Z all 3 digests count: %q", got)
	}
}

func TestWaveCheckin_ADigestAtTheFloorCycleBelongsToTheWaveBefore(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	p.write(".evolve/runs/cycle-2572/failure-digest.json", `{"cycle":2572,"fingerprint":"build|gate-block|floor","pre_class":"gate-block"}`)
	before := at(t, "2026-11-20T21:00:00Z")
	if err := os.Chtimes(filepath.Join(p.evolveDir(), "runs", "cycle-2572", "failure-digest.json"), before, before); err != nil {
		t.Fatal(err)
	}
	if got := stepReading(t, p.run(at(t, "2026-11-21T04:30:00Z"), checkinGreenCI, "--json"), "A3"); !strings.Contains(got, "3 failed cycle(s)") {
		t.Errorf("the digest of the floor cycle 2572 must not count: %q", got)
	}
}

func TestWaveCheckin_TheLiveLanesAreThoseOfTheLastPoolSignalAtOrBeforeNow(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	if got := stepReading(t, p.run(at(t, "2026-11-20T23:30:00Z"), checkinGreenCI, "--json"), "A2"); !strings.Contains(got, "live 5") {
		t.Errorf("at 23:30Z the last pool signal (23:08:52Z) gives 5 live lanes: %q", got)
	}
	if got := stepReading(t, p.run(at(t, "2026-11-21T04:30:00Z"), checkinGreenCI, "--json"), "A2"); !strings.Contains(got, "live 1") {
		t.Errorf("at 04:30Z the last pool signal gives 1 live lane: %q", got)
	}
}

func TestWaveCheckin_ReadsTheSamplesThatTheLoopWrites(t *testing.T) {
	t.Parallel()
	p := newCheckinPlane(t, wave184Fixture)
	now := at(t, "2026-11-21T03:00:00Z")
	var lines []string
	for _, ago := range []time.Duration{10 * time.Minute, 5 * time.Minute, time.Minute} {
		raw, err := json.Marshal(hostSampleLine{TS: now.Add(-ago).Format(time.RFC3339), Load1: 70, CPUs: 14, LoadPerCore: 5})
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	path := hostSamplesPath(loopConfig{EvolveDir: p.evolveDir()})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := p.run(now, checkinGreenCI, "--json")
	if !strings.Contains(stepReading(t, run, "A8-load"), "lowest 5.00 per core") {
		t.Errorf("the verb must read the file at hostSamplesPath: %q", stepReading(t, run, "A8-load"))
	}
}
