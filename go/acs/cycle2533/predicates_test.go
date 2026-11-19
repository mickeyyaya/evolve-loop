//go:build acs

package cycle2533

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
	"github.com/mickeyyaya/evolve-loop/go/internal/kbgraph"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	recordID      = "2026-11-19-train-ejected-console-prs"
	otherRecordID = "2026-11-19-watchdog-overfired"
	legacyID      = "2026-08-09-zero-ship-batch"
	trackedItem   = "edit-layout-c2-regenerate-generated-conflicts"
	otherItem     = "watchdog-grace-window"
)

type fixtureRecord struct {
	id          string
	title       string
	fingerprint string
	signals     string
	prs         string
	items       string
	lessons     string
}

func (f fixtureRecord) document() string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nschema: %s\nid: %s\ntitle: %s\nlevel: record\nstatus: fixed\n", incidentkb.Schema, f.id, f.title)
	fmt.Fprintf(&b, "selfheal: inc-2026-11-19-ab12\nfingerprint: %s\nsignals: %s\nprs: %s\nitems: %s\n", f.fingerprint, f.signals, f.prs, f.items)
	b.WriteString("actions: []\neffectiveness:\n  metric: {code: SHIP_INTEGRATION_EJECTED, filters: [], unit: wave, since: 051ad74bf}\n")
	b.WriteString("  baseline: {value: 4, units: 2}\n  target: 0\n  checks: 3\n  results: []\nverified_at: \"\"\n---\n\n")
	fmt.Fprintf(&b, "# %s\n", f.title)
	for _, section := range incidentkb.RequiredSections {
		fmt.Fprintf(&b, "\n## %s\n\n", section)
		if section == "Lessons" {
			b.WriteString(f.lessons + "\n")
		}
	}
	return b.String()
}

func trainRecord() fixtureRecord {
	return fixtureRecord{
		id:          recordID,
		title:       "the train ejected four console PRs",
		fingerprint: "integration-ejection:derived",
		signals:     `["cycle:2481:57", "channel:errors:1832"]`,
		prs:         "[1555, 1558]",
		items:       "[" + trackedItem + "]",
		lessons:     "Regenerate the generated files before the merge.\n\nA quokka sighting on the research index is the first sign of a stale shard.",
	}
}

func watchdogRecord() fixtureRecord {
	return fixtureRecord{
		id:          otherRecordID,
		title:       "the watchdog fired on a healthy phase",
		fingerprint: "watchdog:overfire",
		signals:     `["cycle:94:3"]`,
		prs:         "[1558, 1601]",
		items:       "[" + otherItem + "]",
		lessons:     "Give each phase a grace window sized by a pangolin of past runtimes.",
	}
}

func seed(t *testing.T, files map[string]string) {
	t.Helper()
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func projectWith(t *testing.T, records ...fixtureRecord) string {
	t.Helper()
	root := t.TempDir()
	for _, r := range records {
		seed(t, map[string]string{incidentkb.RecordPath(root, r.id): r.document()})
	}
	return root
}

func neighborsOf(t *testing.T, root, id string, hops int) kbgraph.Subgraph {
	t.Helper()
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatalf("kbgraph.Build: %v", err)
	}
	sub, err := g.Neighbors(id, hops)
	if err != nil {
		t.Fatalf("Neighbors(%s): %v", id, err)
	}
	return sub
}

func nodeIDs(sub kbgraph.Subgraph) []string {
	ids := make([]string, 0, len(sub.Nodes))
	for _, n := range sub.Nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

func hasEdge(sub kbgraph.Subgraph, from, to, kind string) bool {
	return slices.ContainsFunc(sub.Edges, func(e kbgraph.Edge) bool {
		return e.From == from && e.To == to && e.Kind == kind
	})
}

func hasNode(sub kbgraph.Subgraph, id, kind string) bool {
	return slices.ContainsFunc(sub.Nodes, func(n kbgraph.Node) bool { return n.ID == id && n.Kind == kind })
}

func TestC2533_001_IncidentNeighborsListCyclePRAndItem(t *testing.T) {
	root := projectWith(t, trainRecord())
	incident := "incident:" + recordID
	sub := neighborsOf(t, root, incident, 1)

	if !hasNode(sub, incident, "incident") {
		t.Errorf("nodes %v: want %s of kind incident", nodeIDs(sub), incident)
	}
	if !hasNode(sub, "cycle:2481", "cycle") || !hasEdge(sub, incident, "cycle:2481", "affected") {
		t.Errorf("want an affected edge from the incident to cycle:2481; nodes=%v edges=%v", nodeIDs(sub), sub.Edges)
	}
	if hasNode(sub, "cycle:1832", "cycle") {
		t.Errorf("signal channel:errors:1832 is not a cycle signal but produced cycle:1832; nodes=%v", nodeIDs(sub))
	}
	for _, pr := range []string{"1555", "1558"} {
		found := false
		for _, e := range sub.Edges {
			if e.From == incident && e.Kind == "fixed-by" && strings.HasSuffix(e.To, pr) && hasNode(sub, e.To, "pr") {
				found = true
			}
		}
		if !found {
			t.Errorf("want a fixed-by edge from the incident to a pr node for #%s; nodes=%v edges=%v", pr, nodeIDs(sub), sub.Edges)
		}
	}
	if !hasNode(sub, "item:"+trackedItem, "item") || !hasEdge(sub, incident, "item:"+trackedItem, "tracked-by") {
		t.Errorf("want a tracked-by edge from the incident to item:%s; nodes=%v edges=%v", trackedItem, nodeIDs(sub), sub.Edges)
	}
	if !hasNode(sub, "fingerprint:integration-ejection:derived", "fingerprint") ||
		!hasEdge(sub, incident, "fingerprint:integration-ejection:derived", "recurs-as") {
		t.Errorf("want a recurs-as edge from the incident to its fingerprint; nodes=%v edges=%v", nodeIDs(sub), sub.Edges)
	}
}

func TestC2533_002_IncidentJoinsTheInboxItemNode(t *testing.T) {
	root := projectWith(t, trainRecord())
	seed(t, map[string]string{filepath.Join(root, ".evolve", "inbox", "tracked.json"): fmt.Sprintf(`{"id":%q,"title":"regenerate generated conflicts","deps":["upstream-dep"]}`, trackedItem)})

	sub := neighborsOf(t, root, "incident:"+recordID, 2)

	if !hasEdge(sub, "item:"+trackedItem, "item:upstream-dep", "depends-on") {
		t.Errorf("the incident's item must be the inbox item node, reaching its dependency at 2 hops; nodes=%v edges=%v", nodeIDs(sub), sub.Edges)
	}
}

func TestC2533_003_IncidentsSharingAPRMeetAtOnePRNode(t *testing.T) {
	root := projectWith(t, trainRecord(), watchdogRecord())

	oneHop := neighborsOf(t, root, "incident:"+recordID, 1)
	if slices.Contains(nodeIDs(oneHop), "incident:"+otherRecordID) {
		t.Errorf("one hop from incident A must not reach incident B; nodes=%v", nodeIDs(oneHop))
	}
	if slices.ContainsFunc(nodeIDs(oneHop), func(id string) bool { return strings.HasSuffix(id, "1601") }) {
		t.Errorf("one hop from incident A must not reach B's own PR #1601; nodes=%v", nodeIDs(oneHop))
	}
	if slices.Contains(nodeIDs(oneHop), "item:"+otherItem) {
		t.Errorf("one hop from incident A must not reach B's item; nodes=%v", nodeIDs(oneHop))
	}

	twoHops := neighborsOf(t, root, "incident:"+recordID, 2)
	if !slices.Contains(nodeIDs(twoHops), "incident:"+otherRecordID) {
		t.Errorf("two hops through the shared PR #1558 must reach incident B; nodes=%v", nodeIDs(twoHops))
	}
}

func TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph(t *testing.T) {
	root := projectWith(t, trainRecord())
	seed(t, map[string]string{
		filepath.Join(incidentkb.Dir(root), legacyID+".md"):                 "# Zero-ship batch\n\nNo frontmatter here.\n",
		filepath.Join(incidentkb.Dir(root), "REGRESSION-COVERAGE-INDEX.md"): "# index\n",
	})

	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatalf("Build must tolerate legacy incident files: %v", err)
	}
	if _, err := g.Neighbors("incident:"+legacyID, 1); err == nil {
		t.Errorf("a legacy file with no frontmatter must not become an incident node")
	}
	if _, err := g.Neighbors("incident:"+recordID, 1); err != nil {
		t.Errorf("the schema record beside the legacy files must stay in the graph: %v", err)
	}

	broken := t.TempDir()
	seed(t, map[string]string{incidentkb.RecordPath(broken, recordID): "---\nschema: incident-record/1\nbogus: 1\n---\n\n# broken\n"})
	_, err = kbgraph.Build(broken)
	var parseErr *kbgraph.RecordParseError
	if !errors.As(err, &parseErr) || filepath.Base(parseErr.Path) != recordID+".md" {
		t.Errorf("Build of a record with an unparsable frontmatter must fail with a RecordParseError naming %s, got %v", recordID, err)
	}
}

func TestC2533_005_UnknownIncidentIsAnError(t *testing.T) {
	root := projectWith(t, trainRecord())
	g, err := kbgraph.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Neighbors("incident:2026-11-19-no-such-record", 1); err == nil {
		t.Errorf("Neighbors of an incident with no record returned a nil error")
	}
}

type cliNeighbors struct {
	stdout string
	stderr string
	code   int
}

func runKBNeighbors(t *testing.T, root, node string, extra ...string) cliNeighbors {
	t.Helper()
	args := append([]string{"-C", acsassert.GoModuleDir(t), "run", "./cmd/evolve", "kb", "graph", "neighbors", node, "--project-root", root}, extra...)
	out, errOut, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil && code == 0 {
		t.Fatalf("go run evolve: %v", err)
	}
	return cliNeighbors{stdout: out, stderr: errOut, code: code}
}

func TestC2533_006_CLINeighborsOfIncidentPrintsCyclePRAndItem(t *testing.T) {
	root := projectWith(t, trainRecord())

	res := runKBNeighbors(t, root, "incident:"+recordID, "--json")
	if res.code != 0 {
		t.Fatalf("evolve kb graph neighbors exit=%d stderr=%s", res.code, res.stderr)
	}
	var sub kbgraph.Subgraph
	if err := json.Unmarshal([]byte(res.stdout), &sub); err != nil {
		t.Fatalf("decode --json output: %v\n%s", err, res.stdout)
	}
	if !hasEdge(sub, "incident:"+recordID, "cycle:2481", "affected") {
		t.Errorf("CLI output lacks the cycle edge: %v", sub.Edges)
	}
	if !hasEdge(sub, "incident:"+recordID, "item:"+trackedItem, "tracked-by") {
		t.Errorf("CLI output lacks the item edge: %v", sub.Edges)
	}
	if !slices.ContainsFunc(sub.Edges, func(e kbgraph.Edge) bool { return e.Kind == "fixed-by" && strings.HasSuffix(e.To, "1558") }) {
		t.Errorf("CLI output lacks the PR edge: %v", sub.Edges)
	}

	text := runKBNeighbors(t, root, "incident:"+recordID)
	for _, want := range []string{"node cycle:2481", "node item:" + trackedItem, "-fixed-by->"} {
		if !strings.Contains(text.stdout, want) {
			t.Errorf("line output lacks %q:\n%s", want, text.stdout)
		}
	}

	missing := runKBNeighbors(t, root, "incident:2026-11-19-no-such-record")
	if missing.code == 0 {
		t.Errorf("neighbors of an unknown incident exited 0; stdout=%s", missing.stdout)
	}
}

func lookupIDs(t *testing.T, roots []string, keyword string) []string {
	t.Helper()
	got, err := research.NewFileKB(roots).Lookup(context.Background(), research.Query{Keywords: []string{keyword}})
	if err != nil {
		t.Fatalf("Lookup(%q): %v", keyword, err)
	}
	ids := make([]string, 0, len(got))
	for _, l := range got {
		ids = append(ids, l.ID)
	}
	return ids
}

func TestC2533_007_RecallFindsARecordByAWordOfItsLesson(t *testing.T) {
	root := projectWith(t, trainRecord(), watchdogRecord())
	roots := []string{incidentkb.Dir(root)}

	if got := lookupIDs(t, roots, "quokka"); !slices.Equal(got, []string{recordID}) {
		t.Errorf("recall of a word in the second paragraph of the train lesson = %v, want only %s", got, recordID)
	}
	if got := lookupIDs(t, roots, "pangolin"); !slices.Equal(got, []string{otherRecordID}) {
		t.Errorf("recall of a word in the watchdog lesson = %v, want only %s", got, otherRecordID)
	}
	if got := lookupIDs(t, roots, "narwhal"); len(got) != 0 {
		t.Errorf("recall of a word that no lesson holds = %v, want none", got)
	}
}

func TestC2533_008_RecallSkipsLegacyIncidentFiles(t *testing.T) {
	root := projectWith(t, trainRecord())
	seed(t, map[string]string{filepath.Join(incidentkb.Dir(root), legacyID+".md"): "# Zero-ship batch\n\n## Lessons\n\nA narwhal is no lesson of a schema record.\n"})
	roots := []string{incidentkb.Dir(root)}

	if got := lookupIDs(t, roots, "quokka"); !slices.Equal(got, []string{recordID}) {
		t.Errorf("recall beside a legacy file = %v, want only %s", got, recordID)
	}
	if got := lookupIDs(t, roots, "narwhal"); slices.Contains(got, legacyID) {
		t.Errorf("legacy file %s surfaced as a recalled record: %v", legacyID, got)
	}
}

func TestC2533_009_DefaultRecallRootsIncludeDocsIncidents(t *testing.T) {
	root := projectWith(t, trainRecord())
	var roots []string
	for _, rel := range research.SearchPathsFromEnv(policy.PathsConfig{}) {
		roots = append(roots, filepath.Join(root, rel))
	}

	if got := lookupIDs(t, roots, "quokka"); !slices.Equal(got, []string{recordID}) {
		t.Errorf("recall through the default search paths = %v, want %s from docs/incidents", got, recordID)
	}
}

func TestC2533_010_ConfiguredSearchPathsStayExact(t *testing.T) {
	got := research.SearchPathsFromEnv(policy.PathsConfig{KBSearchPaths: "/a:/b"})
	if !slices.Equal(got, []string{"/a", "/b"}) {
		t.Errorf("an explicit kb_search_paths must stay exact, got %v", got)
	}
}
