package kbgraph

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeInbox(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNeighborsHopsTable(t *testing.T) {
	root := t.TempDir()
	writeInbox(t, root, "a.json", `{"id":"a","connects_to":["b"],"deps":["z"]}`)
	writeInbox(t, root, "b.json", `{"id":"b","connects_to":["c"]}`)
	writeInbox(t, root, "c.json", `{"id":"c"}`)
	g, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		hops    int
		want    []string
		notWant string
	}{
		{0, []string{"item:b", "item:z"}, "item:c"},
		{1, []string{"item:b", "item:z"}, "item:c"},
		{2, []string{"item:b", "item:c", "item:z"}, ""},
	}
	for _, c := range cases {
		sub, err := g.Neighbors("item:a", c.hops)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, n := range sub.Nodes {
			ids = append(ids, n.ID)
		}
		for _, w := range c.want {
			if !slices.Contains(ids, w) {
				t.Errorf("hops=%d nodes=%v missing %s", c.hops, ids, w)
			}
		}
		if c.notWant != "" && slices.Contains(ids, c.notWant) {
			t.Errorf("hops=%d nodes=%v must not hold %s", c.hops, ids, c.notWant)
		}
	}
}

func TestBuildRejectsMalformedInboxItem(t *testing.T) {
	root := t.TempDir()
	writeInbox(t, root, "bad.json", `{`)
	if _, err := Build(root); err == nil {
		t.Error("Build accepted a malformed inbox item")
	}
}

func TestBuildNamesAnItemWithoutIDByItsFileName(t *testing.T) {
	root := t.TempDir()
	writeInbox(t, root, "anon.json", `{"title":"x"}`)
	g, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := g.nodes["item:anon"]; !ok || len(g.nodes) != 1 {
		t.Errorf("nodes = %v, want the one item named by its file, item:anon", g.nodes)
	}
}

func TestBuild_InboxReadFailuresAreErrorsAndNonItemsAreSkipped(t *testing.T) {
	notADir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(notADir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notADir, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(notADir); err == nil {
		t.Error("an inbox that is not a directory is an error")
	}
	root := t.TempDir()
	writeInbox(t, root, "notes.txt", "not an item")
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "gone"), filepath.Join(root, ".evolve", "inbox", "dangling.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root); err == nil {
		t.Error("an inbox item it cannot read is an error")
	}
	if err := os.Remove(filepath.Join(root, ".evolve", "inbox", "dangling.json")); err != nil {
		t.Fatal(err)
	}
	g, err := Build(root)
	if err != nil || len(g.nodes) != 0 {
		t.Errorf("a note and a subdirectory are not items: nodes=%v err=%v", g.nodes, err)
	}
}
