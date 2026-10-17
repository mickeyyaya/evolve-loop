package cycleoutcome

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const (
	auditorPersona = "agents/evolve-auditor.md"
	routedItem     = "routed-item"
	keptPredicate  = "package p\nfunc TestC1933_004(t *testing.T) { os.ReadFile(\"agents/evolve-auditor.md\") }\n"
)

func protectsAuditor(p string) bool { return p == auditorPersona }

type protectedScenario struct {
	route     committedset.ProtectedRoute
	predicate string
	protected func(string) bool
	committed []string
	drop      bool
}

func defaultScenario() protectedScenario {
	return protectedScenario{
		route:     committedset.ProtectedRoute{Cycle: 7, ItemID: routedItem},
		predicate: keptPredicate,
		protected: protectsAuditor,
		committed: []string{routedItem},
	}
}

func runProtectedScenario(t *testing.T, sc protectedScenario) (root, inbox string, stderr *bytes.Buffer) {
	t.Helper()
	root, inbox, ws := seedProject(t, routedItem, sc.committed)
	item, _ := json.Marshal(map[string]any{"id": routedItem, "title": "fixture", "kind": "bug", "files": []string{"go/internal/phases/audit/"}})
	if err := os.WriteFile(filepath.Join(inbox, "2026-07-29T00-00-00Z-"+routedItem+".json"), item, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}, routedItem, "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeProtectedPredicateRoute, routedItem)
	if err := committedset.WriteProtectedRoute(ws, sc.route); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(ws, committedset.ProtectedPredicatesDir)
	if err := os.MkdirAll(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "predicates_test.go"), []byte(sc.predicate), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr = &bytes.Buffer{}
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, stderr)
	in.Protected = sc.protected
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	return root, inbox, stderr
}

func stuckInProcessing(root string) bool {
	entries, _ := os.ReadDir(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-7"))
	return len(entries) > 0
}

func TestApplyFailure_AProvenProtectedPredicateRoutesTheItemWithItsFileAndReason(t *testing.T) {
	root, inbox, stderr := runProtectedScenario(t, defaultScenario())
	rec := itemRecord(t, inbox, routedItem)
	if rec["route"] != "console-manual" || rec["routed_cycle"] != float64(7) || rec["routed_at"] == nil {
		t.Errorf("route fields: %v", rec)
	}
	reason, _ := rec["routed_reason"].(string)
	if !strings.Contains(reason, auditorPersona) || !strings.Contains(reason, "TestC1933_004") || !strings.Contains(reason, "predicates_test.go") {
		t.Errorf("routed_reason must name the file and the test function: %q", reason)
	}
	files, _ := rec["files"].([]any)
	if len(files) != 2 || files[0] != "go/internal/phases/audit/" || files[1] != auditorPersona {
		t.Errorf("files keep the declared entries and gain the proven file: %v", files)
	}
	if _, bumped := rec["failure_count"]; bumped || stuckInProcessing(root) {
		t.Errorf("a routed item is not bumped and not stranded: %v", rec)
	}
	if strings.Contains(stderr.String(), "[cycleoutcome] WARN") {
		t.Errorf("a proven route is quiet: %s", stderr)
	}
}

func TestApplyFailure_AnUnprovenRouteReleasesTheClaimBumpsAndSaysSo(t *testing.T) {
	cases := map[string]func(sc *protectedScenario){
		"the committed set is not the one item": func(sc *protectedScenario) { sc.committed = []string{routedItem, "other"} },
		"the artifact is for another cycle":     func(sc *protectedScenario) { sc.route.Cycle = 3 },
		"the artifact names another item":       func(sc *protectedScenario) { sc.route.ItemID = "elsewhere" },
		"the membership test is not wired":      func(sc *protectedScenario) { sc.protected = nil },
		"the membership test denies the path":   func(sc *protectedScenario) { sc.protected = func(string) bool { return false } },
		"the kept predicate no longer reads it": func(sc *protectedScenario) { sc.predicate = "package p\nfunc TestX(t *testing.T) {}\n" },
		"the kept predicate does not parse":     func(sc *protectedScenario) { sc.predicate = "package p\nfunc (" },
	}
	for name, mutate := range cases {
		sc := defaultScenario()
		mutate(&sc)
		root, inbox, stderr := runProtectedScenario(t, sc)
		rec := itemRecord(t, inbox, routedItem)
		if rec["route"] != nil {
			t.Errorf("%s: an unproven route must not route: %v", name, rec)
		}
		if stuckInProcessing(root) {
			t.Errorf("%s: the claim is stranded in processing", name)
		}
		if !strings.Contains(stderr.String(), "[cycleoutcome] WARN") {
			t.Errorf("%s: the closeout must say why it did not route: %q", name, stderr.String())
		}
	}
}

func TestApplyFailure_NoArtifactIsAnUnprovenRoute(t *testing.T) {
	root, inbox, ws := seedProject(t, routedItem, []string{routedItem})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}, routedItem, "7"); err != nil {
		t.Fatal(err)
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeProtectedPredicateRoute, routedItem)
	var stderr bytes.Buffer
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, &stderr)
	in.Protected = protectsAuditor
	if _, err := ApplyFailure(in); err != nil {
		t.Fatal(err)
	}
	if itemRecord(t, inbox, routedItem)["route"] != nil || stuckInProcessing(root) || !strings.Contains(stderr.String(), "no route artifact") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestApplyFailure_AnItemThatLeftTheInboxDoesNotStrandOrFault(t *testing.T) {
	sc := defaultScenario()
	root, inbox, ws := seedProject(t, routedItem, sc.committed)
	if err := os.Remove(filepath.Join(inbox, "2026-07-29T00-00-00Z-"+routedItem+".json")); err != nil {
		t.Fatal(err)
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeProtectedPredicateRoute, routedItem)
	if err := committedset.WriteProtectedRoute(ws, sc.route); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(ws, committedset.ProtectedPredicatesDir)
	if err := os.MkdirAll(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "p.go"), []byte(sc.predicate), 0o644); err != nil {
		t.Fatal(err)
	}
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, &bytes.Buffer{})
	in.Protected = protectsAuditor
	var stderr bytes.Buffer
	in.Stderr = &stderr
	if _, err := ApplyFailure(in); err != nil || stuckInProcessing(root) || !strings.Contains(stderr.String(), "could not gain its protected files") {
		t.Errorf("err=%v stderr=%q", err, stderr.String())
	}
}

func TestAddFiles_KeepsAnExistingEntryOnce(t *testing.T) {
	item := map[string]json.RawMessage{"files": json.RawMessage(`["a",` + `"` + auditorPersona + `"]`)}
	addFiles(item, []string{auditorPersona, "b"})
	if string(item["files"]) != `["a","agents/evolve-auditor.md","b"]` {
		t.Errorf("files = %s", item["files"])
	}
}
