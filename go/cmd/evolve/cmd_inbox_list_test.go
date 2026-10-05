package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

type listedJSON struct {
	Item struct {
		ID string `json:"id"`
	} `json:"item"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func listJSON(t *testing.T, args ...string) []listedJSON {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if rc := runInbox(append([]string{"list", "--json"}, args...), nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("list %v: rc = %d stderr = %q", args, rc, stderr.String())
	}
	var out []listedJSON
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	return out
}

func listedIDs(items []listedJSON) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Item.ID
	}
	slices.Sort(ids)
	return ids
}

func TestCmd_InboxList_ConsoleStatusIsExactlyWhatBatchesCallsOperatorOwned(t *testing.T) {
	curationRoot(t, map[string]string{
		"a.json": readyItem,
		"b.json": consoleItem,
		"c.json": waitingItem,
		"d.json": `{"id":"routed-one","kind":"feature","weight":0.2,"route":"console-manual"}`,
		"e.json": `{"id":"protected-one","kind":"bug","weight":0.2,"files":["go/internal/guards/phase.go"]}`,
	})
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"batches", "--json"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("batches: rc = %d", rc)
	}
	var batches inboxBatchesDoc
	if err := json.Unmarshal(stdout.Bytes(), &batches); err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, ex := range batches.ConsoleRouted {
		want = append(want, ex.Item.ID)
	}
	slices.Sort(want)

	got := listJSON(t, "--status", "console")

	if !slices.Equal(listedIDs(got), want) || len(want) != 3 {
		t.Errorf("list --status console = %v, batches' operator-owned = %v", listedIDs(got), want)
	}
	for _, it := range got {
		if it.Status != "console" || it.Reason == "" || !strings.HasPrefix(it.Path, ".evolve/inbox/") {
			t.Errorf("entry = %+v, want its status, reason and path", it)
		}
	}
}

func TestCmd_InboxList_FiltersByStatusKindAndRoute(t *testing.T) {
	curationRoot(t, map[string]string{
		"a.json": readyItem,
		"b.json": consoleItem,
		"c.json": waitingItem,
		"d.json": `{"id":"routed-one","kind":"feature","weight":0.2,"route":"console-manual"}`,
	})
	for name, tc := range map[string]struct {
		args []string
		want []string
	}{
		"everything":          {nil, []string{"console-one", "ready-one", "routed-one", "waiting-one"}},
		"ready":               {[]string{"--status", "ready"}, []string{"ready-one"}},
		"waiting":             {[]string{"--status", "waiting"}, []string{"waiting-one"}},
		"a kind":              {[]string{"--kind", "pipeline-repair"}, []string{"console-one"}},
		"a route":             {[]string{"--route", "console-manual"}, []string{"routed-one"}},
		"a kind and a status": {[]string{"--kind", "feature", "--status", "console"}, []string{"routed-one"}},
	} {
		if got := listedIDs(listJSON(t, tc.args...)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: list = %v, want %v", name, got, tc.want)
		}
	}
}

func TestCmd_InboxList_PrintsOneLinePerItem(t *testing.T) {
	menuRoot(t)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"list", "--status", "waiting"}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "waiting-one") || !strings.Contains(stdout.String(), "deps unmet: needs ready-one") ||
		strings.Contains(stdout.String(), "console-one") {
		t.Errorf("rc = %d stdout = %q", rc, stdout.String())
	}
}

func TestCmd_InboxList_RefusesABadFilter(t *testing.T) {
	menuRoot(t)
	for _, args := range [][]string{{"list", "--status", "claimed"}, {"list", "extra"}, {"list", "--kind"}} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 {
			t.Errorf("%v: rc = %d, want 10", args, rc)
		}
	}
}
