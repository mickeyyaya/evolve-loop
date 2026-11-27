//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func shipEnforceLane(t *testing.T, l shiptest.Lane, fail func(string, []string) bool) (*recordingRunner, RunResult, error) {
	t.Helper()
	rr := &recordingRunner{fail: fail}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, laneOptions(l, rr.run(), fleetIntegration(policy.FleetIntegrationEnforce)))
	return rr, res, err
}

func waveDirOf(l shiptest.Lane) string {
	return filepath.Join(l.Repo, ".evolve", "waves", "integration-"+testWave)
}

func recordBytes(t *testing.T, l shiptest.Lane) []byte {
	t.Helper()
	raw, err := os.ReadFile(integration.RecordPath(waveDirOf(l), shiptest.LandingCycle))
	if err != nil {
		t.Fatalf("no deliverable record: %v", err)
	}
	return raw
}

func requireWaveDirHoldsOnlyTheRecord(t *testing.T, l shiptest.Lane) {
	t.Helper()
	entries, err := os.ReadDir(waveDirOf(l))
	if err != nil || len(entries) != 1 {
		t.Fatalf("wave directory holds %d entries (err=%v), want only the record and no *.tmp", len(entries), err)
	}
}

func deliveredBefore(t *testing.T, l shiptest.Lane, sha, patch string) {
	t.Helper()
	earlier := integration.DeliverableRecord{
		Wave: testWave, Cycle: shiptest.LandingCycle, Trailer: integration.CycleTrailer(shiptest.LandingCycle),
		Commits: []integration.CommitIdentity{{SHA: sha, BaseSHA: l.Base, PatchID: patch}},
	}
	if _, err := integration.WriteRecordWith(waveDirOf(l), earlier, integration.WriteOnce); err != nil {
		t.Fatal(err)
	}
}

type laneWork map[string][]byte

func laneWorkOf(t *testing.T, l shiptest.Lane) laneWork {
	t.Helper()
	work := laneWork{}
	for _, entry := range strings.Split(shiptest.RunGitOut(t, l.WT, "status", "--porcelain", "-z", "-uall"), "\x00") {
		if len(entry) < 4 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(l.WT, entry[3:]))
		if err != nil {
			t.Fatal(err)
		}
		work[entry[3:]] = raw
	}
	return work
}

func (w laneWork) rewind(t *testing.T, l shiptest.Lane) {
	t.Helper()
	shiptest.RunGit(t, l.WT, "reset", "-q", "--hard", l.Base)
	shiptest.RunGit(t, l.WT, "clean", "-fdq")
	for rel, raw := range w {
		shiptest.MustWrite(t, filepath.Join(l.WT, rel), string(raw))
	}
}

func failTheLaneRefMove(name string, args []string) bool {
	return name == "git" && strings.Contains(strings.Join(args, " "), "update-ref -m evolve ship: the landing commit HEAD")
}

func untilTheNextSecond() {
	for started := time.Now().Unix(); time.Now().Unix() == started; {
		runtime.Gosched()
	}
}

func deliveredThenRebuilt(t *testing.T, l shiptest.Lane) {
	t.Helper()
	work := laneWorkOf(t, l)
	if _, res, err := shipEnforceLane(t, l, nil); err != nil || !res.Delivered {
		t.Fatalf("the first ship must deliver: exit=%v delivered=%v err=%v", res.ExitCode, res.Delivered, err)
	}
	requireTheConsumedStampIsARecentTime(t, l)
	work.rewind(t, l)
	untilTheNextSecond()
}

func requireTheConsumedStampIsARecentTime(t *testing.T, l shiptest.Lane) {
	t.Helper()
	consumed := shiptest.RunGitOut(t, l.WT, "ls-tree", "--name-only", recordOf(t, l).Commits[0].SHA, ".evolve/inbox/consumed/")
	item := shiptest.RunGitOut(t, l.WT, "show", recordOf(t, l).Commits[0].SHA+":"+strings.TrimSpace(consumed))
	var doc struct {
		Consumed struct {
			At time.Time `json:"at"`
		} `json:"consumed"`
	}
	if err := json.Unmarshal([]byte(item), &doc); err != nil {
		t.Fatalf("the consumed item of the delivered commit is not JSON: %v\n%s", err, item)
	}
	if age := time.Since(doc.Consumed.At); age < 0 || age > time.Hour {
		t.Errorf("the consumed stamp %v is %v old: it must stay a real time of the cycle, not a constant", doc.Consumed.At, age)
	}
}

func requireRetryReadsAsDelivered(t *testing.T, l shiptest.Lane, message string) {
	t.Helper()
	written := recordBytes(t, l)

	rr := &recordingRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	opts := laneOptions(l, rr.run(), fleetIntegration(policy.FleetIntegrationEnforce))
	opts.CommitMessage = message
	res, err := Run(ctx, opts)

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("the ship after a delivery must not fail: exit=%v err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if !shiptest.ContainsLog(res.Logs, "DELIVERED") {
		t.Errorf("the ship did not report DELIVERED: %v", res.Logs)
	}
	if rr.pushs != 0 || shiptest.HeadSHA(t, l.Repo) != l.Base {
		t.Errorf("the ship reached main: pushes=%d head=%s base=%s", rr.pushs, shiptest.HeadSHA(t, l.Repo), l.Base)
	}
	if string(recordBytes(t, l)) != string(written) {
		t.Error("the ship rewrote the record")
	}
	requireWaveDirHoldsOnlyTheRecord(t, l)
}

func TestDeliverWriteOnce_AShipRetryUnderANewSHAWithTheSamePatchesReadsAsDelivered(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	deliveredThenRebuilt(t, l)
	requireRetryReadsAsDelivered(t, l, "evolve-cycle 1830 rebuilt under a new SHA")
}

func TestDeliverWriteOnce_AResumedShipOfTheSameBuildReadsAsDelivered(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	deliveredThenRebuilt(t, l)
	requireRetryReadsAsDelivered(t, l, "evolve-cycle 1830")
}

func TestDeliverWriteOnce_ARetryAfterTheLaneRefFailedMovesTheRefAndKeepsTheRecord(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	work := laneWorkOf(t, l)
	if _, res, err := shipEnforceLane(t, l, failTheLaneRefMove); err == nil || res.ExitCode == ExitOK {
		t.Fatalf("the first ship must stop at the lane ref: exit=%v err=%v", res.ExitCode, err)
	}
	written := recordBytes(t, l)
	if l.Tip(t) != l.Base {
		t.Fatalf("the lane ref moved although the move failed: %s", l.Tip(t))
	}
	work.rewind(t, l)
	untilTheNextSecond()

	rr := &recordingRunner{}
	res, err := Run(context.Background(), laneOptions(l, rr.run(), fleetIntegration(policy.FleetIntegrationEnforce)))

	if err != nil || res.ExitCode != ExitOK || !res.Delivered {
		t.Fatalf("the retry after a stuck lane ref must deliver: exit=%v err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if l.Tip(t) != res.CommitSHA || l.Tip(t) == l.Base {
		t.Errorf("the no-op delivery did not move the lane ref: tip=%s commit=%s base=%s", l.Tip(t), res.CommitSHA, l.Base)
	}
	if string(recordBytes(t, l)) != string(written) || rr.pushs != 0 {
		t.Errorf("the retry rewrote the record or reached main: pushes=%d", rr.pushs)
	}
	requireWaveDirHoldsOnlyTheRecord(t, l)
}

func TestDeliverWriteOnce_ADeliveryWithNewPatchesIsAPermanentRefusalThatLeavesMainAndTheRecordAlone(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	deliveredBefore(t, l, strings.Repeat("a", 40), "an-earlier-build")
	written := recordBytes(t, l)

	rr, res, err := shipEnforceLane(t, l, nil)

	if err == nil || res.ExitCode == ExitOK {
		t.Fatalf("new patches over a delivered record must refuse: exit=%v err=%v", res.ExitCode, err)
	}
	var se *core.ShipError
	if !errors.As(err, &se) || se.Class == core.ShipClassTransient {
		t.Errorf("refusal = %v, want a ShipError that is not transient (a retry cannot change the patches)", err)
	}
	if rr.pushs != 0 || shiptest.HeadSHA(t, l.Repo) != l.Base || shiptest.RemoteHeadSHA(t, l.Repo) != l.Base {
		t.Errorf("main moved: pushes=%d", rr.pushs)
	}
	if string(recordBytes(t, l)) != string(written) {
		t.Error("the refusal rewrote the record")
	}
	requireWaveDirHoldsOnlyTheRecord(t, l)
}

func TestDeliverWriteOnce_AShadowRetryStillReplacesTheRecord(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	deliveredBefore(t, l, strings.Repeat("a", 40), "an-earlier-build")

	rr := &recordingRunner{}
	res, err := Run(context.Background(), laneOptions(l, rr.run(), fleetIntegration(policy.FleetIntegrationShadow)))

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("shadow ship: exit=%v err=%v", res.ExitCode, err)
	}
	if got := recordOf(t, l); got.Commits[0].PatchID == "an-earlier-build" {
		t.Errorf("shadow kept the older record: %+v", got.Commits)
	}
}
