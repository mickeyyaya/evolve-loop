//go:build integration

package ship

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const testWave = "wave-0147"

type recordingRunner struct {
	mu    sync.Mutex
	ops   []string
	fail  func(name string, args []string) bool
	pushs int
}

func (r *recordingRunner) run() sysexec.RunFunc {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		r.mu.Lock()
		r.ops = append(r.ops, name+" "+strings.Join(args, " "))
		if name == "git" && len(args) > 0 && args[0] == "push" {
			r.pushs++
		}
		failing := r.fail != nil && r.fail(name, args)
		r.mu.Unlock()
		if failing {
			_, _ = io.WriteString(stderr, "injected failure\n")
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

func (r *recordingRunner) writeOps() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	readOnly := regexp.MustCompile(`^git( -C \S+)? (rev-list|diff-tree|patch-id|log)\b`)
	var out []string
	for _, op := range r.ops {
		if !readOnly.MatchString(op) {
			out = append(out, op)
		}
	}
	return out
}

func laneOptions(l shiptest.Lane, runner sysexec.RunFunc, in *Integration) Options {
	return Options{
		Class: ClassCycle, CommitMessage: "evolve-cycle 1830", CycleID: shiptest.LandingCycle, RunID: shiptest.LandingRunID, AuditRound: 1,
		ActiveWorktree: l.WT, WorktreeBaseSHA: l.Base, WorkspacePath: l.WS, ProjectRoot: l.Repo, PluginRoot: l.Repo,
		ShipBinaryPath:                  filepath.Join(l.Repo, "ship-binary-fixture"),
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion, BuildExplanation: l.View,
		RequireBuildExplanationHandoff: true, Runner: runner, Sleep: func(time.Duration) {},
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard, Integration: in,
		NowFn: func() Now { return Now{Unix: 1793000000, RFC3339: "2026-11-03T07:33:20Z"} },
	}
}

func fleetIntegration(stage string) *Integration {
	return &Integration{Stage: stage, Wave: testWave, Risk: "low", InboxRank: 7, Scope: []string{"fixture.txt"}, Deps: []string{"cycle-1829"}}
}

func shipLane(t *testing.T, in *Integration, fail func(string, []string) bool) (shiptest.Lane, *recordingRunner, RunResult, error, []signalcenter.Event) {
	t.Helper()
	l := shiptest.LaneReadyToLand(t)
	rr := &recordingRunner{fail: fail}
	center := signalcenter.New()
	var mu sync.Mutex
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})
	opts := laneOptions(l, rr.run(), in)
	opts.Signals = center
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, opts)
	mu.Lock()
	defer mu.Unlock()
	return l, rr, res, err, append([]signalcenter.Event(nil), events...)
}

func recordOf(t *testing.T, l shiptest.Lane) integration.DeliverableRecord {
	t.Helper()
	dir := filepath.Join(l.Repo, ".evolve", "waves", "integration-"+testWave)
	rec, err := integration.ReadRecord(integration.RecordPath(dir, shiptest.LandingCycle))
	if err != nil {
		t.Fatalf("no deliverable record: %v", err)
	}
	return rec
}

func TestDeliver_AFleetShipWritesOneRecordWithEveryD2Field(t *testing.T) {
	t.Parallel()
	l, _, res, err, events := shipLane(t, fleetIntegration(policy.FleetIntegrationShadow), nil)
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("ship: exit=%v err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	rec := recordOf(t, l)
	if rec.Schema != integration.RecordSchema || rec.Wave != testWave || rec.Cycle != shiptest.LandingCycle {
		t.Errorf("identity = %+v", rec)
	}
	if rec.Trailer != "Evolve-Cycle: 1830" || rec.Risk != "low" || rec.InboxRank != 7 {
		t.Errorf("trailer/risk/rank = %q %q %d", rec.Trailer, rec.Risk, rec.InboxRank)
	}
	if strings.Join(rec.Scope, ",") != "fixture.txt" || strings.Join(rec.Deps, ",") != "cycle-1829" {
		t.Errorf("scope=%v deps=%v", rec.Scope, rec.Deps)
	}
	if rec.Audit.TreeSHA != l.Audited || rec.Audit.ArtifactSHA == "" {
		t.Errorf("audit binding = %+v, want tree %s and an artifact sha", rec.Audit, l.Audited)
	}
	if want := time.Unix(1793000000, 0).UTC(); !rec.DeliveredAt.Equal(want) {
		t.Errorf("delivered_at = %v, want %v", rec.DeliveredAt, want)
	}
	if len(rec.Commits) != 1 {
		t.Fatalf("commits = %+v, want 1", rec.Commits)
	}
	c := rec.Commits[0]
	if c.BaseSHA != l.Base || c.SHA != l.Tip(t) {
		t.Errorf("commit = %+v, want base %s tip %s", c, l.Base, l.Tip(t))
	}
	if want := independentPatchID(t, l.Repo, c.SHA); c.PatchID != want {
		t.Errorf("patch_id = %q, want %q", c.PatchID, want)
	}
	if len(events) == 0 {
		t.Fatal("no events")
	}
	var delivered []signalcenter.Event
	for _, e := range events {
		if e.Kind == signalcenter.KindShipDelivered && e.Code == codeIntegrationDelivered {
			delivered = append(delivered, e)
		}
	}
	if len(delivered) != 1 {
		t.Fatalf("ship.delivered events = %d, want 1", len(delivered))
	}
	wantPath := integration.RecordPath(integration.WaveDir(l.Repo, testWave), shiptest.LandingCycle)
	f := delivered[0].Fields
	if f["wave"] != testWave || f["cycle"] != "1830" || f["commits"] != "1" || f["record"] != wantPath {
		t.Errorf("ship.delivered fields = %v, want wave %s cycle 1830 commits 1 record %s", f, testWave, wantPath)
	}
}

func independentPatchID(t *testing.T, repo, sha string) string {
	t.Helper()
	diff := shiptest.RunGitOut(t, repo, "diff-tree", "-p", "--root", sha)
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(out))[0]
}

func normalizeOps(ops []string, l shiptest.Lane) []string {
	out := make([]string, len(ops))
	for i, op := range ops {
		out[i] = strings.NewReplacer(l.Repo, "<repo>", l.WT, "<wt>", l.Base, "<base>", l.Bare, "<bare>").Replace(op)
	}
	return out
}

func TestDeliver_ShadowLandingIsIdenticalToTheLandingWithoutIntegration(t *testing.T) {
	t.Parallel()
	lPlain, rPlain, resPlain, errPlain, _ := shipLane(t, nil, nil)
	lShadow, rShadow, resShadow, errShadow, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationShadow), nil)
	if errPlain != nil || errShadow != nil {
		t.Fatalf("plain=%v shadow=%v", errPlain, errShadow)
	}
	if resPlain.ExitCode != resShadow.ExitCode {
		t.Errorf("exit plain=%v shadow=%v", resPlain.ExitCode, resShadow.ExitCode)
	}
	plainOps, shadowOps := normalizeOps(rPlain.writeOps(), lPlain), normalizeOps(rShadow.writeOps(), lShadow)
	shaRe := regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	for i := range plainOps {
		plainOps[i] = shaRe.ReplaceAllString(plainOps[i], "<sha>")
	}
	for i := range shadowOps {
		shadowOps[i] = shaRe.ReplaceAllString(shadowOps[i], "<sha>")
	}
	if strings.Join(plainOps, "\n") != strings.Join(shadowOps, "\n") {
		t.Errorf("git operations differ.\nplain:\n%s\nshadow:\n%s", strings.Join(plainOps, "\n"), strings.Join(shadowOps, "\n"))
	}
	if rPlain.pushs != rShadow.pushs || rShadow.pushs == 0 {
		t.Errorf("pushes plain=%d shadow=%d, want equal and nonzero", rPlain.pushs, rShadow.pushs)
	}
	if got, want := normalizeLogs(resShadow.Logs, lShadow), normalizeLogs(resPlain.Logs, lPlain); got != want {
		t.Errorf("logs differ.\nplain:\n%s\nshadow:\n%s", want, got)
	}
	if a, b := shiptest.RunGitOut(t, lPlain.Repo, "show", "main:fixture.txt"), shiptest.RunGitOut(t, lShadow.Repo, "show", "main:fixture.txt"); a != b {
		t.Errorf("landed payload differs: %q vs %q", a, b)
	}
	if _, err := os.Stat(filepath.Join(lPlain.Repo, ".evolve", "waves")); !os.IsNotExist(err) {
		t.Errorf("the plain lane wrote a wave directory: %v", err)
	}
	if rec := recordOf(t, lShadow); rec.Cycle != shiptest.LandingCycle {
		t.Errorf("shadow record = %+v", rec)
	}
}

func normalizeLogs(logs []string, l shiptest.Lane) string {
	shaRe := regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	ageRe := regexp.MustCompile(`age -?\d+s`)
	out := make([]string, len(logs))
	for i, line := range logs {
		line = strings.NewReplacer(l.Repo, "<repo>", l.WT, "<wt>", l.Bare, "<bare>").Replace(line)
		out[i] = ageRe.ReplaceAllString(shaRe.ReplaceAllString(line, "<sha>"), "age <n>s")
	}
	return strings.Join(out, "\n")
}

func TestDeliver_ShadowStillLandsWhenTheRecordCannotBeWritten(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	shiptest.MustWrite(t, filepath.Join(l.Repo, ".evolve", "waves"), "a file where the directory belongs\n")
	rr := &recordingRunner{}
	res, err := Run(context.Background(), laneOptions(l, rr.run(), fleetIntegration(policy.FleetIntegrationShadow)))
	if err != nil || res.ExitCode != ExitOK || rr.pushs == 0 {
		t.Fatalf("shadow must land: exit=%v err=%v pushes=%d", res.ExitCode, err, rr.pushs)
	}
	if !shiptest.ContainsLog(res.Logs, "deliverable record not written") {
		t.Errorf("no WARN for the unwritten record: %v", res.Logs)
	}
}

func TestDeliver_EnforceStopsBeforeMainAfterWritingTheRecord(t *testing.T) {
	t.Parallel()
	l, rr, res, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), nil)
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("enforce ship: exit=%v err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if rr.pushs != 0 {
		t.Errorf("pushes = %d, want 0", rr.pushs)
	}
	if main, origin := shiptest.HeadSHA(t, l.Repo), shiptest.RemoteHeadSHA(t, l.Repo); main != l.Base || origin != l.Base {
		t.Errorf("main=%s origin=%s, want both at base %s", main, origin, l.Base)
	}
	rec := recordOf(t, l)
	if len(rec.Commits) != 1 || rec.Commits[0].SHA != l.Tip(t) || res.CommitSHA != l.Tip(t) {
		t.Errorf("record commits=%+v result=%s tip=%s", rec.Commits, res.CommitSHA, l.Tip(t))
	}
	if _, err := os.Stat(l.IntentFile()); !os.IsNotExist(err) {
		t.Errorf("a landing intent exists at enforce: %v", err)
	}
	if !shiptest.ContainsLog(res.Logs, "DELIVERED") {
		t.Errorf("no DELIVERED log: %v", res.Logs)
	}
}

func TestDeliver_EnforceFailsLoudlyWhenTheRecordCannotBeWritten(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	shiptest.MustWrite(t, filepath.Join(l.Repo, ".evolve", "waves"), "a file where the directory belongs\n")
	rr := &recordingRunner{}
	res, err := Run(context.Background(), laneOptions(l, rr.run(), fleetIntegration(policy.FleetIntegrationEnforce)))
	if err == nil || res.ExitCode == ExitOK {
		t.Fatalf("enforce with an unwritable record must fail: exit=%v err=%v", res.ExitCode, err)
	}
	if rr.pushs != 0 || !strings.Contains(err.Error(), "deliverable record not written") {
		t.Errorf("pushes=%d err=%v", rr.pushs, err)
	}
}

func TestDeliver_EnforceFailsWhenTheLaneRefCannotMove(t *testing.T) {
	t.Parallel()
	fail := func(name string, args []string) bool {
		return name == "git" && strings.Contains(strings.Join(args, " "), "update-ref -m evolve ship: the landing commit HEAD")
	}
	_, rr, res, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), fail)
	if err == nil || res.ExitCode == ExitOK || rr.pushs != 0 {
		t.Fatalf("exit=%v err=%v pushes=%d, want a failed ship with no push", res.ExitCode, err, rr.pushs)
	}
}

func TestDeliver_GitFaultsAreReportedNotSwallowed(t *testing.T) {
	t.Parallel()
	cases := map[string]func(string, []string) bool{
		"rev-list": func(n string, a []string) bool {
			return n == "git" && contains(a, "rev-list") && contains(a, "--reverse")
		},
		"diff-tree": func(n string, a []string) bool {
			return n == "git" && contains(a, "diff-tree") && contains(a, "--root")
		},
		"patch-id": func(n string, a []string) bool { return n == "git" && contains(a, "patch-id") },
	}
	for name, fail := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, rr, res, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), fail)
			if err == nil || res.ExitCode == ExitOK || rr.pushs != 0 {
				t.Fatalf("exit=%v err=%v pushes=%d, want a failed ship", res.ExitCode, err, rr.pushs)
			}
		})
	}
}

func contains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

func TestDeliver_AnEmptyCommitListFailsTheRecord(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	opts := laneOptions(l, (&recordingRunner{}).run(), fleetIntegration(policy.FleetIntegrationEnforce))
	opts.WorktreeBaseSHA = l.Tip(t)
	s := &worktreeShip{ctx: context.Background(), opts: &opts, worktree: l.WT}
	if _, err := s.buildDeliverable(l.Tip(t)); err != nil {
		t.Fatalf("an empty range builds an empty list: %v", err)
	}
	if _, _, err := s.writeDeliverable(l.Tip(t)); err == nil {
		t.Fatal("a record without commits must not be written")
	}
}

func TestDeliver_TheDeliveredAtClockDefaultsToTheSystemClock(t *testing.T) {
	t.Parallel()
	s := &worktreeShip{opts: &Options{}}
	if at := s.deliveredAt(); at.IsZero() || time.Since(at) > time.Minute {
		t.Errorf("deliveredAt = %v, want about now", at)
	}
}

func landedCommitFor(t *testing.T, repo string, rec integration.DeliverableRecord) string {
	t.Helper()
	shiptest.RunGit(t, repo, "cherry-pick", "--no-commit", rec.Commits[0].SHA)
	shiptest.RunGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "compose: the lane commit\n\n"+rec.Trailer)
	return shiptest.HeadSHA(t, repo)
}

func TestLanded_ACycleIsFoundByTrailerAndPatchIDAfterACompose(t *testing.T) {
	t.Parallel()
	l, _, _, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := recordOf(t, l)
	shiptest.RunGit(t, l.Repo, "checkout", "-q", "main")
	rewritten := landedCommitFor(t, l.Repo, rec)
	shiptest.MustWrite(t, filepath.Join(l.Repo, "later.txt"), "later\n")
	shiptest.RunGit(t, l.Repo, "add", "later.txt")
	shiptest.RunGit(t, l.Repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "a later commit")
	shiptest.RunGit(t, l.Repo, "push", "-q", "origin", "main")
	if rewritten == rec.Commits[0].SHA {
		t.Fatal("precondition: the compose must give the commit a new SHA")
	}
	opts := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationEnforce))
	if isAncestor(context.Background(), &opts, rec.Commits[0].SHA, "origin/main") {
		t.Fatal("precondition: the original SHA must not be an ancestor of main")
	}
	if !isLanded(context.Background(), &opts, rec.Commits[0].SHA) {
		t.Error("isLanded = false, want true by trailer and patch-id")
	}
}

func TestLanded_ATrailerWithADifferentPatchIsNotTheCycle(t *testing.T) {
	t.Parallel()
	l, _, _, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := recordOf(t, l)
	shiptest.RunGit(t, l.Repo, "checkout", "-q", "main")
	shiptest.MustWrite(t, filepath.Join(l.Repo, "other.txt"), "other\n")
	shiptest.RunGit(t, l.Repo, "add", "other.txt")
	shiptest.RunGit(t, l.Repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "other work\n\n"+rec.Trailer)
	shiptest.RunGit(t, l.Repo, "push", "-q", "origin", "main")
	opts := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationEnforce))
	if isLanded(context.Background(), &opts, rec.Commits[0].SHA) {
		t.Error("a trailer alone must not match the cycle")
	}
}

func TestLanded_NoCycleIdentityOrAnUnknownBaseFindsNothing(t *testing.T) {
	t.Parallel()
	l, _, _, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), nil)
	if err != nil {
		t.Fatal(err)
	}
	opts := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationEnforce))
	opts.CycleID = 0
	opts.WorkspacePath = ""
	if landedByIdentity(context.Background(), &opts, l.Base, "HEAD") {
		t.Error("without a cycle id nothing can match")
	}
}

func TestLanded_AMatchingTrailerWhosePatchCannotBeDerivedIsSkipped(t *testing.T) {
	t.Parallel()
	l, _, _, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := recordOf(t, l)
	shiptest.RunGit(t, l.Repo, "checkout", "-q", "main")
	landedCommitFor(t, l.Repo, rec)
	broken := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && contains(args, "patch-id") {
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	opts := laneOptions(l, broken, fleetIntegration(policy.FleetIntegrationEnforce))
	if landedByIdentity(context.Background(), &opts, rec.Commits[0].SHA, "HEAD") {
		t.Error("a trailer commit whose patch cannot be derived must not match")
	}
	opts.Runner = (&recordingRunner{}).run()
	if !landedByIdentity(context.Background(), &opts, rec.Commits[0].SHA, "HEAD") {
		t.Error("with a working git the same history matches")
	}
}

func TestLanded_ARecordThatDoesNotHoldTheCommitDoesNotVouchForIt(t *testing.T) {
	t.Parallel()
	l, _, _, err, _ := shipLane(t, fleetIntegration(policy.FleetIntegrationEnforce), nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := recordOf(t, l)
	shiptest.RunGit(t, l.Repo, "checkout", "-q", "main")
	landedCommitFor(t, l.Repo, rec)
	opts := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationEnforce))
	if !landedByIdentity(context.Background(), &opts, rec.Commits[0].SHA, "HEAD") {
		t.Fatal("precondition: the recorded commit is found by its trailer and patch-id")
	}
	if landedByIdentity(context.Background(), &opts, l.Base, "HEAD") {
		t.Error("the record of the cycle vouched for a commit that it does not hold")
	}
}

func TestLanded_WithoutAnIntegrationOrARecordTheAncestryRuleStands(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	plain := laneOptions(l, nil, nil)
	if landedByIdentity(context.Background(), &plain, l.Base, "HEAD") {
		t.Error("no integration, so no identity match")
	}
	fleet := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationShadow))
	if landedByIdentity(context.Background(), &fleet, l.Base, "HEAD") {
		t.Error("no record on disk, so no identity match")
	}
	if isLanded(context.Background(), &fleet, l.Base) != isAncestor(context.Background(), &fleet, l.Base, "origin/main") {
		t.Error("with no record, isLanded must equal the ancestry rule")
	}
}

func failGitSub(sub string) sysexec.RunFunc {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && contains(args, sub) {
			return 2, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

func TestDeliver_ADiffTreeFaultFailsThePatchID(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	opts := laneOptions(l, failGitSub("diff-tree"), nil)
	if _, err := (integration.Repo{Dir: l.WT, Git: shipGit(&opts)}).PatchID(context.Background(), l.Base); err == nil {
		t.Fatal("a failed diff-tree must be an error")
	}
}

func TestDeliver_TheRecordListsTheCommitsOldestFirst(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	commitFile := func(name string) string {
		shiptest.MustWrite(t, filepath.Join(l.WT, name), name+"\n")
		shiptest.RunGit(t, l.WT, "add", name)
		shiptest.RunGit(t, l.WT, "-c", "commit.gpgsign=false", "commit", "-q", "-m", name)
		return shiptest.HeadSHA(t, l.WT)
	}
	first, second := commitFile("first.txt"), commitFile("second.txt")
	opts := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationEnforce))
	s := &worktreeShip{ctx: context.Background(), opts: &opts, worktree: l.WT}
	rec, err := s.buildDeliverable(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Commits) != 2 || rec.Commits[0].SHA != first || rec.Commits[1].SHA != second {
		t.Fatalf("commits = %+v, want %s then %s", rec.Commits, first, second)
	}
}

func TestDeliver_APatchIDFaultFailsTheBuildOfTheRecord(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	shiptest.MustWrite(t, filepath.Join(l.WT, "extra.txt"), "extra\n")
	shiptest.RunGit(t, l.WT, "add", "extra.txt")
	shiptest.RunGit(t, l.WT, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "extra")
	opts := laneOptions(l, failGitSub("patch-id"), fleetIntegration(policy.FleetIntegrationEnforce))
	s := &worktreeShip{ctx: context.Background(), opts: &opts, worktree: l.WT}
	if _, err := s.buildDeliverable(shiptest.HeadSHA(t, l.WT)); err == nil {
		t.Fatal("a failed patch-id must fail the record")
	}
}

func TestLanded_AnUnreadableCycleStateFindsNothing(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	opts := laneOptions(l, nil, fleetIntegration(policy.FleetIntegrationShadow))
	opts.CycleID = 0
	if err := os.Remove(filepath.Join(l.Repo, ".evolve", "cycle-state.json")); err != nil {
		t.Fatal(err)
	}
	if landedByIdentity(context.Background(), &opts, l.Base, "HEAD") {
		t.Error("without a readable cycle state nothing can match")
	}
}

func TestDeliver_ARevListFaultFailsTheBuildOfTheRecord(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	opts := laneOptions(l, failGitSub("rev-list"), fleetIntegration(policy.FleetIntegrationEnforce))
	s := &worktreeShip{ctx: context.Background(), opts: &opts, worktree: l.WT}
	if _, err := s.buildDeliverable(l.Tip(t)); err == nil {
		t.Fatal("a failed rev-list must fail the record")
	}
}

func TestLand_AnIntentThatCannotBePreparedIsAbandonedBeforeMain(t *testing.T) {
	t.Parallel()
	l := shiptest.LaneReadyToLand(t)
	rr := &recordingRunner{}
	opts := laneOptions(l, rr.run(), nil)
	if err := os.MkdirAll(shipJournalPath(l.Repo), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, opts)
	if err == nil || !strings.Contains(err.Error(), "cannot record the landing intent") {
		t.Fatalf("Run = exit %v err %v logs %v, want the landing intent refusal", res.ExitCode, err, res.Logs)
	}
	if rr.pushs != 0 {
		t.Errorf("pushes = %d, want none after an abandoned landing", rr.pushs)
	}
}
