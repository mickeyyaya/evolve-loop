package kbgraph

import (
	"errors"
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

func writeIncidentFile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "incidents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuild_IncidentsAddEdgesAndSkipLegacy(t *testing.T) {
	root := t.TempDir()
	cyclesDir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(cyclesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cyclesDir, "cycle-120.json"), []byte(`{"cycle":120,"final_verdict":"PASS"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeInbox(t, root, "fix.json", `{"id":"fix-item","title":"fix item","connects_to":["root-dep"],"deps":["root-dep"]}`)
	writeIncidentFile(t, root, "2026-08-09-old.md", "# old legacy\n\nNo frontmatter.\n")
	writeIncidentFile(t, root, "REGRESSION-COVERAGE-INDEX.md", "# index\n")
	writeIncidentFile(t, root, "2026-11-19-no-id.md", "---\nschema: incident-record/1\ntitle: fallback id\nlevel: record\nstatus: open\n---\n\n# fallback\n")
	writeIncidentFile(t, root, "2026-11-19-sample.md", `---
schema: incident-record/1
id: 2026-11-19-sample
title: sample incident
level: record
status: fixed
fingerprint: sample:fp
signals: ["cycle:120:4", "cycle:120:9", "cycle:bad:1", "channel:errors:77"]
prs: [301, 302]
items: [fix-item]
actions: []
effectiveness:
  metric: {code: SAMPLE_ERR, filters: [], unit: wave, since: abc123}
  baseline: {value: 2, units: 1}
  target: 0
  checks: 2
  results: []
verified_at: ""
---

# sample incident
`)

	g, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := g.Neighbors("incident:2026-08-09-old", 1); err == nil {
		t.Error("legacy file must not produce an incident node")
	}
	if _, err := g.Neighbors("incident:2026-11-19-no-id", 1); err != nil {
		t.Errorf("record without id must fall back to filename: %v", err)
	}
	sub, err := g.Neighbors("incident:2026-11-19-sample", 2)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if slices.ContainsFunc(sub.Nodes, func(n Node) bool { return n.ID == "cycle:77" }) {
		t.Errorf("non-cycle signal created cycle:77: %v", sub.Nodes)
	}
	wantEdges := []Edge{
		{From: "incident:2026-11-19-sample", To: "cycle:120", Kind: KindAffected},
		{From: "incident:2026-11-19-sample", To: "fingerprint:sample:fp", Kind: KindRecursAs},
		{From: "incident:2026-11-19-sample", To: "pr:301", Kind: KindFixedBy},
		{From: "incident:2026-11-19-sample", To: "pr:302", Kind: KindFixedBy},
		{From: "incident:2026-11-19-sample", To: "item:fix-item", Kind: KindTrackedBy},
		{From: "item:fix-item", To: "item:root-dep", Kind: KindDependsOn},
	}
	for _, want := range wantEdges {
		if !slices.Contains(sub.Edges, want) {
			t.Errorf("missing edge %+v in %+v", want, sub.Edges)
		}
	}
	affectedCount := 0
	for _, e := range sub.Edges {
		if e.From == "incident:2026-11-19-sample" && e.To == "cycle:120" && e.Kind == KindAffected {
			affectedCount++
		}
	}
	if affectedCount != 1 {
		t.Errorf("duplicate affected edges for repeated cycle signals: got %d, want 1", affectedCount)
	}
}

func TestBuild_UnparsableIncidentReturnsErrorNamingFile(t *testing.T) {
	root := t.TempDir()
	writeIncidentFile(t, root, "2026-11-19-bad.md", "---\nschema: incident-record/1\nbogus: 1\n---\n\n# bad\n")
	_, err := Build(root)
	var parseErr *RecordParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("Build error = %v, want a *RecordParseError", err)
	}
	if want := filepath.Join(root, "docs", "incidents", "2026-11-19-bad.md"); parseErr.Path != want {
		t.Errorf("RecordParseError.Path = %q, want %q", parseErr.Path, want)
	}
	if parseErr.Err == nil || errors.Unwrap(parseErr) != parseErr.Err {
		t.Fatalf("RecordParseError must unwrap to the parse failure %v", parseErr.Err)
	}
	if got, want := parseErr.Error(), "kbgraph: parse "+parseErr.Path+": "+parseErr.Err.Error(); got != want {
		t.Errorf("RecordParseError message = %q, want %q", got, want)
	}
}

func TestBuild_IncidentReadErrors(t *testing.T) {
	notADir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(notADir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notADir, "docs", "incidents"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(notADir); err == nil {
		t.Error("docs/incidents that is not a directory must fail Build")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "docs", "incidents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "gone"), filepath.Join(dir, "2026-11-19-dangling.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root); err == nil {
		t.Error("unreadable incident symlink must fail Build")
	}
}
