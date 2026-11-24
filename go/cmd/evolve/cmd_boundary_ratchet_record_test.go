package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

type fakeRecorder struct {
	err error
}

func (r *fakeRecorder) install(t *testing.T) {
	t.Helper()
	prev := ratchetRecord
	t.Cleanup(func() { ratchetRecord = prev })
	ratchetRecord = r.record
}

func (r *fakeRecorder) record(req ratchetRecordRequest) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	binding, err := req.bind(integration.Evidence{Commits: []string{req.head}, Paths: []string{ratchetListPath, rawGitBaselinePath}, DiffSHA: "d"})
	if err != nil {
		return "", err
	}
	binding.ArtifactSHA = strings.Repeat("a", 64)
	binding.TreeSHA = strings.Repeat("1", 40)
	rec := integration.DeliverableRecord{
		Schema: integration.RecordSchema, Source: integration.SourceConsolePR, Wave: integration.ConsoleWave, PR: req.pr,
		Commits: []integration.CommitIdentity{{SHA: req.head, BaseSHA: strings.Repeat("2", 40), PatchID: "p"}}, Audit: binding,
	}
	return integration.WriteRecord(integration.WaveDir(req.root, integration.ConsoleWave), rec)
}

func (o ratchetOrigin) policy(t *testing.T, raw string) {
	t.Helper()
	dir := filepath.Join(o.plane.Dir, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (o ratchetOrigin) stage(t *testing.T, word string) {
	t.Helper()
	o.policy(t, `{"boundary":{"ratchet_merge":{"stage":"`+word+`"}}}`)
}

func (o ratchetOrigin) decisions(t *testing.T) []signalcenter.Event {
	t.Helper()
	var out []signalcenter.Event
	for _, e := range readProjectSignals(t, filepath.Join(o.plane.Dir, ".evolve")) {
		if e.Code == signalcenter.CodeCLIRatchetMergeDecision {
			out = append(out, e)
		}
	}
	return out
}

func (o ratchetOrigin) onlyDecision(t *testing.T) signalcenter.Event {
	t.Helper()
	got := o.decisions(t)
	if len(got) != 1 {
		t.Fatalf("decision signals = %d, want exactly 1: %+v", len(got), got)
	}
	return got[0]
}

func (o ratchetOrigin) listing(number int) func([]string) string {
	return func(create []string) string {
		branch := flagAfter(create, "--head")
		return fmt.Sprintf(`[{"number":%d,"headRefName":%q,"headRefOid":%q}]`, number, branch, o.bare.Git("rev-parse", branch))
	}
}

func (o ratchetOrigin) recordFor(pr int) (integration.DeliverableRecord, bool) {
	return integration.FindPRRecord(o.plane.Dir, pr)
}

func (o ratchetOrigin) firstRun(t *testing.T, word string) (branch, head string, gh *fakeGH) {
	t.Helper()
	o.stage(t, word)
	gh = &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	if rc, _, errOut := o.tighten(t); rc != 0 || strings.Contains(errOut, "WARN") {
		t.Fatalf("first run rc=%d stderr=%q", rc, errOut)
	}
	branch = flagAfter(gh.created(), "--head")
	return branch, o.bare.Git("rev-parse", branch), gh
}

func (o ratchetOrigin) rerun(t *testing.T, branch string, number int) (stdout, stderr string, gh *fakeGH) {
	t.Helper()
	head := o.bare.Git("rev-parse", branch)
	gh = &fakeGH{list: fmt.Sprintf(`[{"number":%d,"headRefName":%q,"headRefOid":%q}]`, number, branch, head)}
	gh.install(t)
	rc, out, errOut := o.tighten(t)
	if rc != 0 {
		t.Fatalf("rerun rc=%d stderr=%q", rc, errOut)
	}
	return out, errOut, gh
}

func fields(e signalcenter.Event) map[string]string { return e.Fields }

func TestRatchetRecord_EnforceRecordsThePRWithTheListsClass(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)

	branch, head, _ := o.firstRun(t, "enforce")

	rec, ok := o.recordFor(31)
	if !ok || rec.Audit.Class != integration.RatchetListsClass || rec.Commits[0].SHA != head {
		t.Fatalf("record %+v ok=%v, want class %s bound to head %s of %s", rec.Audit, ok, integration.RatchetListsClass, head, branch)
	}
	e := o.onlyDecision(t)
	f := fields(e)
	if f["decision"] != "recorded" || f["pr"] != "31" || f["head"] != head || f["stage"] != "enforce" || f["skipped_push"] != "false" || f["reason"] != "" {
		t.Errorf("signal fields %v", f)
	}
	if e.Kind != signalcenter.KindGatePassed {
		t.Errorf("kind %s, want %s", e.Kind, signalcenter.KindGatePassed)
	}
}

func TestRatchetRecord_ShadowRecordsNothingAndSaysWouldRecord(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "shadow")
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "ratchet record: would record #31") {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, out, errOut)
	}
	if _, ok := o.recordFor(31); ok {
		t.Error("shadow wrote a record")
	}
	if f := fields(o.onlyDecision(t)); f["decision"] != "would_record" || f["stage"] != "shadow" {
		t.Errorf("signal fields %v", f)
	}
}

func TestRatchetRecord_AbsentStageIsShadow(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	o.tighten(t)

	if _, ok := o.recordFor(31); ok {
		t.Error("an absent policy recorded the PR")
	}
	if f := fields(o.onlyDecision(t)); f["stage"] != "shadow" || f["decision"] != "would_record" {
		t.Errorf("signal fields %v", f)
	}
}

func TestRatchetRecord_OffRecordsNothingEmitsNothingAndStillProposes(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "off")
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, out, _ := o.tighten(t)

	if rc != 0 || !strings.Contains(out, "opened a pull request") || strings.Contains(out, "ratchet record") {
		t.Fatalf("rc=%d stdout=%q, want the PR opened and no record line", rc, out)
	}
	if _, ok := o.recordFor(31); ok || len(o.decisions(t)) != 0 {
		t.Errorf("off recorded or signalled")
	}
}

func TestRatchetRecord_UnreadablePolicyRecordsNothingAndSaysSo(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.policy(t, `{"boundary":{"ratchet_merge":{"stage":"sometimes"}}}`)
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, out, _ := o.tighten(t)

	f := fields(o.onlyDecision(t))
	if _, ok := o.recordFor(31); rc != 0 || ok || f["decision"] != "no_record" || f["reason"] != "policy_unreadable" || !strings.Contains(out, "opened a pull request") {
		t.Errorf("rc=%d fields=%v stdout=%q", rc, f, out)
	}
}

func TestRatchetRecord_ARedScanRecordsNothingPushesNothingAndNamesTheScan(t *testing.T) {
	for name, tc := range map[string]struct {
		offenders, rawgit, reason string
	}{
		"size":   {`{"internal/fixture.Big": 60, "internal/fixture.Gone": 60}`, `{"internal/fixture/raw_test.go": 1}`, "scan_size_red"},
		"rawgit": {`{"internal/fixture.Big": 90}`, `{}`, "scan_rawgit_red"},
	} {
		t.Run(name, func(t *testing.T) {
			o := newRatchetOriginWith(t, 60, tc.offenders, tc.rawgit)
			(&fakeRecorder{}).install(t)
			o.stage(t, "enforce")
			gh := &fakeGH{list: "[]"}
			gh.install(t)
			t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

			rc, _, errOut := o.tighten(t)

			f := fields(o.onlyDecision(t))
			if rc != 0 || !strings.Contains(errOut, "WARN") || f["decision"] != "no_record" || f["reason"] != tc.reason || f["scans"] != tc.reason {
				t.Errorf("rc=%d stderr=%q fields=%v, want %s", rc, errOut, f, tc.reason)
			}
			if heads := o.bare.Git("for-each-ref", "--format=%(refname)", "refs/heads/"+ratchetBranchPrefix); heads != "" || gh.created() != nil {
				t.Errorf("a red scan pushed %q or opened a PR", heads)
			}
		})
	}
}

func TestRatchetRecord_ACandidateThatIsNotCleanRecordsNothing(t *testing.T) {
	cases := map[string]struct{ list, reason string }{
		"two candidates": {`[{"number":7,"headRefName":"chore/ratchet-tighten-a","headRefOid":"` + strings.Repeat("a", 40) + `"},{"number":8,"headRefName":"chore/ratchet-tighten-b","headRefOid":"` + strings.Repeat("b", 40) + `"}]`, "ambiguous_pr"},
		"a draft":        {`[{"number":7,"headRefName":"chore/ratchet-tighten-a","headRefOid":"` + strings.Repeat("a", 40) + `","isDraft":true}]`, "draft"},
		"no head":        {`[{"number":7,"headRefName":"chore/ratchet-tighten-a"}]`, "head_unknown"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			(&fakeRecorder{}).install(t)
			o.stage(t, "enforce")
			gh := &fakeGH{list: tc.list}
			gh.install(t)
			t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

			o.tighten(t)

			f := fields(o.onlyDecision(t))
			if _, ok := o.recordFor(7); ok || f["decision"] != "no_record" || f["reason"] != tc.reason {
				t.Errorf("fields=%v, want no_record %s", f, tc.reason)
			}
		})
	}
}

func TestRatchetRecord_AForkPRIsNeverRecorded(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: `[{"number":9,"headRefName":"chore/ratchet-tighten-fork","isCrossRepository":true,"headRefOid":"` + strings.Repeat("f", 40) + `"}]`}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	o.tighten(t)

	if _, ok := o.recordFor(9); ok {
		t.Fatal("a fork PR was recorded")
	}
	if rec, ok := o.recordFor(31); !ok || rec.PR != 31 {
		t.Errorf("the new same-repo PR was not recorded")
	}
}

func TestRatchetRecord_AHeadThatMovedAfterThePushRecordsNothing(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = func(create []string) string {
		return fmt.Sprintf(`[{"number":31,"headRefName":%q,"headRefOid":%q}]`, flagAfter(create, "--head"), strings.Repeat("9", 40))
	}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	o.tighten(t)

	f := fields(o.onlyDecision(t))
	if _, ok := o.recordFor(31); ok || f["reason"] != "head_moved" || f["head_gate"] != "head_moved" {
		t.Errorf("fields=%v, want head_moved and no record", f)
	}
}

func TestRatchetRecord_ARefusedRecordWarnsRejectsAndNeverStopsTheBoundary(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{err: &integration.Refusal{PR: 31, Reason: "head moved"}}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, _, errOut := o.tighten(t)

	e := o.onlyDecision(t)
	if rc != 0 || !strings.Contains(errOut, "WARN") || !strings.Contains(errOut, "head moved") || fields(e)["reason"] != "record_refused" || e.Kind != signalcenter.KindGateRejected {
		t.Errorf("rc=%d stderr=%q signal=%+v", rc, errOut, e)
	}
}

func TestRatchetRecord_AnErrorThatIsNotARefusalIsRecordError(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{err: errors.New("gh down")}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = o.listing(31)
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	o.tighten(t)

	if f := fields(o.onlyDecision(t)); f["reason"] != "record_error" || f["decision"] != "no_record" {
		t.Errorf("fields=%v", f)
	}
}

func TestRatchetRecord_ALiveLeaseWritesNoSignalAndNoRecord(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	runDir := filepath.Join(o.plane.Dir, ".evolve", "runs", "cycle-7")
	writeLiveLease(t, runDir)
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	_, _, errOut := o.tighten(t)

	if !strings.Contains(errOut, "cycle-7") || len(o.decisions(t)) != 0 || len(gh.calls) != 0 {
		t.Errorf("stderr=%q decisions=%d gh calls=%d, want a refusal before any write", errOut, len(o.decisions(t)), len(gh.calls))
	}
}

func TestRatchetRecord_TightListsAndNoPRSignalNone(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 64}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	_, out, _ := o.tighten(t)

	if f := fields(o.onlyDecision(t)); f["decision"] != "none" || f["reason"] != "" || !strings.Contains(out, "tight") {
		t.Errorf("fields=%v stdout=%q", f, out)
	}
}

func TestRatchetRecord_TightListsWithAnOpenPRNameTheStalePR(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 64}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: `[{"number":7,"headRefName":"chore/ratchet-tighten-old","headRefOid":"` + strings.Repeat("c", 40) + `"}]`}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, _, errOut := o.tighten(t)

	f := fields(o.onlyDecision(t))
	if rc != 0 || f["decision"] != "none" || f["reason"] != "stale_open_pr" || f["pr"] != "7" || !strings.Contains(errOut, "WARN") || !strings.Contains(errOut, "#7") {
		t.Errorf("rc=%d stderr=%q fields=%v", rc, errOut, f)
	}
}

func TestRatchetRecord_AnUnchangedPushIsSkippedAndTheRecordKept(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	branch, head, _ := o.firstRun(t, "enforce")

	out, _, gh := o.rerun(t, branch, 31)

	if got := o.bare.Git("rev-parse", branch); got != head {
		t.Fatalf("the branch moved from %s to %s: an unchanged push must be skipped", head, got)
	}
	if !strings.Contains(out, "unchanged, kept #31") || gh.created() != nil {
		t.Errorf("stdout=%q created=%v", out, gh.created())
	}
	rec, ok := o.recordFor(31)
	if !ok || rec.Commits[0].SHA != head {
		t.Errorf("the record no longer binds %s", head)
	}
	decisions := o.decisions(t)
	last := fields(decisions[len(decisions)-1])
	if last["decision"] != "already_recorded" || last["skipped_push"] != "true" {
		t.Errorf("signal fields %v", last)
	}
}

func TestRatchetRecord_AHeadBehindMainWithTheSameListsIsStillSkipped(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	branch, head, _ := o.firstRun(t, "enforce")
	o.plane.Write("docs/other.md", "other\n")
	o.plane.Git("add", "-A")
	o.plane.Git("commit", "-q", "-m", "an unrelated change")
	o.plane.Git("push", "-q", "origin", "main")

	out, _, _ := o.rerun(t, branch, 31)

	if got := o.bare.Git("rev-parse", branch); got != head || !strings.Contains(out, "unchanged, kept #31") {
		t.Errorf("branch %s want %s, stdout %q: main moved but the lists did not", got, head, out)
	}
}

func TestRatchetRecord_ChangedListsPushANewTipAndRecordTheNewHead(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	branch, head, _ := o.firstRun(t, "enforce")
	o.plane.Write("go/internal/fixture/big.go", strings.Replace(o.plane.Git("show", "main:go/internal/fixture/big.go"), "total += 5\n", "total += 5\n\ttotal += 6\n", 1)+"\n")
	o.plane.Git("add", "-A")
	o.plane.Git("commit", "-q", "-m", "grow big")
	o.plane.Git("push", "-q", "origin", "main")
	o.stage(t, "enforce")
	gh := &fakeGH{}
	gh.listFn = func() string {
		return fmt.Sprintf(`[{"number":31,"headRefName":%q,"headRefOid":%q}]`, branch, o.bare.Git("rev-parse", branch))
	}
	gh.install(t)

	rc, _, errOut := o.tighten(t)

	newHead := o.bare.Git("rev-parse", branch)
	rec, ok := o.recordFor(31)
	if rc != 0 || newHead == head || !ok || rec.Commits[0].SHA != newHead {
		t.Errorf("rc=%d stderr=%q: head %s -> %s, record binds %v", rc, errOut, head, newHead, rec.Commits)
	}
}

func TestRatchetMessages_DeclareNoInboxItem(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{"message": ratchetTightenMessage, "body": ratchetTightenBody} {
		if !closesinbox.Declares([]byte(text)) || len(closesinbox.IDs([]byte(text))) != 0 {
			t.Errorf("the %s must carry Closes-Inbox: none and no id:\n%s", name, text)
		}
	}
}

func TestRatchetListsBinder_BindsOnlyThePathsOfTheTwoLists(t *testing.T) {
	t.Parallel()
	ok := integration.Evidence{Commits: []string{"c"}, Paths: []string{ratchetListPath, rawGitBaselinePath}, DiffSHA: "d"}
	if b, err := ratchetListsBinder(ok); err != nil || b.Class != integration.RatchetListsClass {
		t.Errorf("binder = %+v, %v", b, err)
	}
	for _, paths := range [][]string{{"go/internal/core/x.go"}, {ratchetListPath, "go/x.go"}, nil} {
		ev := ok
		ev.Paths = paths
		if _, err := ratchetListsBinder(ev); err == nil {
			t.Errorf("paths %v bound", paths)
		}
	}
	if _, ok := reviewClasses[integration.RatchetListsClass]; ok {
		t.Error("the ratchet-lists class is in reviewClasses: an operator could attest a hand edit")
	}
}

func TestRatchetScanRegistry_ReturnsErrorsAndNamesTheRefusalOfEachScan(t *testing.T) {
	t.Parallel()
	if len(ratchetScanTable) != 2 || ratchetScanTable[0].key != "size" || ratchetScanTable[0].reason != "scan_size_red" || ratchetScanTable[1].key != "rawgit" || ratchetScanTable[1].reason != "scan_rawgit_red" {
		t.Fatalf("scan table = %+v", ratchetScanTable)
	}
	if err := runRatchetScans(t.TempDir()); err == nil {
		t.Error("a module with no lists scanned clean")
	}
}

func TestRatchetScansAdapter_PrintsAndReturnsABool(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if ratchetScans["size"](t.TempDir(), &stderr) || stderr.Len() == 0 {
		t.Errorf("the stderr adapter must print the scan error and return false: %q", stderr.String())
	}
}

func TestRatchetHeadMatches_FailsClosedOnEveryShapeThatIsNotTheGeneratedCommit(t *testing.T) {
	t.Parallel()
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	wt := o.plane.Dir
	size := sizeratchet.OffendersRelPath
	raw := rawgitratchet.BaselineRelPath
	lists := []tightenedList{
		{rel: size, encoded: []byte("{\n  \"internal/fixture.Big\": 64\n}\n")},
		{rel: raw, encoded: []byte("{\n  \"internal/fixture/raw_test.go\": 1\n}\n")},
	}
	commit := func(name string, files map[string]string) string {
		o.plane.Git("checkout", "-q", "-b", name, "origin/main")
		for rel, body := range files {
			o.plane.Write(rel, body)
		}
		o.plane.Git("add", "-A")
		o.plane.Git("commit", "-q", "-m", name)
		sha := o.plane.Git("rev-parse", "HEAD")
		o.plane.Git("checkout", "-q", "main")
		return sha
	}
	want := map[string]string{"go/" + size: string(lists[0].encoded), "go/" + raw: string(lists[1].encoded)}
	good := commit("good", want)
	extra := commit("extra", map[string]string{"go/" + size: want["go/"+size], "go/" + raw: want["go/"+raw], "go/x.go": "package x\n"})
	stale := commit("stale", map[string]string{"go/" + size: want["go/"+size], "go/" + raw: "{}\n"})
	wrong := commit("wrong", map[string]string{"go/" + size: "{\n  \"internal/fixture.Big\": 99\n}\n", "go/" + raw: want["go/"+raw]})
	o.plane.Git("checkout", "-q", "-b", "two", "origin/main")
	o.plane.Write("go/a.go", "package a\n")
	o.plane.Git("add", "-A")
	o.plane.Git("commit", "-q", "-m", "one")
	o.plane.Write("go/"+size, want["go/"+size])
	o.plane.Write("go/"+raw, want["go/"+raw])
	o.plane.Git("add", "-A")
	o.plane.Git("commit", "-q", "-m", "two")
	two := o.plane.Git("rev-parse", "HEAD")
	o.plane.Git("checkout", "-q", "main")

	for name, tc := range map[string]struct{ head, want string }{
		"the generated commit": {good, ""},
		"an extra file":        {extra, "unexpected_files"},
		"a stale other list":   {stale, "content_differs"},
		"a wrong list":         {wrong, "content_differs"},
		"two commits":          {two, "not_single_commit"},
		"an unknown head":      {strings.Repeat("0", 40), "content_unknown"},
	} {
		if got := ratchetHeadMatches(wt, tc.head, lists); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

func writeLiveLease(t *testing.T, runDir string) {
	t.Helper()
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestRatchetRecord_AGitStepThatFailsRecordsNothing(t *testing.T) {
	cases := map[string]struct {
		verb   string
		reason string
	}{
		"rev-parse of the pushed head": {"rev-parse", "propose_failed"},
		"show of a list at the head":   {"show", "content_unknown"},
		"diff-tree of the head":        {"diff-tree", "content_unknown"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			(&fakeRecorder{}).install(t)
			o.stage(t, "enforce")
			gh := &fakeGH{list: "[]"}
			gh.afterCreate = o.listing(31)
			pushed := false
			gh.failGit = func(args []string) bool {
				if len(args) > 0 && args[0] == "push" {
					pushed = true
				}
				if tc.verb == "rev-parse" {
					return pushed && len(args) > 1 && args[0] == "rev-parse" && args[1] == "HEAD"
				}
				return pushed && len(args) > 0 && args[0] == tc.verb
			}
			gh.install(t)
			t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

			o.tighten(t)

			f := fields(o.onlyDecision(t))
			if _, ok := o.recordFor(31); ok || f["decision"] != "no_record" || f["reason"] != tc.reason {
				t.Errorf("fields=%v, want no_record %s", f, tc.reason)
			}
		})
	}
}

func TestRatchetRecord_AListThatCannotBeReadWarnsAndRecordsNothing(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	prev := ratchetRead
	t.Cleanup(func() { ratchetRead = prev })
	ratchetRead = func(string) ([]byte, error) { return nil, errors.New("disk gone") }
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, _, errOut := o.tighten(t)

	f := fields(o.onlyDecision(t))
	if rc != 0 || !strings.Contains(errOut, "disk gone") || f["reason"] != "propose_failed" {
		t.Errorf("rc=%d stderr=%q fields=%v", rc, errOut, f)
	}
}

func TestRatchetRecord_AnotherPushAfterOursRecordsNothing(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	o.stage(t, "enforce")
	gh := &fakeGH{list: "[]"}
	gh.afterCreate = func(create []string) string {
		branch := flagAfter(create, "--head")
		o.plane.Git("fetch", "-q", "origin", "refs/heads/"+branch+":refs/heads/other")
		o.plane.Git("checkout", "-q", "other")
		o.plane.Git("commit", "-q", "--allow-empty", "-m", "someone else pushed")
		o.plane.Git("push", "-q", "origin", "HEAD:refs/heads/"+branch)
		o.plane.Git("checkout", "-q", "main")
		return o.listing(31)(create)
	}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	o.tighten(t)

	f := fields(o.onlyDecision(t))
	if _, ok := o.recordFor(31); ok || f["reason"] != "head_moved" || f["head_gate"] != "head_moved" {
		t.Errorf("fields=%v, want head_moved and no record", f)
	}
}

func TestRatchetRecord_TwoOpenCandidatesNeverKeepAnUnchangedTip(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	(&fakeRecorder{}).install(t)
	branch, head, _ := o.firstRun(t, "enforce")
	gh := &fakeGH{list: fmt.Sprintf(`[{"number":31,"headRefName":%q,"headRefOid":%q},{"number":32,"headRefName":"chore/ratchet-tighten-other","headRefOid":%q}]`, branch, head, strings.Repeat("e", 40))}
	gh.install(t)

	rc, out, _ := o.tighten(t)

	f := fields(o.decisionsLast(t))
	if rc != 0 || !strings.Contains(out, "updated pull request #31") || strings.Contains(out, "unchanged") || f["reason"] != "ambiguous_pr" || f["skipped_push"] != "false" {
		t.Errorf("rc=%d stdout=%q fields=%v: with two candidates the step must push again and record nothing", rc, out, f)
	}
}

func (o ratchetOrigin) decisionsLast(t *testing.T) signalcenter.Event {
	t.Helper()
	got := o.decisions(t)
	if len(got) == 0 {
		t.Fatal("no decision signal")
	}
	return got[len(got)-1]
}
