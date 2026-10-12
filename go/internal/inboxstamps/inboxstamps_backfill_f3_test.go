package inboxstamps

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type gitReplyF3 struct {
	stdout string
	stderr string
	code   int
	err    error
}

type scriptGitF3 struct {
	replies map[string]gitReplyF3
	calls   []string
}

func (g *scriptGitF3) Capture(_ context.Context, args ...string) (string, string, int, error) {
	line := strings.Join(args, " ")
	g.calls = append(g.calls, line)
	for prefix, reply := range g.replies {
		if strings.HasPrefix(line, prefix) {
			return reply.stdout, reply.stderr, reply.code, reply.err
		}
	}
	return "", "", 0, nil
}

func rawF3(s string) json.RawMessage { return json.RawMessage(s) }

func TestPlanAgainst_NoStampsMeansNoPlanAndNoGitCall(t *testing.T) {
	for name, stamps := range map[string][]Stamp{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			g := &scriptGitF3{}

			plan, err := PlanAgainst(context.Background(), g, stamps, "origin/main")

			if err != nil || len(plan.Kept)+len(plan.Retired)+len(plan.Replay)+len(plan.Superseded) != 0 || len(g.calls) != 0 {
				t.Fatalf("plan=%+v err=%v calls=%v", plan, err, g.calls)
			}
		})
	}
}

func TestPlanAgainst_AFailingRemoteDiffEndsThePlanBeforeTheMergeBase(t *testing.T) {
	g := &scriptGitF3{replies: map[string]gitReplyF3{"diff": {stderr: "bad revision", code: 128}}}
	stamp := Stamp{Path: untouchedItem, Set: map[string]json.RawMessage{"route": rawF3(`"x"`)}}

	plan, err := PlanAgainst(context.Background(), g, []Stamp{stamp}, "origin/main")

	if err == nil || !strings.Contains(err.Error(), "git diff: rc=128: bad revision") {
		t.Fatalf("err = %v", err)
	}
	if len(plan.Kept) != 0 || len(g.calls) != 1 {
		t.Fatalf("plan=%+v calls=%v, want the merge-base never asked", plan, g.calls)
	}
}

func TestRemoteSideItemAt_ReadsTheRemoteCopyThenTheMergeBaseCopy(t *testing.T) {
	const path = ".evolve/inbox/a.json"
	cases := []struct {
		name        string
		replies     map[string]gitReplyF3
		wantErr     string
		wantPresent bool
		wantBase    bool
	}{
		{name: "both copies exist", replies: map[string]gitReplyF3{
			"show REF:" + path:  {stdout: `{"id":"now"}`},
			"show BASE:" + path: {stdout: `{"id":"base"}`},
		}, wantPresent: true, wantBase: true},
		{name: "the item is new since the merge base", replies: map[string]gitReplyF3{
			"cat-file -e BASE:" + path: {code: 1},
			"show REF:" + path:         {stdout: `{"id":"now"}`},
		}, wantPresent: true},
		{name: "origin removed the item", replies: map[string]gitReplyF3{
			"cat-file -e REF:" + path: {code: 1},
			"show BASE:" + path:       {stdout: `{"id":"base"}`},
		}, wantBase: true},
		{name: "the remote copy is not an object", replies: map[string]gitReplyF3{
			"show REF:" + path: {stdout: `[1,2]`},
		}, wantErr: "not a JSON object"},
		{name: "the merge-base copy is not an object", replies: map[string]gitReplyF3{
			"show REF:" + path:  {stdout: `{"id":"now"}`},
			"show BASE:" + path: {stdout: `{torn`},
		}, wantErr: path + " at BASE is not a JSON object"},
		{name: "the merge-base copy cannot be shown", replies: map[string]gitReplyF3{
			"show REF:" + path:  {stdout: `{"id":"now"}`},
			"show BASE:" + path: {stderr: "bad object", code: 128},
		}, wantErr: "git show: rc=128: bad object"},
		{name: "the merge-base probe cannot run", replies: map[string]gitReplyF3{
			"show REF:" + path:         {stdout: `{"id":"now"}`},
			"cat-file -e BASE:" + path: {err: errors.New("no git")},
		}, wantErr: "no git"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := remoteSide{g: &scriptGitF3{replies: tc.replies}, base: "BASE", ref: "REF"}

			item, err := remote.itemAt(context.Background(), path)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || item.present || item.now != nil || item.atBase != nil {
					t.Fatalf("item=%+v err=%v, want error containing %q", item, err, tc.wantErr)
				}
				return
			}
			if err != nil || item.present != tc.wantPresent || (item.atBase != nil) != tc.wantBase {
				t.Fatalf("item=%+v err=%v", item, err)
			}
		})
	}
}

func TestPlanPrepareAndCommits_DoNothingAndCallNoGitWhenThereIsNothingToDo(t *testing.T) {
	stamp := Stamp{Path: untouchedItem, Set: map[string]json.RawMessage{"route": rawF3(`"x"`)}}
	cases := map[string]func(ctx context.Context, g Git) error{
		"prepare with nothing prepared":   func(ctx context.Context, g Git) error { return Plan{Kept: []Stamp{stamp}}.Prepare(ctx, g) },
		"commit kept with nothing kept":   func(ctx context.Context, g Git) error { return Plan{Replay: []Stamp{stamp}}.CommitKept(ctx, g) },
		"commit replay with no replay":    func(ctx context.Context, g Git) error { return Plan{Kept: []Stamp{stamp}}.CommitReplay(ctx, g) },
		"commit paths with an empty list": func(ctx context.Context, g Git) error { return commitPaths(ctx, g, "msg", nil) },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			g := &scriptGitF3{}

			if err := call(context.Background(), g); err != nil || len(g.calls) != 0 {
				t.Fatalf("err=%v calls=%v", err, g.calls)
			}
		})
	}
}

func TestPlanPrepare_RestoresExactlyThePreparedPathsFromHead(t *testing.T) {
	g := &scriptGitF3{}
	plan := Plan{prepared: []Stamp{{Path: untouchedItem}, {Path: curatedItem}}}

	if err := plan.Prepare(context.Background(), g); err != nil {
		t.Fatal(err)
	}

	want := "restore --source=HEAD --staged --worktree -- " + untouchedItem + " " + curatedItem
	if len(g.calls) != 1 || g.calls[0] != want {
		t.Fatalf("calls = %q, want %q", g.calls, want)
	}
}

func TestWorktreeRead_ReturnsTheFileBytesOrNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, untouchedItem), []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := worktree{root: root}

	if got := w.read(untouchedItem); string(got) != `{"a":1}` {
		t.Fatalf("read = %q", got)
	}
	if got := w.read(".evolve/inbox/absent.json"); got != nil {
		t.Fatalf("an unreadable file reads as nothing, got %q", got)
	}
	if got := w.read(".evolve/inbox"); got != nil {
		t.Fatalf("a directory reads as nothing, got %q", got)
	}
}

func TestLifecycleOnlyChange_AnItemTheWorktreeCannotDecodeIsNotAStamp(t *testing.T) {
	headItem := gitReplyF3{stdout: `{"failure_count":1}`}
	cases := []struct {
		name   string
		onDisk *string
		want   bool
	}{
		{name: "file removed from the worktree", onDisk: nil},
		{name: "not json", onDisk: ptrF3(`{torn`)},
		{name: "json null", onDisk: ptrF3(`null`)},
		{name: "a json array", onDisk: ptrF3(`[1]`)},
		{name: "a lifecycle field moved", onDisk: ptrF3(`{"failure_count":2}`), want: true},
		{name: "a field the mover never writes moved", onDisk: ptrF3(`{"failure_count":1,"title":"edited"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.onDisk != nil {
				if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, untouchedItem), []byte(*tc.onDisk), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			g := &scriptGitF3{replies: map[string]gitReplyF3{"show HEAD:" + untouchedItem: headItem}}

			stamp, ok, err := worktree{root: root}.lifecycleOnlyChange(context.Background(), g, " M", untouchedItem)

			if err != nil || ok != tc.want {
				t.Fatalf("stamp=%+v ok=%v err=%v", stamp, ok, err)
			}
			if tc.want && (string(stamp.Set["failure_count"]) != "2" || stamp.Path != untouchedItem) {
				t.Fatalf("stamp = %+v", stamp)
			}
			if !tc.want && (stamp.Path != "" || stamp.Set != nil || stamp.Removed != nil) {
				t.Fatalf("a refused stamp is empty, got %+v", stamp)
			}
		})
	}
}

func ptrF3(s string) *string { return &s }

func TestJSONEqual_ComparesValuesNotBytesAndFallsBackToBytesForInvalidJSON(t *testing.T) {
	cases := []struct {
		name string
		a, b json.RawMessage
		want bool
	}{
		{name: "identical numbers", a: rawF3(`1`), b: rawF3(`1`), want: true},
		{name: "same number spelled differently", a: rawF3(`1`), b: rawF3(`1.0`), want: true},
		{name: "different numbers", a: rawF3(`1`), b: rawF3(`2`), want: false},
		{name: "objects differing only in spacing and order", a: rawF3(`{"a":1,"b":[1,2]}`), b: rawF3(`{ "b": [1, 2], "a": 1 }`), want: true},
		{name: "arrays are ordered", a: rawF3(`[1,2]`), b: rawF3(`[2,1]`), want: false},
		{name: "string versus number", a: rawF3(`"1"`), b: rawF3(`1`), want: false},
		{name: "null versus absent bytes", a: rawF3(`null`), b: nil, want: false},
		{name: "both absent", a: nil, b: nil, want: true},
		{name: "identical invalid bytes", a: rawF3(`{torn`), b: rawF3(`{torn`), want: true},
		{name: "different invalid bytes", a: rawF3(`{torn`), b: rawF3(`{tore`), want: false},
		{name: "invalid against valid", a: rawF3(`{torn`), b: rawF3(`{}`), want: false},
		{name: "valid against invalid", a: rawF3(`{}`), b: rawF3(`{torn`), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonEqual(tc.a, tc.b); got != tc.want {
				t.Fatalf("jsonEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if jsonEqual(tc.b, tc.a) != tc.want {
				t.Fatalf("jsonEqual must be symmetric for %q, %q", tc.a, tc.b)
			}
		})
	}
}

func FuzzJSONEqualF3(f *testing.F) {
	for _, seed := range []string{`1`, `1.0`, `{"a":1}`, `{ "a" : 1 }`, `[1,2]`, `"x"`, `null`, `{torn`, ``, `true`, `1e3`, `1000`} {
		f.Add([]byte(seed), []byte(seed))
		f.Add([]byte(seed), []byte(`{}`))
	}
	f.Fuzz(func(t *testing.T, a, b []byte) {
		if !jsonEqual(a, a) {
			t.Fatalf("jsonEqual must be reflexive for %q", a)
		}
		if jsonEqual(a, b) != jsonEqual(b, a) {
			t.Fatalf("jsonEqual must be symmetric for %q, %q", a, b)
		}
	})
}

func TestChangedKeys_APresenceChangeCountsEvenWhenTheBytesAreEqual(t *testing.T) {
	before := map[string]json.RawMessage{"same": rawF3(`1`), "moved": rawF3(`1`), "gone": nil}
	after := map[string]json.RawMessage{"same": rawF3(`1.0`), "moved": rawF3(`2`), "new": nil}

	got := changedKeys(before, after)

	if strings.Join(got, ",") != "gone,moved,new" {
		t.Fatalf("changedKeys = %v, want the sorted removed, moved and added keys only", got)
	}
}
