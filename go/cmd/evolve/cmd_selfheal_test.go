package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var selfhealNow = time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)

const selfhealCIFixture = `[
 {"databaseId":40420241292,"headBranch":"main","status":"completed","conclusion":"failure","attempt":1,"createdAt":"2026-10-26T23:30:38Z","updatedAt":"2026-10-26T23:51:03Z"},
 {"databaseId":40265789886,"headBranch":"main","status":"completed","conclusion":"cancelled","attempt":1,"createdAt":"2026-10-26T03:47:23Z","updatedAt":"2026-10-26T08:55:28Z"},
 {"databaseId":40269836920,"headBranch":"docs/cell-context-index","status":"completed","conclusion":"success","attempt":2,"createdAt":"2026-10-26T04:35:14Z","updatedAt":"2026-10-26T05:40:00Z"},
 {"databaseId":1,"headBranch":"main","status":"completed","conclusion":"cancelled","attempt":1,"createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T09:00:00Z"}
]`

func selfhealFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func selfhealProject(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	selfhealFile(t, filepath.Join(evolveDir, paths.PolicyFile), policyJSON)
	log, err := channel.New(paths.EventChannelsRoot(evolveDir), "errors", channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	breach := signalcenter.Event{SchemaVersion: signalcenter.SchemaVersion, PID: 7, Seq: 1, TS: "2026-10-26T20:00:00Z", Module: signalcenter.ModuleQuality, Origin: "qualitypanel.Publish",
		Kind: signalcenter.KindQualityBreach, Severity: signalcenter.SeverityWarn, Code: "QUALITY_PQ1_BREACH", Reason: "PQ1 breach"}
	if _, err := log.Append([]channel.Record{{Source: "test", Signal: &breach}}); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(evolveDir, "runs")
	for i, id := range []string{"2026", "2030", "2035"} {
		started := selfhealNow.Add(time.Duration(i-10) * time.Hour).Format(time.RFC3339)
		selfhealFile(t, filepath.Join(runs, "cycle-"+id, "run.json"), `{"started_at":"`+started+`","audit_repair_attempts":2}`)
		selfhealFile(t, filepath.Join(runs, "cycle-"+id, "triage-decision.json"), `{"top_n":[{"id":"retro-minting-files-through-inbox-add-lane"}]}`)
	}
	selfhealFile(t, filepath.Join(runs, "cycle-2035", "defect-ledger.json"), `{"entries":[{"text":"M1: go/internal/core/failurelearning floor tests fail; protected surface","status":"OPEN"}]}`)
	selfhealFile(t, filepath.Join(evolveDir, shipJournalFile), `{"sha":"b","class":"cycle","cycle":2036}`+"\n")
	selfhealFile(t, filepath.Join(evolveDir, "usage-windows.json"), `{"claude":{"cli":"claude","observed_at":"2026-10-26T18:28:52Z","windows":[]}}`)
	return root
}

func selfhealTestEnv(gh func(context.Context, string, ...string) (string, error)) selfhealEnv {
	return selfhealEnv{
		now: func() time.Time { return selfhealNow }, gh: gh, declared: func(f string) bool { return f == "claude" },
		newCenter: func(root, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
			c := newRootSignalCenter(root, evolveDir, console)
			return c, rootwire.Attach(c, roleSelfheal, evolveDir, console)
		},
	}
}

func fixtureGH(_ context.Context, _ string, args ...string) (string, error) {
	if !slices.Contains(args, "required.yml") || !slices.Contains(args, selfhealCIFields) {
		return "", errors.New("unexpected gh call " + strings.Join(args, " "))
	}
	return selfhealCIFixture, nil
}

type selfhealEnvelope struct {
	Schema   string              `json:"schema"`
	OK       bool                `json:"ok"`
	Data     selfhealReport      `json:"data"`
	Error    *struct{ Code int } `json:"error"`
	Warnings []string            `json:"warnings"`
}

func runSelfhealJSON(t *testing.T, env selfhealEnv, root string) (int, selfhealEnvelope, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runSelfhealWith(env, []string{"incidents", "--json", "--project-root", root}, &stdout, &stderr)
	var e selfhealEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &e); err != nil {
		t.Fatalf("stdout is not one envelope: %v\n%s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return rc, e, stderr.String()
}

func selfhealFingerprints(incs []selfheal.Incident) []string {
	var out []string
	for _, inc := range incs {
		out = append(out, inc.Fingerprint)
	}
	slices.Sort(out)
	return out
}

func TestSelfhealIncidents_APassOpensTheFixtureIncidentsWritesTheRecordsAndPublishesThem(t *testing.T) {
	t.Parallel()
	root := selfhealProject(t, `{}`)
	rc, e, stderr := runSelfhealJSON(t, selfhealTestEnv(fixtureGH), root)
	if rc != 0 || !e.OK || e.Schema != selfhealSchema || e.Data.Stage != "shadow" || len(e.Warnings) != 0 {
		t.Fatalf("rc=%d envelope=%+v stderr=%s", rc, e, stderr)
	}
	want := []string{
		"ci-hang:main", "claim-churn:retro-minting-files-through-inbox-add-lane", "flaky-test:rerun-green", "main-red:main",
		"quality:QUALITY_PQ1_BREACH", "repair-loop:retro-minting-files-through-inbox-add-lane", "usage-blind:claude",
	}
	if got := selfhealFingerprints(e.Data.Incidents); !slices.Equal(got, want) || len(e.Data.Transitions) != len(want) {
		t.Fatalf("fingerprints %v, want %v; transitions %+v", got, want, e.Data.Transitions)
	}
	records, err := selfheal.LoadIncidents(selfheal.Dir(paths.EvolveDirOf(root)))
	if err != nil || len(records) != len(want) {
		t.Fatalf("records on disk: %v %d", err, len(records))
	}
	for _, r := range records {
		if r.Fingerprint == "repair-loop:retro-minting-files-through-inbox-add-lane" && (r.Tier != selfheal.TierKernel || r.Severity != signalcenter.SeverityIncident) {
			t.Fatalf("the repair loop on a protected finding is T0 and INCIDENT: %+v", r)
		}
	}
	events := selfhealChannelEvents(t, root)
	if len(events) != len(want) || events[0].Kind != signalcenter.KindSelfhealIncident || events[0].Fields["transition"] != "opened" {
		t.Fatalf("the selfheal.incident channel holds one opened event for each record: %+v", events)
	}
	rc, again, _ := runSelfhealJSON(t, selfhealTestEnv(fixtureGH), root)
	if rc != 0 || len(again.Data.Transitions) != 0 || len(selfhealChannelEvents(t, root)) != len(want) {
		t.Fatalf("a second pass over the same evidence opens and publishes nothing: %+v", again.Data.Transitions)
	}
}

func selfhealChannelEvents(t *testing.T, root string) []signalcenter.Event {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "selfheal.incident", channel.Config{})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := log.Read(0)
	if err != nil {
		t.Fatal(err)
	}
	var out []signalcenter.Event
	for _, r := range batch.Records {
		if r.Signal != nil {
			out = append(out, *r.Signal)
		}
	}
	return out
}

func TestSelfhealIncidents_StageOffListsTheRecordsAndRunsNoPass(t *testing.T) {
	t.Parallel()
	root := selfhealProject(t, `{"healing":{"detector":{"stage":"off"}}}`)
	if err := selfheal.WriteIncident(selfheal.Dir(paths.EvolveDirOf(root)), selfheal.Incident{ID: "inc-2026-10-26-00000000", State: selfheal.StateOpen}); err != nil {
		t.Fatal(err)
	}
	called := false
	gh := func(context.Context, string, ...string) (string, error) { called = true; return "[]", nil }
	var stdout, stderr bytes.Buffer
	rc := runSelfhealWith(selfhealTestEnv(gh), []string{"incidents", "--project-root", root}, &stdout, &stderr)
	if rc != 0 || called || !strings.Contains(stdout.String(), "stage off, 1 record(s), 1 open, 0 transition(s)") || !strings.Contains(stdout.String(), "inc-2026-10-26-00000000 open") {
		t.Fatalf("rc=%d called=%v stdout=%s stderr=%s", rc, called, stdout.String(), stderr.String())
	}
}

func TestSelfhealIncidents_AFailedSourceIsAWarningAndThePassStillRuns(t *testing.T) {
	t.Parallel()
	root := selfhealProject(t, `{}`)
	evolveDir := paths.EvolveDirOf(root)
	selfhealFile(t, filepath.Join(evolveDir, "usage-windows.json"), "{")
	selfhealFile(t, filepath.Join(evolveDir, "waves"), "not a directory")
	selfhealFile(t, filepath.Join(evolveDir, "runs", "cycle-2040", "run.json"), "{")
	gh := func(context.Context, string, ...string) (string, error) { return "", errors.New("gh: not logged in") }
	rc, e, stderr := runSelfhealJSON(t, selfhealTestEnv(gh), root)
	if rc != 0 || len(e.Warnings) != 4 || !strings.Contains(stderr, "the CI rules did not run: gh: not logged in") {
		t.Fatalf("rc=%d warnings=%v stderr=%s", rc, e.Warnings, stderr)
	}
	for _, want := range []string{"the usage rule did not run", "no incident can close", "cycle-2040"} {
		if !slices.ContainsFunc(e.Warnings, func(w string) bool { return strings.Contains(w, want) }) {
			t.Errorf("warnings %v must name %q", e.Warnings, want)
		}
	}
	if got := selfhealFingerprints(e.Data.Incidents); !slices.Contains(got, "quality:QUALITY_PQ1_BREACH") || slices.Contains(got, "main-red:main") {
		t.Fatalf("the signal and cycle rules still run; the CI rules do not: %v", got)
	}
}

func TestSelfhealIncidents_AnUndecodableCIListOrAnUnreadableShipJournalIsAWarning(t *testing.T) {
	t.Parallel()
	root := selfhealProject(t, `{}`)
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), shipJournalFile), "{\n")
	gh := func(context.Context, string, ...string) (string, error) { return "not json", nil }
	rc, e, _ := runSelfhealJSON(t, selfhealTestEnv(gh), root)
	if rc != 0 || len(e.Warnings) != 2 || !strings.Contains(e.Warnings[0], "decode") || !strings.Contains(e.Warnings[1], "the cycle rules did not run") {
		t.Fatalf("rc=%d warnings=%v", rc, e.Warnings)
	}
}

func TestSelfhealIncidents_AnUnreadableRunsDirectoryIsAWarning(t *testing.T) {
	t.Parallel()
	root := selfhealProject(t, `{}`)
	runs := filepath.Join(paths.EvolveDirOf(root), "runs")
	if err := os.RemoveAll(runs); err != nil {
		t.Fatal(err)
	}
	selfhealFile(t, runs, "a file")
	rc, e, _ := runSelfhealJSON(t, selfhealTestEnv(fixtureGH), root)
	if rc != 0 || len(e.Warnings) != 1 || !strings.Contains(e.Warnings[0], "the cycle rules did not run") {
		t.Fatalf("rc=%d warnings=%v", rc, e.Warnings)
	}
}

func TestSelfhealIncidents_TheExitCodesOfTheContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T) []string
		rc    int
	}{
		{"no verb", func(*testing.T) []string { return []string{"--json"} }, exitUsage},
		{"an extra word", func(*testing.T) []string { return []string{"incidents", "now", "--json"} }, exitUsage},
		{"an unknown flag", func(*testing.T) []string { return []string{"incidents", "--bogus", "--json"} }, exitUsage},
		{"a bad policy", func(t *testing.T) []string {
			return []string{"incidents", "--json", "--project-root", selfhealProject(t, `{"healing":{"detector":{"stage":"enforce"}}}`)}
		}, selfhealExitIn},
		{"a corrupt record", func(t *testing.T) []string {
			root := selfhealProject(t, `{}`)
			selfhealFile(t, filepath.Join(selfheal.Dir(paths.EvolveDirOf(root)), "inc-bad.json"), "{")
			return []string{"incidents", "--json", "--project-root", root}
		}, exitIO},
		{"an unreadable channel", func(t *testing.T) []string {
			root := selfhealProject(t, `{}`)
			selfhealFile(t, filepath.Join(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "loop"), "a file")
			return []string{"incidents", "--json", "--project-root", root}
		}, exitIO},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		rc := runSelfhealWith(selfhealTestEnv(fixtureGH), c.setup(t), &stdout, &stderr)
		var e selfhealEnvelope
		if err := json.Unmarshal(stdout.Bytes(), &e); err != nil || rc != c.rc || e.OK || e.Error == nil || e.Error.Code != c.rc {
			t.Errorf("%s: rc=%d, want %d; envelope %s (%v)", c.name, rc, c.rc, stdout.String(), err)
		}
	}
}

func TestSelfhealIncidents_ARecordThatCannotBeWrittenIsAnIOError(t *testing.T) {
	t.Parallel()
	probe := selfhealProject(t, `{}`)
	_, e, _ := runSelfhealJSON(t, selfhealTestEnv(fixtureGH), probe)
	root := selfhealProject(t, `{}`)
	if err := os.MkdirAll(filepath.Join(selfheal.Dir(paths.EvolveDirOf(root)), e.Data.Incidents[0].ID+".json"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, failed, _ := runSelfhealJSON(t, selfhealTestEnv(fixtureGH), root)
	if rc != exitIO || failed.OK {
		t.Fatalf("rc=%d envelope=%+v", rc, failed)
	}
}

func TestRunSelfheal_IsTheRegisteredVerb(t *testing.T) {
	t.Parallel()
	cmd := lookupCommand("selfheal")
	if cmd == nil || !strings.Contains(cmd.Summary, "never acts") {
		t.Fatal("evolve selfheal must be a registered verb")
	}
	var stdout, stderr bytes.Buffer
	if rc := runSelfheal([]string{"bogus"}, nil, &stdout, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), selfhealUsage) {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
}

func TestNewSelfhealCenter_PublishesToTheSelfhealIncidentChannelUnderItsOwnRole(t *testing.T) {
	useRealRootEvents(t)
	root := t.TempDir()
	var stderr bytes.Buffer
	center, closeCenter := newSelfhealCenter(root, paths.EvolveDirOf(root), &stderr)
	inc := selfheal.Incident{ID: "inc-2026-10-27-0a1b2c3d", Class: selfheal.ClassMainRed, Severity: signalcenter.SeverityIncident, Tier: selfheal.TierPipeline, State: selfheal.StateOpen}
	center.Emit(selfheal.Transition{Kind: selfheal.TransitionOpened, Incident: inc}.Event("shadow"))
	closeCenter()
	events := selfhealChannelEvents(t, root)
	if len(events) != 1 || events[0].Fields["incident"] != inc.ID || strings.Contains(stderr.String(), "publishes no events") {
		t.Fatalf("the production center must publish under a known role: %+v\n%s", events, stderr.String())
	}
}
