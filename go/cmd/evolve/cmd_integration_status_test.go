package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const statusSha = "1111111111111111111111111111111111111111"

func cleanEvidence() integrationStatusEvidence {
	return integrationStatusEvidence{
		Found: true,
		Record: integration.WaveRecord{
			Pool: 5, Epoch: 2, ShadowBase: statusSha, Ended: true, LastCycle: 9,
			Members:   map[int]integration.MemberState{6: {Status: integration.MemberComposed}},
			Decisions: []integration.FlushRecord{{Trigger: integration.TriggerWaveEnd}},
		},
		Shipped: []int{6},
	}
}

func TestIntegrationVerdict_EachConditionOfCleanFailsOnItsOwn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*integrationStatusEvidence)
		want   string
		failed string
	}{
		{"the unchanged evidence", func(*integrationStatusEvidence) {}, verdictClean, ""},
		{"no wave record", func(e *integrationStatusEvidence) { *e = integrationStatusEvidence{} }, verdictNotClean, "no wave record"},
		{"not ended", func(e *integrationStatusEvidence) { e.Record.Ended = false }, verdictNotClean, "has no Ended"},
		{"no wave-end decision", func(e *integrationStatusEvidence) { e.Record.Decisions = nil }, verdictNotClean, "no wave-end decision"},
		{"another trigger only", func(e *integrationStatusEvidence) {
			e.Record.Decisions = []integration.FlushRecord{{Trigger: integration.TriggerCapReached}}
		}, verdictNotClean, "no wave-end decision"},
		{"no last cycle", func(e *integrationStatusEvidence) { e.Record.LastCycle = 0 }, verdictNotClean, "no last cycle"},
		{"a shipped cycle with no member", func(e *integrationStatusEvidence) { e.Shipped = []int{6, 7} }, verdictNotClean, "missing record: cycle 7 shipped and has no member"},
		{"a driver failure", func(e *integrationStatusEvidence) {
			e.Record.Failures = []integration.DriverFailure{{Step: "flush", Error: "boom"}}
		}, verdictNotClean, "driver failure at flush: boom"},
		{"an unreadable source", func(e *integrationStatusEvidence) { e.ReadFailures = []string{"runs/7: denied"} }, verdictNotClean, "unreadable: runs/7: denied"},
		{"zero composes", func(e *integrationStatusEvidence) {
			e.Record.Members = map[int]integration.MemberState{}
			e.Shipped = nil
		}, verdictNoEvidence, ""},
		{"only failed members", func(e *integrationStatusEvidence) {
			e.Record.Members = map[int]integration.MemberState{6: {Status: integration.MemberFailed}, 7: {Status: integration.MemberFailed}}
			e.Shipped = []int{6, 7}
		}, verdictNoEvidence, ""},
		{"a shipped member still pending at wave end", func(e *integrationStatusEvidence) {
			e.Record.Members[7] = integration.MemberState{Status: integration.MemberPending}
			e.Shipped = []int{6, 7}
		}, verdictNotClean, "cycle 7 is still pending at wave end"},
		{"a pending member of a wave that has not ended names only the missing end", func(e *integrationStatusEvidence) {
			e.Record.Ended = false
			e.Record.Members[7] = integration.MemberState{Status: integration.MemberPending}
			e.Shipped = []int{6, 7}
		}, verdictNotClean, "has no Ended"},
		{"a shadow conflict does not stop clean", func(e *integrationStatusEvidence) {
			e.Record.Members = map[int]integration.MemberState{6: {Status: integration.MemberConflicted}}
		}, verdictClean, ""},
		{"a base-ahead conflict counts as a compose", func(e *integrationStatusEvidence) {
			e.Record.Members = map[int]integration.MemberState{6: {Status: integration.MemberConflictedBaseAhead}}
		}, verdictClean, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev := cleanEvidence()
			tc.mutate(&ev)
			verdict, failed := integrationVerdict(ev)
			if verdict != tc.want {
				t.Fatalf("verdict = %q (%v), want %q", verdict, failed, tc.want)
			}
			if tc.failed != "" && !strings.Contains(strings.Join(failed, "\n"), tc.failed) {
				t.Fatalf("failed = %v, want it to name %q", failed, tc.failed)
			}
			if tc.want != verdictNotClean && len(failed) != 0 {
				t.Fatalf("failed = %v on a %s wave", failed, tc.want)
			}
		})
	}
}

func TestBuildIntegrationStatus_CountsMembersAndPrintsTheFixedNotes(t *testing.T) {
	t.Parallel()
	ev := cleanEvidence()
	ev.Record.Members = map[int]integration.MemberState{
		6: {Status: integration.MemberComposed, Landed: true},
		7: {Status: integration.MemberConflicted},
		8: {Status: integration.MemberConflictedBaseAhead},
	}
	ev.Shipped = []int{6, 7, 8}

	rep := buildIntegrationStatus(5, "shadow", ev)

	want := map[string]int{"members": 3, "composed": 1, "conflicted": 1, "conflicted_base_ahead": 1, "landed": 1, "shipped": 3}
	for k, v := range want {
		if rep.Counts[k] != v {
			t.Errorf("counts[%s] = %d, want %d (%v)", k, rep.Counts[k], v, rep.Counts)
		}
	}
	if len(rep.Members) != 3 || rep.Members[0].Cycle != 6 || rep.Members[2].Cycle != 8 || !rep.Members[0].Landed {
		t.Errorf("members = %+v, want them in cycle order", rep.Members)
	}
	if rep.Order != "order: by delivery time; rank, risk and deps not supplied" || rep.Enforce != "enforce: not ready (no enforce actor)" {
		t.Errorf("notes = %q, %q", rep.Order, rep.Enforce)
	}
}

func TestCountsLine_IsSortedByName(t *testing.T) {
	t.Parallel()
	if got := countsLine(map[string]int{"members": 2, "composed": 1}); got != "composed=1 members=2" {
		t.Fatalf("countsLine = %q", got)
	}
}

type statusRig struct {
	root  string
	store integration.FileWaveStore
}

func newStatusRig(t *testing.T) statusRig {
	t.Helper()
	root := t.TempDir()
	return statusRig{root: root, store: integration.FileWaveStore{Root: root}}
}

func (r statusRig) writePool(t *testing.T, pool int, mutate func(*integration.WaveRecord)) {
	t.Helper()
	rec := integration.WaveRecord{Pool: pool, Epoch: 2, ShadowBase: statusSha, Ended: true, LastCycle: pool + 3,
		Members:   map[int]integration.MemberState{pool + 1: {Status: integration.MemberComposed}},
		Decisions: []integration.FlushRecord{{Trigger: integration.TriggerWaveEnd, At: time.Unix(0, 0).UTC()}}}
	if mutate != nil {
		mutate(&rec)
	}
	if err := r.store.Write(rec); err != nil {
		t.Fatal(err)
	}
}

func (r statusRig) run(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := statusVerb(integrationEnv{Root: r.root}, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestStatusVerb_RejectsBadSelectorsWithTheUsage(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	for _, args := range [][]string{
		{"extra"}, {"--pool"}, {"--pool", "x"}, {"--pool", "0"}, {"--wave", "x"}, {"--wave", "0"}, {"--pool", "1", "--wave", "1"},
	} {
		code, stdout, stderr := r.run(args...)
		if code != exitUsage || stdout != "" || !strings.Contains(stderr, integrationStatusUsage) {
			t.Errorf("args %v = %d, stdout %q, stderr %q, want usage", args, code, stdout, stderr)
		}
	}
}

func TestStatusVerb_NoWaveRecordIsARefusalThatNamesTheRoot(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	code, _, stderr := r.run()
	if code != exitRefused || !strings.Contains(stderr, "no wave record exists under "+r.root) {
		t.Fatalf("= %d, %q", code, stderr)
	}
}

func TestStatusVerb_DefaultsToTheLatestPoolAndPrintsTheText(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	r.writePool(t, 12, func(rec *integration.WaveRecord) {
		rec.Batches = []integration.IntegrationBatch{{Index: 1, Base: statusSha, Branch: "integration/p12-b1", Tip: statusSha, Members: []int{13}}}
		rec.Members[13] = integration.MemberState{Status: integration.MemberComposed, Landed: true}
	})

	code, stdout, stderr := r.run()

	if code != 0 || stderr != "" {
		t.Fatalf("= %d, stderr %q\n%s", code, stderr, stdout)
	}
	for _, want := range []string{
		"pool 12: stage shadow, epoch 2, shadow base 1111111, ended true, last cycle 15",
		"verdict: clean",
		"order: by delivery time; rank, risk and deps not supplied",
		"enforce: not ready (no enforce actor)",
		"counts: composed=1 landed=1 members=1 shipped=0",
		"  cycle 13: composed, landed",
		"  batch 1: integration/p12-b1 base 1111111 tip 1111111 members [13]",
		"  decision wave_end: flush [] hold []",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
}

func TestStatusVerb_PoolSelectsOneAndAJSONReportDecodes(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, func(rec *integration.WaveRecord) { rec.Ended = false })
	r.writePool(t, 12, nil)

	code, stdout, _ := r.run("--pool", "5", "--json")

	var rep integrationStatusReport
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if code != exitRefused || rep.Pool != 5 || rep.Verdict != verdictNotClean || rep.Ended || rep.Stage != "shadow" {
		t.Fatalf("code %d report %+v", code, rep)
	}
	if !strings.Contains(strings.Join(rep.Failed, "\n"), "has no Ended") {
		t.Errorf("failed = %v", rep.Failed)
	}
}

func TestStatusVerb_AMissingPoolIsNotCleanAndAnUnreadableRecordIsNamed(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	code, stdout, _ := r.run("--pool", "9", "--json")
	if code != exitRefused || !strings.Contains(stdout, "no wave record") {
		t.Fatalf("missing pool = %d\n%s", code, stdout)
	}
	if err := os.WriteFile(integration.WaveRecordPath(r.root, 5), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = r.run("--pool", "5")
	if code != exitRefused || !strings.Contains(stdout, "failed: unreadable: ") {
		t.Fatalf("torn record = %d\n%s", code, stdout)
	}
}

func TestStatusVerb_AnInvalidStageIsPrintedNotHidden(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	if err := os.MkdirAll(filepath.Join(r.root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.root, ".evolve", "policy.json"), []byte(`{"fleet":{"integration":{"stage":"sideways"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stdout, _ := r.run()
	if !strings.Contains(stdout, "stage invalid: ") {
		t.Fatalf("output:\n%s", stdout)
	}
}

func (r statusRig) seedWaves(t *testing.T, floors ...int) {
	t.Helper()
	store := wave.NewStore(filepath.Join(r.root, ".evolve"))
	for i, f := range floors {
		if err := store.Save(wave.Record{Number: i + 1, CycleFloor: f}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStatusVerb_WaveResolvesItsPoolsThroughTheWaveStore(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 0, 20, 40)
	r.writePool(t, 5, nil)
	r.writePool(t, 12, nil)
	r.writePool(t, 25, nil)

	code, stdout, stderr := r.run("--wave", "1", "--json")

	var set integrationStatusWaves
	if err := json.Unmarshal([]byte(stdout), &set); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if code != 0 || stderr != "" || set.Wave != 1 || len(set.Pools) != 2 || set.Pools[0].Pool != 5 || set.Pools[1].Pool != 12 || set.Pools[0].Wave != 1 {
		t.Fatalf("code %d set %+v", code, set)
	}
	code, stdout, stderr = r.run("--wave", "3")
	if code != exitRefused || strings.Contains(stdout, "pool 25") || !strings.Contains(stderr, "wave 3 has no pool with a wave record") {
		t.Fatalf("wave 3 holds no pool: %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestStatusVerb_ALastWaveHasNoCeilingAndAWaveWorstVerdictDecidesTheExit(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 0, 20)
	r.writePool(t, 25, nil)
	r.writePool(t, 30, func(rec *integration.WaveRecord) { rec.Ended = false })

	code, stdout, _ := r.run("--wave", "2")
	if code != exitRefused || !strings.Contains(stdout, "pool 25 (wave 2)") || !strings.Contains(stdout, "pool 30 (wave 2)") {
		t.Fatalf("= %d\n%s", code, stdout)
	}
	code, _, _ = r.run("--wave", "2", "--json")
	if code != exitRefused {
		t.Fatalf("json = %d, want the worst verdict to decide", code)
	}
}

func TestStatusVerb_AWaveWithoutARecordOrWithoutPoolsIsRefused(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 0)
	if code, _, stderr := r.run("--wave", "9"); code != exitIO || !strings.Contains(stderr, "wave 9 has no record") {
		t.Fatalf("unknown wave = %d, %q", code, stderr)
	}
	if code, _, stderr := r.run("--wave", "1"); code != exitRefused || !strings.Contains(stderr, "wave 1 has no pool with a wave record") {
		t.Fatalf("empty wave = %d, %q", code, stderr)
	}
}

func TestWaveBounds_ANextFloorThatDoesNotRiseGivesTheWaveNoPool(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 0, 20, 20)
	r.writePool(t, 25, nil)

	list, err := waveBoundsList(r.root)

	if err != nil || len(list) != 3 || list[0].Ceiling != 20 || list[1].Ceiling != 20 || list[2].Ceiling != 0 {
		t.Fatalf("bounds = %+v, %v, want a ceiling wherever the next record has a floor", list, err)
	}
	if got := waveOfPool(r.root, 25); got != 3 {
		t.Fatalf("pool 25 reads as wave %d, want wave 3", got)
	}
}

func TestWaveOfPool_IsZeroWithoutAWaveStore(t *testing.T) {
	t.Parallel()
	if got := waveOfPool(t.TempDir(), 5); got != 0 {
		t.Fatalf("wave = %d, want 0", got)
	}
}

func TestPrintIntegrationJSON_ReportsAnUnencodableValue(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := printIntegrationJSON(make(chan int), 0, &out, &errb); code != exitIO || !strings.Contains(errb.String(), "encode the status") {
		t.Fatalf("= %d, %q", code, errb.String())
	}
}

func (r statusRig) shipCycle(t *testing.T, cycle int) {
	t.Helper()
	evolveDir := filepath.Join(r.root, ".evolve")
	in := landing.Intent{Cycle: cycle, CommitSHA: statusSha, Status: landing.IntentComplete}
	if err := landing.WriteIntent(landing.IntentPath(evolveDir, cycle), in); err != nil {
		t.Fatal(err)
	}
}

func TestStatusVerb_AShippedCycleOfTheRangeWithNoMemberIsMissingItsRecord(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	r.shipCycle(t, 6)
	r.shipCycle(t, 8)

	code, stdout, _ := r.run("--pool", "5", "--json")

	var rep integrationStatusReport
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(rep.Failed, "\n")
	if code != exitRefused || rep.Verdict != verdictNotClean || !strings.Contains(joined, "missing record: cycle 8 shipped and has no member") || strings.Contains(joined, "cycle 6") {
		t.Fatalf("code %d report %+v, want only cycle 8 missing", code, rep)
	}
	if rep.Counts["shipped"] != 2 {
		t.Errorf("shipped = %d, want 2", rep.Counts["shipped"])
	}
}

func TestStatusVerb_AShippedCycleAboveTheLastCycleIsOutsideTheRange(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	r.shipCycle(t, 6)
	r.shipCycle(t, 50)

	code, stdout, _ := r.run("--pool", "5")

	if code != 0 || !strings.Contains(stdout, "verdict: clean") {
		t.Fatalf("code %d\n%s", code, stdout)
	}
}

func TestStatusVerb_AnUnreadableLandingIntentOfTheRangeFailsTheVerdict(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	evolveDir := filepath.Join(r.root, ".evolve")
	path := landing.IntentPath(evolveDir, 6)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := r.run("--pool", "5")

	if code != exitRefused || !strings.Contains(stdout, "failed: unreadable: ") {
		t.Fatalf("code %d\n%s", code, stdout)
	}
}

func TestStatusVerb_ATornWaveStoreIsAnIOFailureForWaveAndOmitsTheWaveNumberOtherwise(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	dir := filepath.Join(r.root, ".evolve", "waves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wave-1.json"), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := r.run("--wave", "1"); code != exitIO || !strings.Contains(stderr, "wave") {
		t.Fatalf("--wave = %d, %q", code, stderr)
	}
	code, stdout, _ := r.run("--pool", "5")
	if code != 0 || strings.Contains(stdout, "(wave") {
		t.Fatalf("--pool = %d\n%s", code, stdout)
	}
}

func TestWaveOfPool_IsZeroForAPoolNoWaveHolds(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 100)
	if got := waveOfPool(r.root, 5); got != 0 {
		t.Fatalf("wave = %d, want 0 for a pool below every floor", got)
	}
	if got := waveOfPool(r.root, 150); got != 1 {
		t.Fatalf("wave = %d, want 1", got)
	}
}
