package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const testSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type recordingLanding struct {
	ejected map[int]string
}

func (l *recordingLanding) NoteEjected(_ context.Context, cycle int, briefPath string) error {
	if l.ejected == nil {
		l.ejected = map[int]string{}
	}
	l.ejected[cycle] = briefPath
	return nil
}

type stubEnforcePorts struct {
	briefPath string
	requeued  []int
	promoted  []int
	synced    int
	syncOut   SyncOutcome
	syncErr   error
}

func (s *stubEnforcePorts) Write(_ context.Context, _ *Ejection) (string, error) {
	return s.briefPath, nil
}

func (s *stubEnforcePorts) Requeue(_ context.Context, rec DeliverableRecord, _ string) error {
	s.requeued = append(s.requeued, rec.Cycle)
	return nil
}

func (s *stubEnforcePorts) Promote(_ context.Context, rec DeliverableRecord) error {
	s.promoted = append(s.promoted, rec.Cycle)
	return nil
}

func (s *stubEnforcePorts) Sync(_ context.Context) (SyncOutcome, error) {
	s.synced++
	return s.syncOut, s.syncErr
}

type stubCI struct{ verdict Verdict }

func (c stubCI) Open(_ context.Context, b Batch) (PR, error) { return PR{Number: 1, Tip: b.Tip}, nil }
func (c stubCI) Watch(context.Context, PR) (Verdict, error)  { return c.verdict, nil }
func (c stubCI) Rerun(context.Context, PR) error             { return nil }

type stubMain struct{ head string }

func (m *stubMain) Head(context.Context) (string, error)          { return m.head, nil }
func (m *stubMain) FastForward(_ context.Context, t string) error { m.head = t; return nil }

func TestStageActors_RegistersEnforceOnlyWhenNonNil(t *testing.T) {
	t.Parallel()
	shadow, enforce := &fakeActor{}, &fakeActor{}
	both := StageActors(shadow, enforce)
	if both[policy.FleetIntegrationShadow] != shadow || both[policy.FleetIntegrationEnforce] != enforce {
		t.Fatalf("StageActors(shadow, enforce) = %v", both)
	}
	only := StageActors(shadow, nil)
	if _, ok := only[policy.FleetIntegrationEnforce]; ok || len(only) != 1 {
		t.Fatalf("StageActors(shadow, nil) = %v", only)
	}
}

func TestEnforceActor_LandingFeedNotifiedOnEjection(t *testing.T) {
	t.Parallel()
	ports := &stubEnforcePorts{briefPath: "/tmp/brief-9.txt"}
	landing := &recordingLanding{}
	var feed LandingFeed = landing
	var plane PlaneSync = ports
	var requeue Requeuer = ports
	var briefs BriefStore = ports
	var promote Promoter = ports

	a := EnforceActor{
		Merger: Merger{
			CI:   stubCI{verdict: Red},
			Main: &stubMain{head: testSHA},
			Compose: func(_ context.Context, recs []DeliverableRecord) (Composition, error) {
				return Composition{Batch: Batch{Base: testSHA, Tip: testSHA}, Kept: recs}, nil
			},
		},
		Plane:   plane,
		Landing: feed,
		Requeue: requeue,
		Briefs:  briefs,
		Promote: promote,
		Now:     func() time.Time { return time.Unix(100, 0).UTC() },
	}
	res, err := a.Flush(context.Background(), PendingBatch{
		Pool: 1, Index: 1, Base: testSHA, Members: []DeliverableRecord{{Cycle: 9}},
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if res.Members[9].Status != MemberEjected || landing.ejected[9] != "/tmp/brief-9.txt" {
		t.Fatalf("ejection result = %+v, landing = %v", res.Members[9], landing.ejected)
	}
	if !slices.Equal(ports.requeued, []int{9}) {
		t.Fatalf("requeued = %v, want [9]", ports.requeued)
	}
}

func TestEnforceActor_WaveEndPendingRaisesFailure(t *testing.T) {
	t.Parallel()
	ports := &stubEnforcePorts{syncOut: SyncCurrent}
	a := EnforceActor{Plane: ports, Now: func() time.Time { return time.Unix(200, 0).UTC() }}
	out, err := a.WaveEnd(context.Background(), WaveRecord{
		Pool:       2,
		Epoch:      1,
		ShadowBase: testSHA,
		Members:    map[int]MemberState{4: {Status: MemberPending}},
	})
	if err == nil || len(out.Failures) == 0 {
		t.Fatalf("WaveEnd with pending member: err=%v failures=%v", err, out.Failures)
	}
	if ports.synced != 1 {
		t.Fatalf("synced = %d, want 1", ports.synced)
	}
}

func TestWriteRecordWith_WriteOnceAndReplaceExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rec := DeliverableRecord{
		Wave: "wave-1", Cycle: 12, Trailer: CycleTrailer(12),
		Commits:     []CommitIdentity{{SHA: "a1", BaseSHA: "b0", PatchID: "p1"}},
		Audit:       AuditBinding{TreeSHA: "t1", ArtifactSHA: "art1"},
		Scope:       []string{"go/internal/a"},
		Risk:        "low",
		DeliveredAt: time.Unix(300, 0).UTC(),
	}
	var writePolicy WritePolicy = WriteOnce
	p, err := WriteRecordWith(dir, rec, writePolicy)
	if err != nil {
		t.Fatalf("WriteRecordWith WriteOnce: %v", err)
	}
	if _, err := WriteRecordWith(dir, rec, writePolicy); !errors.Is(err, ErrRecordExists) {
		t.Fatalf("second WriteOnce = %v, want ErrRecordExists", err)
	}
	rec.Scope = []string{"go/internal/b"}
	writePolicy = ReplaceExisting
	if _, err := WriteRecordWith(dir, rec, writePolicy); err != nil {
		t.Fatalf("ReplaceExisting: %v", err)
	}
	got, err := ReadRecord(p)
	if err != nil || !slices.Equal(got.Scope, []string{"go/internal/b"}) {
		t.Fatalf("ReadRecord = %+v, %v", got.Scope, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
}

func TestEnforceActor_ResumeDropsPoolRefs(t *testing.T) {
	t.Parallel()
	rec := WaveRecord{Pool: 3, Epoch: 1, ShadowBase: testSHA}
	if got, err := (EnforceActor{}).Resume(context.Background(), rec); err != nil || got.Pool != 3 {
		t.Fatalf("Resume without git: got=%+v err=%v", got, err)
	}
	var deleted []string
	repo := Repo{
		Dir: t.TempDir(),
		Git: func(_ context.Context, _, _ string, args ...string) (string, error) {
			switch args[0] {
			case "for-each-ref":
				return "refs/heads/integration/p3-b1\n", nil
			case "update-ref":
				deleted = append(deleted, args[len(args)-1])
				return "", nil
			default:
				return "", nil
			}
		},
	}
	if _, err := (EnforceActor{Repo: repo}).Resume(context.Background(), rec); err != nil {
		t.Fatalf("Resume with git: %v", err)
	}
	if !slices.Equal(deleted, []string{"refs/heads/integration/p3-b1"}) {
		t.Fatalf("deleted = %v, want [refs/heads/integration/p3-b1]", deleted)
	}
}

func TestSyncToOrigin_AllOutcomes(t *testing.T) {
	t.Parallel()
	const otherSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	cases := []struct {
		name     string
		local    string
		remote   string
		ahead    bool
		behind   bool
		mergeErr error
		want     SyncOutcome
	}{
		{name: "current", local: testSHA, remote: testSHA, want: SyncCurrent},
		{name: "ahead", local: testSHA, remote: otherSHA, ahead: true, want: SyncAhead},
		{name: "diverged", local: testSHA, remote: otherSHA, want: SyncDiverged},
		{name: "blocked", local: testSHA, remote: otherSHA, behind: true, mergeErr: errors.New("dirty"), want: SyncBlocked},
		{name: "moved", local: testSHA, remote: otherSHA, behind: true, want: SyncMoved},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := Repo{
				Dir: t.TempDir(),
				Git: func(_ context.Context, _, _ string, args ...string) (string, error) {
					switch args[0] {
					case "rev-parse":
						if args[2] == "refs/heads/main" {
							return tc.local, nil
						}
						return tc.remote, nil
					case "merge-base":
						if args[3] == tc.remote && args[4] == tc.local && tc.ahead {
							return "", nil
						}
						if args[3] == tc.local && args[4] == tc.remote && tc.behind {
							return "", nil
						}
						return "", &GitError{Code: 1}
					case "merge":
						return "", tc.mergeErr
					default:
						return "", nil
					}
				},
			}
			got, err := SyncToOrigin(context.Background(), repo)
			if err != nil {
				t.Fatalf("SyncToOrigin(%s): %v", tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("SyncToOrigin(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestWriteRecordWith_WriteOnceLinkFailure(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	rec := DeliverableRecord{
		Wave: "wave-1", Cycle: 13, Trailer: CycleTrailer(13),
		Commits:     []CommitIdentity{{SHA: "a1", BaseSHA: "b0", PatchID: "p1"}},
		Audit:       AuditBinding{TreeSHA: "t1", ArtifactSHA: "art1"},
		Scope:       []string{"go/internal/a"},
		Risk:        "low",
		DeliveredAt: time.Unix(300, 0).UTC(),
	}
	stageDir := t.TempDir()
	prev := createRecordTemp
	t.Cleanup(func() { createRecordTemp = prev })
	createRecordTemp = func(_, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(stageDir, pattern)
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(dir, 0o555); err != nil {
			_ = f.Close()
			return nil, err
		}
		return f, nil
	}
	if _, err := WriteRecordWith(dir, rec, WriteOnce); err == nil || errors.Is(err, ErrRecordExists) || !strings.Contains(err.Error(), "publish") {
		t.Fatalf("WriteRecordWith link error = %v, want publish error", err)
	}
}

type failingPort struct {
	briefErr   error
	requeueErr error
	landingErr error
	promoteErr error
	syncOut    SyncOutcome
	syncErr    error
}

func (p failingPort) Write(context.Context, *Ejection) (string, error) { return "/tmp/b", p.briefErr }
func (p failingPort) Requeue(context.Context, DeliverableRecord, string) error {
	return p.requeueErr
}
func (p failingPort) NoteEjected(context.Context, int, string) error { return p.landingErr }
func (p failingPort) Promote(context.Context, DeliverableRecord) error {
	return p.promoteErr
}
func (p failingPort) Sync(context.Context) (SyncOutcome, error) { return p.syncOut, p.syncErr }

func TestEnforceActor_ErrorBranches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("flush custom eject and unknown landed and promote error", func(t *testing.T) {
		t.Parallel()
		a := EnforceActor{
			Merger: Merger{
				CI:   stubCI{verdict: Green},
				Main: &stubMain{head: testSHA},
				Compose: func(_ context.Context, _ []DeliverableRecord) (Composition, error) {
					return Composition{
						Batch:   Batch{Base: testSHA, Tip: testSHA},
						Kept:    []DeliverableRecord{{Cycle: 999}, {Cycle: 1}},
						Ejected: []*Ejection{{Cycle: 2, Cause: CauseConflict}},
					}, nil
				},
				Eject: func(context.Context, DeliverableRecord, error) error { return nil },
			},
			Promote: failingPort{promoteErr: errors.New("promote boom")},
		}
		res, err := a.Flush(ctx, PendingBatch{
			Pool: 1, Index: 1, Base: testSHA,
			Members: []DeliverableRecord{{Cycle: 1}, {Cycle: 2}},
		})
		if err == nil || len(res.Failures) != 1 || res.Members[2].Status != MemberEjected {
			t.Fatalf("Flush = %+v, err=%v", res, err)
		}
	})

	t.Run("ejectMember brief requeue landing errors", func(t *testing.T) {
		t.Parallel()
		rec := DeliverableRecord{Cycle: 5}
		cause := errors.New("boom")
		for _, tc := range []struct {
			name string
			port failingPort
			step string
		}{
			{"brief", failingPort{briefErr: cause}, "brief"},
			{"requeue", failingPort{requeueErr: cause}, "requeue"},
			{"landing", failingPort{landingErr: cause}, "landing"},
		} {
			var res FlushResult
			a := EnforceActor{Briefs: tc.port, Requeue: tc.port, Landing: tc.port}
			if err := a.ejectMember(ctx, rec, cause, &res); err == nil || len(res.Failures) != 1 || res.Failures[0].Step != tc.step {
				t.Fatalf("%s: failures=%+v err=%v", tc.name, res.Failures, err)
			}
		}
	})

	t.Run("waveEnd sync error blocked and batch ref cleanup", func(t *testing.T) {
		t.Parallel()
		var deleted []string
		repo := Repo{
			Dir: t.TempDir(),
			Git: func(_ context.Context, _, _ string, args ...string) (string, error) {
				if args[0] == "update-ref" {
					deleted = append(deleted, args[len(args)-1])
				}
				return "", nil
			},
		}
		aErr := EnforceActor{Repo: repo, Plane: failingPort{syncErr: errors.New("sync boom")}}
		out, err := aErr.WaveEnd(ctx, WaveRecord{
			Pool: 1, Epoch: 1, ShadowBase: testSHA,
			Batches: []IntegrationBatch{{Branch: "integration/p1-b1"}},
		})
		if err == nil || len(out.Failures) != 1 || !slices.Equal(deleted, []string{"refs/heads/integration/p1-b1"}) {
			t.Fatalf("WaveEnd syncErr: out=%+v err=%v deleted=%v", out, err, deleted)
		}

		aBlocked := EnforceActor{Plane: failingPort{syncOut: SyncBlocked}}
		out, err = aBlocked.WaveEnd(ctx, WaveRecord{Pool: 1, Epoch: 1, ShadowBase: testSHA})
		if err != nil || len(out.Failures) != 1 || out.Failures[0].Step != "plane sync" {
			t.Fatalf("WaveEnd SyncBlocked: out=%+v err=%v", out, err)
		}
	})
}

func TestSyncToOrigin_ErrorBranches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const otherSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	cases := []struct {
		name string
		git  func(context.Context, string, string, ...string) (string, error)
		want string
	}{
		{
			name: "fetch error",
			git: func(_ context.Context, _, _ string, args ...string) (string, error) {
				return "", errors.New("fetch failed")
			},
			want: "fetch origin",
		},
		{
			name: "local rev-parse error",
			git: func(_ context.Context, _, _ string, args ...string) (string, error) {
				if args[0] == "rev-parse" {
					return "", errors.New("bad local")
				}
				return "", nil
			},
			want: "rev-parse refs/heads/main",
		},
		{
			name: "remote rev-parse invalid sha",
			git: func(_ context.Context, _, _ string, args ...string) (string, error) {
				if args[0] == "rev-parse" {
					if args[2] == "refs/heads/main" {
						return testSHA, nil
					}
					return "not-a-sha", nil
				}
				return "", nil
			},
			want: "object id",
		},
		{
			name: "ahead merge-base error",
			git: func(_ context.Context, _, _ string, args ...string) (string, error) {
				switch args[0] {
				case "rev-parse":
					if args[2] == "refs/heads/main" {
						return testSHA, nil
					}
					return otherSHA, nil
				case "merge-base":
					return "", errors.New("merge-base failed")
				}
				return "", nil
			},
			want: "merge-base failed",
		},
		{
			name: "behind merge-base error",
			git: func(_ context.Context, _, _ string, args ...string) (string, error) {
				switch args[0] {
				case "rev-parse":
					if args[2] == "refs/heads/main" {
						return testSHA, nil
					}
					return otherSHA, nil
				case "merge-base":
					if args[3] == otherSHA && args[4] == testSHA {
						return "", &GitError{Code: 1}
					}
					return "", errors.New("behind failed")
				}
				return "", nil
			},
			want: "behind failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := SyncToOrigin(ctx, Repo{Dir: t.TempDir(), Git: tc.git})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("SyncToOrigin(%s) err = %v, want %q", tc.name, err, tc.want)
			}
		})
	}
}
