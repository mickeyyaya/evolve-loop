package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

var classClock = time.Now

type classCycle struct {
	cycle int
	layer string
}

func sealClassCycle(t *testing.T, root string, c classCycle) FailureDigest {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(c.cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "failureclass", "cycle-"+itoa(c.cycle)+"-audit-fail-reason.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := AssembleFailureDigest(c.cycle, ws, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest(%d): %v", c.cycle, err)
	}
	d := validDisposition()
	d["cycle"], d["fingerprint"], d["recurrence"] = c.cycle, digest.Fingerprint, digest.Recurrence
	d["root_cause"] = map[string]any{"layer": c.layer, "summary": "the floor stops before the composed floor runs"}
	writeJSON(t, ws, "disposition.json", d)

	if err := (&Orchestrator{now: classClock}).finalizeRetroCompletion(ws); err != nil {
		t.Fatalf("finalizeRetroCompletion(%d): %v", c.cycle, err)
	}
	return readClassRecord(t, root, c.cycle)
}

func readClassRecord(t *testing.T, root string, cycle int) FailureDigest {
	t.Helper()
	var record, inWorkspace FailureDigest
	for path, into := range map[string]*FailureDigest{
		filepath.Join(root, ".evolve", "failure-digests", "cycle-"+itoa(cycle)+".json"):     &record,
		filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cycle), "failure-digest.json"): &inWorkspace,
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatal(err)
		}
	}
	if record.ClassFingerprint != inWorkspace.ClassFingerprint || record.ClassRecurrence != inWorkspace.ClassRecurrence {
		t.Fatalf("cycle %d: the record (%q %d) and the workspace digest (%q %d) disagree", cycle,
			record.ClassFingerprint, record.ClassRecurrence, inWorkspace.ClassFingerprint, inWorkspace.ClassRecurrence)
	}
	return record
}

func TestFailureClass_TheFloorShortCircuitCyclesShareOneClassAndTheThirdRoutesToTheConsole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	c2043 := sealClassCycle(t, root, classCycle{2043, "pipeline-code"})
	c2046 := sealClassCycle(t, root, classCycle{2046, "pipeline-code"})
	c2048 := sealClassCycle(t, root, classCycle{2048, "task-code"})
	c2055 := sealClassCycle(t, root, classCycle{2055, "pipeline-code"})

	if c2043.Fingerprint == c2046.Fingerprint || c2046.Fingerprint == c2055.Fingerprint {
		t.Fatalf("the exact fingerprints must stay distinct: %q %q %q", c2043.Fingerprint, c2046.Fingerprint, c2055.Fingerprint)
	}
	if c2043.ClassFingerprint == "" || c2043.ClassFingerprint != c2046.ClassFingerprint || c2046.ClassFingerprint != c2055.ClassFingerprint {
		t.Fatalf("class fingerprints = %q %q %q; want one shared class", c2043.ClassFingerprint, c2046.ClassFingerprint, c2055.ClassFingerprint)
	}
	if got := []int{c2043.ClassRecurrence, c2046.ClassRecurrence, c2055.ClassRecurrence}; got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("class recurrence = %v; want [1 2 3]", got)
	}
	if c2043.ClassRoute != "queue" || c2046.ClassRoute != "queue" {
		t.Errorf("routes of the first two = %q %q; want queue below the floor", c2043.ClassRoute, c2046.ClassRoute)
	}
	if c2055.ClassRoute != "console" || !strings.HasPrefix(c2055.ClassRouteReason, "floor: recurrence 3 >= 3") {
		t.Errorf("third route = %q (%q); want console with the recurrence floor reason", c2055.ClassRoute, c2055.ClassRouteReason)
	}
	if c2048.ClassFingerprint != "" || c2048.ClassRecurrence != 0 {
		t.Errorf("task-code cycle 2048 got class %q recurrence %d; want no class count", c2048.ClassFingerprint, c2048.ClassRecurrence)
	}
	if c2048.Fingerprint == "" {
		t.Error("task-code cycle 2048 lost its exact fingerprint")
	}
}

func TestFailureClass_TheExplanationFloorVariantIsADifferentClass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	c2043 := sealClassCycle(t, root, classCycle{2043, "pipeline-code"})
	c2054 := sealClassCycle(t, root, classCycle{2054, "pipeline-code"})

	if c2054.ClassFingerprint == "" || c2054.ClassFingerprint == c2043.ClassFingerprint {
		t.Fatalf("2054 class %q, 2043 class %q; want two different classes", c2054.ClassFingerprint, c2043.ClassFingerprint)
	}
	if c2054.ClassRecurrence != 1 {
		t.Errorf("2054 class recurrence = %d; want 1", c2054.ClassRecurrence)
	}
}

func TestFailingChecks_NamesTheFloorChecksThatFailed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		reasons []string
		want    string
	}{
		{"composed only", []string{"review gate: build handoff floor: 2 deterministic check failure(s) — fix:\n  sizeratchet: x\n  apicover naming floor: y"}, "composed-floor"},
		{"explanation only", []string{"build handoff floor: 2 deterministic check failure(s) — fix:\n  Explanation Documentation: a\n  Explanation Documentation: b"}, "explanation-documentation"},
		{"both", []string{"build handoff floor: 2 deterministic check failure(s) — fix:\n  Explanation Documentation: a\n  [ste-lint] b"}, "composed-floor,explanation-documentation"},
		{"two reasons", []string{"build handoff floor: 1 deterministic check failure(s) — fix:\n  [ste-lint] b", "build handoff floor: 1 deterministic check failure(s) — fix:\n  Explanation Documentation: a"}, "composed-floor,explanation-documentation"},
		{"not a floor reason", []string{"EGPS floor blocked ship: red_count=1"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(failingChecks(tc.reasons), ","); got != tc.want {
				t.Errorf("failingChecks = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestFailureClassKey_EachPartSplitsTheClass(t *testing.T) {
	t.Parallel()
	base := failureClassKey{phase: "build", preClass: "gate-block", checks: []string{"composed-floor"}, layer: "pipeline-code"}.fingerprint()
	if !strings.HasPrefix(base, "build|gate-block|class-") {
		t.Fatalf("class fingerprint %q; want the phase|pre-class|class-<hash> shape", base)
	}
	for name, other := range map[string]string{
		"phase":     failureClassKey{phase: "audit", preClass: "gate-block", checks: []string{"composed-floor"}, layer: "pipeline-code"}.fingerprint(),
		"pre-class": failureClassKey{phase: "build", preClass: "verdict-fail", checks: []string{"composed-floor"}, layer: "pipeline-code"}.fingerprint(),
		"checks":    failureClassKey{phase: "build", preClass: "gate-block", checks: []string{"explanation-documentation"}, layer: "pipeline-code"}.fingerprint(),
		"layer":     failureClassKey{phase: "build", preClass: "gate-block", checks: []string{"composed-floor"}, layer: "harness"}.fingerprint(),
	} {
		if other == base {
			t.Errorf("a different %s gives the same class %q", name, base)
		}
	}
}

func TestFailureClass_AResolvedClassStartsCountingAgain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := sealClassCycle(t, root, classCycle{2043, "pipeline-code"})
	if err := AppendResolvedFingerprint(filepath.Join(root, ".evolve"), first.ClassFingerprint, "fix-item", time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(root, ".evolve", "failure-digests", "cycle-2043.json"), time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	again := sealClassCycle(t, root, classCycle{2046, "pipeline-code"})

	if again.ClassRecurrence != 1 {
		t.Errorf("class recurrence after the resolution = %d; want 1, from the recorded time and not the file time", again.ClassRecurrence)
	}
}

func TestFailureClass_OutsideTheRunsLayoutRecordsNothing(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeMatchingDigest(t, ws)
	d := validDisposition()
	d["root_cause"] = map[string]any{"layer": "pipeline-code", "summary": "s"}
	writeJSON(t, ws, "disposition.json", d)

	got, err := recordFailureClass(ws, classClock)

	if err != nil || got.ClassFingerprint != "" {
		t.Fatalf("recordFailureClass = %+v, %v; want no class and no error outside .evolve/runs", got, err)
	}
}

func TestFailureClass_ErrorsAreReturned(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (root, ws string) {
		root, ws = layoutWorkspace(t, "2043")
		writeHarnessDigestRecord(t, root, "2043", "build|gate-block|aaaa", 0)
		d := validDisposition()
		d["root_cause"] = map[string]any{"layer": "infra", "summary": "s"}
		writeJSON(t, ws, "disposition.json", d)
		return root, ws
	}
	t.Run("missing disposition", func(t *testing.T) {
		_, ws := setup(t)
		if err := os.Remove(filepath.Join(ws, "disposition.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "disposition") {
			t.Fatalf("err = %v; want a disposition read error", err)
		}
	})
	t.Run("malformed disposition", func(t *testing.T) {
		_, ws := setup(t)
		if err := os.WriteFile(filepath.Join(ws, "disposition.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "disposition") {
			t.Fatalf("err = %v; want a disposition parse error", err)
		}
	})
	t.Run("malformed record", func(t *testing.T) {
		root, ws := setup(t)
		if err := os.WriteFile(filepath.Join(root, ".evolve", "failure-digests", "cycle-2043.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "digest record") {
			t.Fatalf("err = %v; want a record parse error", err)
		}
	})
	t.Run("malformed resolved ledger", func(t *testing.T) {
		root, ws := setup(t)
		if err := os.WriteFile(filepath.Join(root, ".evolve", "resolved-fingerprints.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "resolved-fingerprints") {
			t.Fatalf("err = %v; want a resolved ledger error", err)
		}
	})
	t.Run("unreadable prior record", func(t *testing.T) {
		root, ws := setup(t)
		writeJSON(t, filepath.Join(root, ".evolve", "failure-digests"), "cycle-1.json", "not a digest")
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "cycle-1.json") {
			t.Fatalf("err = %v; want the bad prior record named", err)
		}
	})
	t.Run("unwritable workspace digest", func(t *testing.T) {
		_, ws := setup(t)
		if err := os.Mkdir(filepath.Join(ws, "failure-digest.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil {
			t.Fatal("err = nil; want the workspace write error")
		}
	})
}

func TestFinalizeRetroCompletion_AClassRecordErrorWarnsAndKeepsTheGate(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "1034")
	writeHarnessDigestRecord(t, root, "1034", "audit|gate-block|egps", 0)
	d := validDisposition()
	d["root_cause"] = map[string]any{"layer": "harness", "summary": "s"}
	writeJSON(t, ws, "disposition.json", d)
	writeJSON(t, filepath.Join(root, ".evolve", "failure-digests"), "cycle-1.json", "not a digest")
	var warn strings.Builder
	o := &Orchestrator{stderr: &warn, now: classClock}

	err := o.finalizeRetroCompletion(ws)

	if err != nil {
		t.Fatalf("finalizeRetroCompletion = %v; want nil, because the disposition gate passed", err)
	}
	if !strings.Contains(warn.String(), "failure class") {
		t.Errorf("stderr = %q; want a failure class warning", warn.String())
	}
}

func TestBlockerBreaker_OneClassWithDistinctExactFingerprintsHalts(t *testing.T) {
	t.Parallel()
	digests := []FailureDigest{
		{Cycle: 2043, Fingerprint: "build|gate-block|0a48aa5747ca", PreClass: "gate-block", ClassFingerprint: "build|gate-block|class-1"},
		{Cycle: 2046, Fingerprint: "build|gate-block|c98f5503ae68", PreClass: "gate-block", ClassFingerprint: "build|gate-block|class-1"},
		{Cycle: 2048, Fingerprint: "build|gate-block|89fb584113df", PreClass: "gate-block"},
		{Cycle: 2055, Fingerprint: "build|gate-block|6e9e4730e355", PreClass: "gate-block", ClassFingerprint: "build|gate-block|class-1"},
	}

	v := EvaluateBlockerBreaker(digests, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3})

	if !v.Halt || v.Rule != "identical-fingerprint" || v.Fingerprint != "build|gate-block|class-1" || v.Count != 3 {
		t.Fatalf("verdict = %+v; want an identical-fingerprint halt on the class at count 3", v)
	}
	acked := EvaluateBlockerBreaker(digests, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3, AckedFingerprints: map[string]bool{"build|gate-block|class-1": true}})
	if acked.Halt {
		t.Errorf("verdict = %+v; want no halt once the class is resolved", acked)
	}
}

func classErrorWorkspace(t *testing.T) (digestsDir, ws string) {
	t.Helper()
	root, ws := layoutWorkspace(t, "2043")
	writeHarnessDigestRecord(t, root, "2043", "build|gate-block|aaaa", 0)
	d := validDisposition()
	d["root_cause"] = map[string]any{"layer": "pipeline-code", "summary": "s"}
	writeJSON(t, ws, "disposition.json", d)
	return filepath.Join(root, ".evolve", "failure-digests"), ws
}

func TestFailureClass_UnlistableOrUnwritableRecordsAreErrors(t *testing.T) {
	t.Parallel()
	for name, mode := range map[string]os.FileMode{"unlistable": 0o311, "unwritable": 0o555} {
		t.Run(name, func(t *testing.T) {
			dir, ws := classErrorWorkspace(t)
			if err := os.Chmod(dir, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

			if _, err := recordFailureClass(ws, classClock); err == nil {
				t.Fatalf("recordFailureClass = nil error; want an error for an %s record directory", name)
			}
		})
	}
}

func TestFailureClass_AMarshalErrorIsReturned(t *testing.T) {
	_, ws := classErrorWorkspace(t)
	orig := failureDigestMarshal
	failureDigestMarshal = func(any) ([]byte, error) { return nil, errors.New("encoder refused") }
	t.Cleanup(func() { failureDigestMarshal = orig })

	if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "encoder refused") {
		t.Fatalf("recordFailureClass = %v; want the marshal error", err)
	}
}

func TestLoadResolvedFingerprintTimes_TheLatestResolutionWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	early := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	late := early.Add(48 * time.Hour)
	for _, at := range []time.Time{late, early} {
		if err := AppendResolvedFingerprint(dir, "build|gate-block|class-1", "fix", at); err != nil {
			t.Fatal(err)
		}
	}

	got, err := loadResolvedFingerprintTimes(dir)

	if err != nil || !got["build|gate-block|class-1"].Equal(late) {
		t.Fatalf("loadResolvedFingerprintTimes = %v, %v; want the latest resolution %v", got, err, late)
	}
}

func TestLoadResolvedFingerprintTimes_SkipsARecordWithoutAFingerprint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeJSON(t, dir, "resolved-fingerprints.json", []map[string]string{{"fingerprint": ""}, {"fingerprint": "a|b|c"}})

	got, err := loadResolvedFingerprintTimes(dir)

	if err != nil || len(got) != 1 {
		t.Fatalf("loadResolvedFingerprintTimes = %v, %v; want only the record with a fingerprint", got, err)
	}
}

func TestAssembleFailureDigest_ALaterAssemblyKeepsTheRecordedClassOfTheSameFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classed := sealClassCycle(t, root, classCycle{2043, "pipeline-code"})
	ws := filepath.Join(root, ".evolve", "runs", "cycle-2043")

	(&Orchestrator{now: classClock}).ensureFailureDigest(2043, root, ws, "retro", "cycle aborted in phase retro (abnormal-exit epilogue)")

	again := readClassRecord(t, root, 2043)
	if again.ClassFingerprint != classed.ClassFingerprint || again.ClassLayer != "pipeline-code" || again.ClassRecurrence != 1 || again.ClassRoute != classed.ClassRoute {
		t.Fatalf("after the epilogue assembly the class is %+v; want the recorded class %+v", again, classed)
	}

	writeAuditFailReason(t, ws, "build", "another failure")
	if _, err := AssembleFailureDigest(2043, ws, nil); err != nil {
		t.Fatal(err)
	}
	if changed := readClassRecord(t, root, 2043); changed.ClassFingerprint != "" || changed.ClassRecurrence != 0 {
		t.Errorf("a new failure identity kept the old class %+v", changed)
	}
}

func TestFailureClass_TheCountSkipsTemporaryFilesAndDirectories(t *testing.T) {
	t.Parallel()
	dir, ws := classErrorWorkspace(t)
	if err := os.WriteFile(filepath.Join(dir, "cycle-7.json.tmp"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "archive.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := recordFailureClass(ws, classClock)

	if err != nil || got.ClassRecurrence != 1 {
		t.Fatalf("recordFailureClass = %+v, %v; want class recurrence 1 with the partial write and the directory skipped", got, err)
	}
}

func TestFailingChecks_ReadsTheReasonThatTheFloorWrites(t *testing.T) {
	t.Parallel()
	var calls []string
	cases := map[string]BuildHandoffFloor{
		"explanation-documentation":                {recordingCheck(explanationFloorCheckName, &calls, explanationdocs.FailurePrefix+"cited path a.go is not in the Build diff")},
		composedFloorCheckName:                     {recordingCheck("production", &calls, "sizeratchet: x grew", "[ste-lint] y")},
		"composed-floor,explanation-documentation": {recordingCheck(explanationFloorCheckName, &calls, explanationdocs.FailurePrefix+"z"), recordingCheck("production", &calls, "coverage below 100%")},
	}
	for want, floor := range cases {
		rr := floor.Review(context.Background(), ReviewInput{Phase: string(PhaseBuild)})
		if got := strings.Join(failingChecks([]string{"review gate: phase \"build\" deliverable rejected after 1 correction(s): " + rr.Reason}), ","); got != want {
			t.Errorf("failingChecks(%q) = %q; want %q", rr.Reason, got, want)
		}
	}
}

func TestFailureClass_TheThirdFailureStagesOneConsoleEscalation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, c := range []int{2043, 2046} {
		sealClassCycle(t, root, classCycle{c, "pipeline-code"})
	}
	pending := dispositionrouter.PendingActionsPath(filepath.Join(root, ".evolve", "escalations"))
	if intents, err := dispositionrouter.LoadIntents(pending); err != nil || len(intents) != 0 {
		t.Fatalf("intents after two failures = %+v, %v; want none below the floor", intents, err)
	}

	third := sealClassCycle(t, root, classCycle{2055, "pipeline-code"})

	intents, err := dispositionrouter.LoadIntents(pending)
	if err != nil || len(intents) != 1 {
		t.Fatalf("intents = %+v, %v; want one", intents, err)
	}
	in := intents[0]
	_, hash, _ := strings.Cut(third.ClassFingerprint, "|class-")
	if in.Action != dispositionrouter.ActionAutofile || in.Route != dispositionrouter.RouteConsole || in.ItemID != "failure-class-"+hash ||
		in.Pattern != third.ClassFingerprint || in.Recurrence != 3 || in.Cycle != 2055 || in.Weight != 0.75 || in.PriorityClass != "correctness" || in.Reason != third.ClassRouteReason {
		t.Errorf("intent = %+v; want an autofile to the console for the class", in)
	}
}

func TestFailureClass_AConsoleRoutingRaisesTheFirstFailure(t *testing.T) {
	t.Parallel()
	_, ws := classErrorWorkspace(t)
	d := validDisposition()
	d["root_cause"], d["routing"] = map[string]any{"layer": "pipeline-code", "summary": "s"}, "console"
	writeJSON(t, ws, "disposition.json", d)

	got, err := recordFailureClass(ws, classClock)

	if err != nil || got.ClassRecurrence != 1 || got.ClassRoute != "console" || got.ClassRouteReason != "advisory raise: queue -> console" {
		t.Fatalf("recordFailureClass = %+v, %v; want the advisory console raise at recurrence 1", got, err)
	}
}

func TestFailureClass_EscalationFaultsAreReturned(t *testing.T) {
	t.Parallel()
	t.Run("an unreadable policy", func(t *testing.T) {
		dir, ws := classErrorWorkspace(t)
		d := validDisposition()
		d["root_cause"], d["routing"] = map[string]any{"layer": "pipeline-code", "summary": "s"}, "console"
		writeJSON(t, ws, "disposition.json", d)
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "policy.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "autofile weight") {
			t.Fatalf("err = %v; want the policy error", err)
		}
	})
	t.Run("an unwritable escalations directory", func(t *testing.T) {
		dir, ws := classErrorWorkspace(t)
		d := validDisposition()
		d["root_cause"], d["routing"] = map[string]any{"layer": "pipeline-code", "summary": "s"}, "console"
		writeJSON(t, ws, "disposition.json", d)
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "escalations"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := recordFailureClass(ws, classClock); err == nil || !strings.Contains(err.Error(), "escalations") {
			t.Fatalf("err = %v; want the staging error", err)
		}
	})
}

func TestBlockerBreaker_AnAckOfTheExactFingerprintRemovesAClassedDigest(t *testing.T) {
	t.Parallel()
	digests := []FailureDigest{
		{Cycle: 1, Fingerprint: "build|gate-block|a", PreClass: "gate-block", ClassFingerprint: "build|gate-block|class-1"},
		{Cycle: 2, Fingerprint: "build|gate-block|b", PreClass: "gate-block", ClassFingerprint: "build|gate-block|class-1"},
		{Cycle: 3, Fingerprint: "build|gate-block|c", PreClass: "gate-block", ClassFingerprint: "build|gate-block|class-1"},
	}

	v := EvaluateBlockerBreaker(digests, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3, AckedFingerprints: map[string]bool{"build|gate-block|b": true}})

	if v.Halt {
		t.Fatalf("verdict = %+v; want no halt, because the ack of one exact fingerprint leaves 2 of the class", v)
	}
}

func TestLoadResolvedFingerprintTimes_AMalformedTimeIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeJSON(t, dir, "resolved-fingerprints.json", []map[string]string{{"fingerprint": "a|b|c"}, {"fingerprint": "d|e|f", "resolved_at": "yesterday"}})

	got, err := loadResolvedFingerprintTimes(dir)

	if err == nil || !strings.Contains(err.Error(), "d|e|f") {
		t.Fatalf("loadResolvedFingerprintTimes = %v, %v; want an error that names the record", got, err)
	}
}

func TestFailingChecks_APrefixInsideAFailureCountsOnce(t *testing.T) {
	t.Parallel()
	r := floorReasonHead + "2" + floorReasonCount + " — fix:\n  " + explanationdocs.FailurePrefix + "build-report.md Explanation Documentation: bad field\n  [ste-lint] b"

	if got := strings.Join(failingChecks([]string{r}), ","); got != "composed-floor,explanation-documentation" {
		t.Fatalf("failingChecks = %q; want both checks, because only a line that starts with the prefix is an explanation failure", got)
	}
}

func TestFailureClass_ARefinalizedCycleDoesNotCountItself(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sealClassCycle(t, root, classCycle{2043, "pipeline-code"})

	again := sealClassCycle(t, root, classCycle{2043, "pipeline-code"})

	if again.ClassRecurrence != 1 {
		t.Fatalf("class recurrence after a second retro completion = %d; want 1", again.ClassRecurrence)
	}
}

func TestFailureDigestRecordPath_RefusesANonCanonicalCycleName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"cycle-007", "cycle-+5", "cycle-x", "run-7"} {
		if path, ok := failureDigestRecordPath(filepath.Join(root, ".evolve", "runs", name)); ok {
			t.Errorf("%s: record path %q; want none", name, path)
		}
	}
	if path, ok := failureDigestRecordPath(filepath.Join(root, ".evolve", "runs", "cycle-7")); !ok || filepath.Base(path) != "cycle-7.json" {
		t.Errorf("cycle-7: %q %v; want cycle-7.json", path, ok)
	}
}
