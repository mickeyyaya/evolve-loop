package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func pipelineProject(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/internal/a/a.go", "package a\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "ship")
	sha := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	write := func(rel, body string) {
		p := filepath.Join(repo.Dir, ".evolve", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("waves/wave-1.json", `{"number":1,"cycle_floor":1}`)
	write("waves/wave-2.json", `{"number":2,"cycle_floor":3}`)
	sealed := `{"schema_version":"signal/1.0","seq":2,"ts":"2026-10-21T10:00:01Z","module":"orchestrator","kind":"cycle.sealed","severity":"INFO","origin":"F.e","reason":"sealed"}` + "\n"
	landed := `{"schema_version":"signal/1.0","seq":1,"ts":"2026-10-21T10:00:00Z","module":"ship","kind":"ship.landed","severity":"INFO","origin":"F.e","reason":"landed"}` + "\n"
	for id, c := range map[int]struct{ item, signals string }{1: {"a", landed + sealed}, 2: {"b", sealed}, 3: {"a", sealed}, 4: {"c", ""}} {
		dir := fmt.Sprintf("runs/cycle-%d/", id)
		write(dir+"run.json", `{"final_verdict":"PASS"}`)
		write(dir+"triage-decision.json", fmt.Sprintf(`{"top_n":[{"id":%q}]}`, c.item))
		write(dir+"signals.ndjson", c.signals)
		if id == 1 {
			write(dir+"ship-binding.json", fmt.Sprintf(`{"commit_sha":%q}`, sha))
		}
	}
	return repo.Dir
}

func TestQualityReport_TheWaveScopeReadsShipRateDeadlineRateAndPackages(t *testing.T) {
	t.Parallel()
	root := pipelineProject(t)
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": waveQualityReaders()}, newCenter: publishingQualityCenter}

	rc, stdout := runQualityReport(t, env, root)

	if rc != 0 || !strings.Contains(stdout, "20 reading(s)") || !strings.Contains(stdout, "6 warning(s)") {
		t.Fatalf("rc %d stdout %q, want 20 readings (PQ1 x2, PQ4 x4, CQ2 x2, CQ1 tree and wave, PQ5 x10) and six no-data warnings: wave 2 shipped nothing (CQ2 and three PQ5 costs) and the fixture tree has no function", rc, stdout)
	}
	byWindow := map[string]map[string]string{}
	for _, e := range channelEvents(t, root, "quality.reading") {
		if len(e.Fields) != 11 {
			t.Errorf("%s %s carries %d keys, want the 11 of spec §5.2", e.Fields["index"], e.Fields["window"], len(e.Fields))
		}
		byWindow[e.Fields["index"]+" "+e.Fields["window"]] = e.Fields
	}
	checks := map[string]string{"PQ1 waves:1-2": "0.250", "PQ4 waves:1-2": "0.250", "PQ4 waves:1-2/system-fail": "0.000", "CQ2 waves:1-2": "1.000", "PQ1 wave:2": "0.000"}
	for key, value := range checks {
		if byWindow[key]["value"] != value {
			t.Errorf("%s = %v, want value %s", key, byWindow[key], value)
		}
	}
	if byWindow["CQ2 wave:2"]["status"] != "no_data" {
		t.Errorf("CQ2 wave:2 = %v, want no_data", byWindow["CQ2 wave:2"])
	}
	if byWindow["CQ2 waves:1-2"]["source"] != ".evolve/runs + git show" || byWindow["PQ1 waves:1-2"]["n"] != "4" {
		t.Errorf("CQ2 %v PQ1 %v, want the git source and n 4", byWindow["CQ2 waves:1-2"], byWindow["PQ1 waves:1-2"])
	}
}

func TestWaveQualityReaders_AMissingWaveRecordIsASourceAbsentWarning(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": waveQualityReaders()}, newCenter: publishingQualityCenter}

	rc, stdout := runQualityReport(t, env, root)

	if rc != 0 || !strings.Contains(stdout, "0 reading(s), 0 breach(es), 6 warning(s)") {
		t.Errorf("rc %d stdout %q, want six warnings and exit 0", rc, stdout)
	}
	for _, e := range channelEvents(t, root, "quality.breach") {
		if e.Code != "QUALITY_SOURCE_ABSENT" || !(strings.HasSuffix(e.Fields["source"], filepath.Join(".evolve", "waves")) || strings.HasSuffix(e.Fields["source"], "go")) {
			t.Errorf("warning %+v, want QUALITY_SOURCE_ABSENT naming .evolve/waves or the go tree", e)
		}
	}
	if got := len(qualityScopes["wave"]); got != 6 {
		t.Errorf("the production wave scope has %d readers, want PQ1, PQ4, CQ2, the two CQ1 readers and PQ5", got)
	}
}

func TestQualityCompare_ReportsTheMDEStatusAndThePairedTest(t *testing.T) {
	t.Parallel()
	root := pipelineProject(t)
	var stdout, stderr bytes.Buffer

	rc := runQualityWith(fakeQualityEnv(), []string{"compare", "--index", "PQ1", "--a", "wave:1", "--b", "wave:2", "--paired", "--project-root", root}, &stdout, &stderr)

	out := stdout.String()
	if rc != 0 || !strings.Contains(out, "a=wave:1 1/2 b=wave:2 0/2 diff=-0.500") || !strings.Contains(out, "status=inconclusive") {
		t.Fatalf("rc %d stdout %q stderr %q", rc, out, stderr.String())
	}
	if !strings.Contains(out, "only in a=1 only in b=0 mcnemar_p=1.0000") {
		t.Errorf("stdout %q, want the paired counts of item a", out)
	}
	events := channelEvents(t, root, "quality.reading")
	if len(events) != 1 || events[0].Fields["index"] != "PQ3" || events[0].Fields["status"] != "inconclusive" || events[0].Fields["window"] != "PQ1 a:wave:1|b:wave:2" {
		t.Errorf("events = %+v, want one inconclusive PQ3 reading", events)
	}
}

func TestQualityCompare_UsageAndSourceErrors(t *testing.T) {
	t.Parallel()
	root := pipelineProject(t)
	for _, args := range [][]string{
		{"--index", "PQ2", "--a", "wave:1", "--b", "wave:2", "--project-root", root},
		{"--index", "PQ1", "--a", "wave:x", "--b", "wave:2", "--project-root", root},
		{"--index", "PQ1", "--a", "wave:1", "--b", "w", "--project-root", root},
		{"--index", "PQ1", "--a", "wave:1", "--b", "wave:2"},
		{"--index", "PQ1", "--a", "wave:1", "--b", "wave:2", "--project-root", root, "extra"},
		{"--index"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runQualityWith(fakeQualityEnv(), append([]string{"compare"}, args...), &stdout, &stderr); rc != exitUsage {
			t.Errorf("args %q: rc %d, want %d", args, rc, exitUsage)
		}
	}
	var stdout, stderr bytes.Buffer
	if rc := runQualityWith(fakeQualityEnv(), []string{"compare", "--index", "PQ4", "--a", "wave:1", "--b", "wave:2", "--project-root", t.TempDir()}, &stdout, &stderr); rc != exitIO {
		t.Errorf("no records: rc %d, want %d", rc, exitIO)
	}
}

func TestQualityCompare_EightDiscordantPairsSevenToOneGiveTheExactMcNemarP(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	evolveDir := filepath.Join(root, ".evolve")
	write := func(rel, body string) {
		p := filepath.Join(evolveDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("waves/wave-1.json", `{"number":1,"cycle_floor":1}`)
	write("waves/wave-2.json", `{"number":2,"cycle_floor":101}`)
	for i := 1; i <= 8; i++ {
		for _, id := range []int{i, 100 + i} {
			shipped := (id <= 7) || id == 108
			write(fmt.Sprintf("runs/cycle-%d/run.json", id), fmt.Sprintf(`{"shipped":%t}`, shipped))
			write(fmt.Sprintf("runs/cycle-%d/triage-decision.json", id), fmt.Sprintf(`{"top_n":[{"id":"item-%d"}]}`, i))
		}
	}
	var stdout, stderr bytes.Buffer

	rc := runQualityWith(fakeQualityEnv(), []string{"compare", "--index", "PQ1", "--a", "wave:1", "--b", "wave:2", "--paired", "--project-root", root}, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "only in a=7 only in b=1 mcnemar_p=0.0703") {
		t.Errorf("rc %d stdout %q stderr %q, want p = 18/256 = 0.0703", rc, stdout.String(), stderr.String())
	}
}

func TestQualityReport_APartialReaderPublishesItsReadingsAndWarns(t *testing.T) {
	t.Parallel()
	root := qualityProject(t, "")
	partial := []qualitypanel.Reading{{Index: "CQ2", Interval: qualitypanel.Interval{Value: 2, Lo: 2, Hi: 2}, Method: qualitypanel.MethodBootstrap95, N: 118}}

	rc, stdout := runQualityReport(t, fakeQualityEnv(readerOf("CQ2", partial, errors.New("git show gone: bad object"))), root)

	if rc != 0 || !strings.Contains(stdout, "1 reading(s), 0 breach(es), 1 warning(s)") {
		t.Errorf("rc %d stdout %q, want the reading and one warning", rc, stdout)
	}
}
