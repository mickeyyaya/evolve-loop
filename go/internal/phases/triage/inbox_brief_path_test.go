package triage

import (
	"strings"
	"testing"
)

func TestInboxBatchesSection_AnItemWithABriefPathShowsThePathToTheNextLane(t *testing.T) {
	root := t.TempDir()
	const briefPath = "/waves/integration-7/brief-41.txt"
	writeInboxItem(t, root, "ejected.json", `{"id":"x-ejected","weight":0.9,"route":"lane","priority_class":"feature","brief_path":"`+briefPath+`"}`)
	writeInboxItem(t, root, "plain.json", `{"id":"x-plain","weight":0.4,"route":"lane","priority_class":"feature"}`)

	got := inboxBatchesSection(root, nil)

	if !strings.Contains(got, "x-ejected") || !strings.Contains(got, "x-plain") {
		t.Fatalf("section =\n%s\nwant both items on the menu", got)
	}
	if line := lineOf(got, "x-ejected: "); !strings.Contains(line, briefPath) {
		t.Errorf("item line %q does not carry the brief path", line)
	}
	if line := lineOf(got, "x-plain: "); strings.Contains(line, "brief") {
		t.Errorf("item line %q names a brief for an item that has none", line)
	}
}

func lineOf(text, prefix string) string {
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, prefix) {
			return l
		}
	}
	return ""
}
