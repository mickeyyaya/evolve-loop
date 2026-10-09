package explanationdocs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	rootInboxItem     = ".evolve/inbox/item.json"
	consumedInboxItem = ".evolve/inbox/consumed/item.json"
	releasedItemBody  = "{\"id\":\"item\",\"released_continuations\":[{\"cycle\":41}]}\n"
)

func (f fixture) sealRequiredWithAnInboxItem(t *testing.T) fixture {
	t.Helper()
	f.write(t, rootInboxItem, "{\"id\":\"item\"}\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "an inbox item at base")
	f.base = f.git(t, "rev-parse", "HEAD")
	f.sealRequired(t)
	return f
}

func (f fixture) consume(t *testing.T) {
	t.Helper()
	f.write(t, consumedInboxItem, releasedItemBody)
	if err := os.Remove(filepath.Join(f.worktree, filepath.FromSlash(rootInboxItem))); err != nil {
		t.Fatal(err)
	}
}

func TestRebind_TheSanctionedConsumptionKeepsTheIdentity(t *testing.T) {
	f := newFixture(t).sealRequiredWithAnInboxItem(t)
	f.consume(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "a peer lane's change\n"})

	rebound := f.rebind(t, newBase)

	if !rebound {
		t.Fatal("ship's consumption of an inbox item, pending beside the audited change, declined the identity")
	}
	rebased := f.binding()
	rebased.BaseSHA = newBase
	if _, active, err := Verify(context.Background(), rebased); err != nil || !active {
		t.Fatalf("Verify on the new base with the consumption pending = (%v, %v), want the rebound view to bind the pending tree", active, err)
	}
	if body, err := os.ReadFile(filepath.Join(f.worktree, filepath.FromSlash(consumedInboxItem))); err != nil || string(body) != releasedItemBody {
		t.Fatalf("the consumed item = (%q, %v), want its released continuation untouched", body, err)
	}
}

func TestRebind_AnyOtherInboxChangeStillDeclines(t *testing.T) {
	cases := map[string]func(t *testing.T, f fixture){
		"a consumed copy with no removal": func(t *testing.T, f fixture) {
			f.write(t, consumedInboxItem, releasedItemBody)
		},
		"a removal with no consumed copy": func(t *testing.T, f fixture) {
			if err := os.Remove(filepath.Join(f.worktree, filepath.FromSlash(rootInboxItem))); err != nil {
				t.Fatal(err)
			}
		},
		"a consumed copy under another name": func(t *testing.T, f fixture) {
			f.consume(t)
			if err := os.Rename(filepath.Join(f.worktree, filepath.FromSlash(consumedInboxItem)), filepath.Join(f.worktree, ".evolve", "inbox", "consumed", "other.json")); err != nil {
				t.Fatal(err)
			}
		},
		"an edit of the root item": func(t *testing.T, f fixture) {
			f.write(t, rootInboxItem, "{\"id\":\"item\",\"priority\":\"high\"}\n")
		},
		"a consumed copy that edits the item": func(t *testing.T, f fixture) {
			f.consume(t)
			f.write(t, consumedInboxItem, "{\"id\":\"item\",\"priority_class\":\"security\",\"released_continuations\":[{\"cycle\":41}]}\n")
		},
		"a new item at the inbox root": func(t *testing.T, f fixture) {
			f.write(t, ".evolve/inbox/new.json", "{\"id\":\"new\"}\n")
		},
		"a consumption pair beside a stray consumed copy": func(t *testing.T, f fixture) {
			f.consume(t)
			f.write(t, ".evolve/inbox/consumed/stray.json", "{\"id\":\"stray\"}\n")
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t).sealRequiredWithAnInboxItem(t)
			change(t, f)

			f.requireDeclinedWithoutWrites(t, f.commitPeer(t, map[string]string{"peer.txt": "peer\n"}))
		})
	}
}

func TestShipConsumption_NamesOnlyWholePairs(t *testing.T) {
	t.Parallel()
	states := []pathState{
		{rel: ".evolve/inbox/a.json", mode: "absent"},
		{rel: ".evolve/inbox/b.json", mode: "absent"},
		{rel: ".evolve/inbox/c.json", mode: "100644"},
		{rel: ".evolve/inbox/consumed/a.json", mode: "100644"},
		{rel: ".evolve/inbox/consumed/c.json", mode: "100644"},
		{rel: ".evolve/inbox/consumed/d.json", mode: "absent"},
		{rel: ".evolve/inbox/d.json", mode: "absent"},
		{rel: ".evolve/inbox/sub/e.json", mode: "absent"},
		{rel: ".evolve/inbox/consumed/sub/e.json", mode: "100644"},
		{rel: "docs/a.json", mode: "100644"},
		{rel: "lane.txt", mode: "100644"},
	}

	ships := map[string]bool{}
	for _, name := range consumptionPairs(states) {
		ships[name] = true
	}

	got := withoutShipConsumption(states, ships)

	want := []string{".evolve/inbox/b.json", ".evolve/inbox/c.json", ".evolve/inbox/consumed/c.json", ".evolve/inbox/consumed/d.json",
		".evolve/inbox/d.json", ".evolve/inbox/sub/e.json", ".evolve/inbox/consumed/sub/e.json", "docs/a.json", "lane.txt"}
	if len(got) != len(want) {
		t.Fatalf("withoutShipConsumption kept %v, want %v: only a root removal paired with a present consumed copy of the same name is ship's", got, want)
	}
	for i, s := range got {
		if s.rel != want[i] {
			t.Fatalf("withoutShipConsumption kept %v, want %v", got, want)
		}
	}
}

func TestSameChange_DeclinesEveryCheckAfterTheDiffDigest(t *testing.T) {
	states := []pathState{{rel: "go/lane.go", mode: "100644", contentSHA: "aa", contentBytes: 2}}
	changed := []string{"go/lane.go"}
	sealed := func() *resultSnapshot {
		prior := &resultSnapshot{MaterialSHA256: foldPathStates(materialDomain, states)}
		prior.View.BaseSHA = "base"
		prior.View.DiffSHA256 = foldPathStates(diffDomain("base"), states)
		prior.View.MaterialPaths = []string{"go/lane.go"}
		prior.View.Status = statusRequired
		prior.View.DocumentPath = "docs/explain/builds/doc.md"
		return prior
	}
	requiredReport := "## Explanation Documentation\n- Status: REQUIRED\n- Document: docs/explain/builds/doc.md\n"
	cases := map[string]struct {
		edit    func(*resultSnapshot)
		report  string
		wantErr bool
	}{
		"other material paths":           {edit: func(p *resultSnapshot) { p.View.MaterialPaths = []string{"go/other.go"} }, report: requiredReport},
		"another material digest":        {edit: func(p *resultSnapshot) { p.MaterialSHA256 = "drifted" }, report: requiredReport},
		"no build report":                {edit: func(*resultSnapshot) {}, wantErr: true},
		"a declaration of another state": {edit: func(*resultSnapshot) {}, report: "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: none\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			if tc.report != "" {
				if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte(tc.report), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			prior := sealed()
			tc.edit(prior)

			same, err := sameChange(context.Background(), CycleBinding{Workspace: workspace}, prior, changed, states, "new-base")

			if same || (err != nil) != tc.wantErr {
				t.Fatalf("sameChange = (%v, %v), want a decline (error %v)", same, err, tc.wantErr)
			}
		})
	}
}

func TestKeepsTheItem_OnlyShipsStampsMayDiffer(t *testing.T) {
	t.Parallel()
	item := []byte("{\"id\":\"item\",\"weight\":0.5}\n")
	cases := map[string]struct {
		before, after []byte
		want          bool
	}{
		"ship's stamps, re-indented":  {item, []byte("{\n  \"consumed\": {\"via\": \"ship\"},\n  \"id\": \"item\",\n  \"released_continuations\": [{}],\n  \"weight\": 0.5\n}\n"), true},
		"an edited field":             {item, []byte("{\"id\":\"item\",\"weight\":0.9,\"consumed\":{}}\n"), false},
		"an added field":              {item, []byte("{\"id\":\"item\",\"weight\":0.5,\"files\":[\"x\"]}\n"), false},
		"an unreadable consumed copy": {item, []byte("not json"), false},
		"an unreadable base item":     {[]byte("not json"), item, false},
	}
	for name, tc := range cases {
		if got := keepsTheItem(tc.before, tc.after); got != tc.want {
			t.Errorf("%s: keepsTheItem = %v, want %v", name, got, tc.want)
		}
	}
}

func TestShipConsumption_AnUnreadableItemIsAnError(t *testing.T) {
	f := newFixture(t).sealRequiredWithAnInboxItem(t)
	f.consume(t)
	states := []pathState{{rel: rootInboxItem, mode: "absent"}, {rel: consumedInboxItem, mode: "100644"}}

	if _, err := shipConsumption(context.Background(), f.worktree, strings.Repeat("0", 40), states); err == nil {
		t.Fatal("shipConsumption read an inbox item at a base git does not hold")
	}
	if err := os.Remove(filepath.Join(f.worktree, filepath.FromSlash(consumedInboxItem))); err != nil {
		t.Fatal(err)
	}
	if _, err := shipConsumption(context.Background(), f.worktree, f.base, states); err == nil {
		t.Fatal("shipConsumption read a consumed copy that is not in the worktree")
	}
}

func TestSameChange_AnUnreadableConsumptionIsAnError(t *testing.T) {
	f := newFixture(t).sealRequiredWithAnInboxItem(t)
	states := []pathState{{rel: rootInboxItem, mode: "absent"}, {rel: consumedInboxItem, mode: "100644"}}

	same, err := sameChange(context.Background(), f.binding(), &resultSnapshot{}, nil, states, strings.Repeat("0", 40))

	if same || err == nil {
		t.Fatalf("sameChange = (%v, %v), want an error for an item git cannot read", same, err)
	}
}
