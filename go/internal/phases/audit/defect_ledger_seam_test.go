package audit

// defect_ledger_seam_test.go — ADR-0103 unit 09: the audit package's seam onto
// internal/core/defectledger — the Null-Object facades carry the REAL lane-scope
// reader and resolver, Config.Signals reaches the ledger, ONE construction
// site, the production spellings, and the ordered signal stream a blocked
// continuation leaves.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingAccessor() (func() *signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return func() *signalcenter.Center { return c }, got
}

func streamOf(events []signalcenter.Event) string {
	var parts []string
	for _, e := range events {
		parts = append(parts, string(e.Module)+"/"+string(e.Kind)+"/"+string(e.Code))
	}
	return strings.Join(parts, " ")
}

// Test 40 — the free facade runs on a Null-Object ledger built with the REAL
// core.LaneScopeIDs: the cycle-1285 F2 fixture (registry + lane-scope pin,
// manifest deleted) still blocks with the registry finding. A source scan
// pins that the one wired construction spells the real collaborators.
func TestDefectLedgerSeam_NullFacadeKeepsTheRegistryFallback(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if err := os.Remove(filepath.Join(ws, "continuation-manifest.json")); err != nil {
		t.Fatal(err)
	}
	diags, blocked, lineage := reconcileContinuationDefects(req)
	if !blocked || lineage != nil || len(diags) == 0 || !strings.Contains(diags[0].Message, "the manifest was deleted or never written") {
		t.Fatalf("the null facade arms from the registry through the real lane-scope reader:\n%s", diagsText(diags))
	}
	src, err := os.ReadFile("defect_ledger.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "defectledger.New(core.LaneScopeIDs, resolveEvidence,") {
		t.Fatal("wiredDefectLedger must pass the real core.LaneScopeIDs and the audit resolver")
	}
}

// Test 41 — Config.Signals reaches the ledger through newHooks (the ONE hooks
// literal New builds); a wired Classify on the F2 fixture records
// AUDIT_LEDGER_MANIFEST_MISSING and keeps the verdict and diagnostics
// byte-identical to the null path.
func TestAuditConfig_SignalsReachTheLedger(t *testing.T) {
	acc, got := recordingAccessor()
	if !newHooks(Config{Signals: acc}).ledger.SignalsWired() {
		t.Fatal("Config.Signals wires the ledger")
	}
	if newHooks(Config{}).ledger.SignalsWired() || (hooks{}).defectLedger().SignalsWired() {
		t.Fatal("no Center ⇒ the Null Object, for New and for a hooks{} literal alike")
	}
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if err := os.Remove(filepath.Join(ws, "continuation-manifest.json")); err != nil {
		t.Fatal(err)
	}
	nullVerdict, nullDiags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	wired := hooks{ledger: wiredDefectLedger(acc)}
	verdict, diags, _ := wired.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict != nullVerdict || diagsText(diags) != diagsText(nullDiags) || verdict == core.VerdictPASS {
		t.Fatalf("wired and null paths agree byte-for-byte on the graded wire:\n%s\n%s", diagsText(diags), diagsText(nullDiags))
	}
	var codes []signalcenter.Code
	for _, e := range *got {
		codes = append(codes, e.Code)
	}
	if len(codes) == 0 || codes[0] != defectledger.CodeManifestMissing || (*got)[0].Cycle != 1285 || (*got)[0].Phase != "audit" {
		t.Fatalf("the wired ledger reports the registry finding: %v", codes)
	}
}

// Test 42 — ONE construction site (the seam file), the production sites use
// the wired ledger, and the Null-Object facades have no production caller.
func TestDefectLedgerSeam_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/phases/audit/defect_ledger.go"
	if offenders := auditNonTestSourcesMentioning(t, "defectledger.New(", onlySite); len(offenders) > 0 {
		t.Errorf("defectledger.New( belongs to ONE non-test file (%s); these use it too: %v", onlySite, offenders)
	}
}

func TestDefectLedgerSeam_ProductionSitesUseTheWiredLedger(t *testing.T) {
	disposition, err := os.ReadFile("disposition.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(disposition), "Via(a.hooks.defectLedger()") != 2 || !strings.Contains(string(disposition), "reconcileContinuationDefectsVia(a.hooks.defectLedger(), a.req)") || !strings.Contains(string(disposition), "emitDefectLedgerVia(a.hooks.defectLedger(), a.artifact, a.req)") {
		t.Error("disposition.go reconciles and emits through the wired ledger")
	}
	audit, err := os.ReadFile("audit.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), "inheritedDefectsPromptBlockVia(h.defectLedger(), req)") != 1 {
		t.Error("ComposePrompt tells the auditor its inherited ids through the wired ledger")
	}
}

func TestNullLedgerFacades_HaveNoProductionCaller(t *testing.T) {
	const onlySite = "internal/phases/audit/defect_ledger.go"
	for _, needle := range []string{"emitDefectLedger(", "reconcileContinuationDefects(", "readDispositions(", "dispositionPreflight("} {
		if offenders := auditNonTestSourcesMentioning(t, needle, onlySite); len(offenders) > 0 {
			t.Errorf("%q is a test-only Null-Object facade; these non-test files call it: %v", needle, offenders)
		}
	}
}

// auditNonTestSourcesMentioning lists the module's non-test Go files outside
// the defectledger leaf and the one allowed site whose source contains needle
// (the core carryover_lifecycle_test.go idiom, re-declared here: module root
// ../../.. and the leaf's own doc comments excluded).
func auditNonTestSourcesMentioning(t *testing.T, needle, allowed string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/core/defectledger/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), needle) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders
}

// Test 43 — the exact ordered {module, kind, code} stream a Classify leaves:
// a blocked continuation with an absent dispositions file and two OPEN
// inherited rows; a non-continuation FAIL; an overflowing rejection; an emit
// read fault (the disposition.go warn text once, no second code).
func TestClassify_LedgerStreamSequenceOnABlockedContinuation(t *testing.T) {
	acc, got := recordingAccessor()
	wired := hooks{ledger: wiredDefectLedger(acc)}

	_, req := continuationFixture(t, 1255, 1270, laundered[:2])
	if v, _, _ := wired.Classify(passingReport(), req, core.BridgeResponse{}); v == core.VerdictPASS {
		t.Fatal("blocked")
	}
	if want := "audit/audit.warning/AUDIT_LEDGER_DISPOSITIONS_MISSING audit/audit.warning/AUDIT_LEDGER_DEFECTS_UNACCOUNTED"; streamOf(*got) != want {
		t.Fatalf("stream = %q, want %q", streamOf(*got), want)
	}

	*got = nil
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	wired.Classify(failingReportWithDefects("a defect"), core.PhaseRequest{Cycle: 1, Workspace: ws, ProjectRoot: t.TempDir()}, core.BridgeResponse{})
	if streamOf(*got) != "" {
		t.Fatalf("a non-continuation FAIL is silent: %q", streamOf(*got))
	}

	defects := make([]string, defectLedgerMaxEntries+1)
	for i := range defects {
		defects[i] = "defect " + strconv.Itoa(i)
	}
	wired.Classify(failingReportWithDefects(defects...), core.PhaseRequest{Cycle: 2, Workspace: t.TempDir(), ProjectRoot: t.TempDir()}, core.BridgeResponse{})
	if streamOf(*got) != "audit/audit.warning/AUDIT_LEDGER_OVERFLOW" {
		t.Fatalf("an overflowing rejection: %q", streamOf(*got))
	}

	*got = nil
	ws = t.TempDir()
	writeACSVerdictShip(t, ws, 0, &yes)
	if err := os.MkdirAll(filepath.Join(ws, ledgerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	_, diags, _ := wired.Classify(failingReportWithDefects("a defect"), core.PhaseRequest{Cycle: 3, Workspace: ws, ProjectRoot: t.TempDir()}, core.BridgeResponse{})
	if streamOf(*got) != "audit/audit.warning/AUDIT_LEDGER_EMIT_FAILED" {
		t.Fatalf("an emit read fault: %q", streamOf(*got))
	}
	if n := strings.Count(diagsText(diags), "defect ledger: could not record this cycle's defects ("); n != 1 || !strings.Contains(diagsText(diags), "a later continuation will have nothing to reconcile against") {
		t.Fatalf("the host's warn text once, verbatim:\n%s", diagsText(diags))
	}
	if d := diagnosticContaining(diags, "could not record this cycle's defects"); d.Severity != "warning" || (*got)[0].Reason != d.Message {
		t.Fatalf("ONE author: the warning Classify surfaces is the event's Reason\n%s: %s\n%s", d.Severity, d.Message, (*got)[0].Reason)
	}
}

func diagnosticContaining(diags []core.Diagnostic, needle string) core.Diagnostic {
	for _, d := range diags {
		if strings.Contains(d.Message, needle) {
			return d
		}
	}
	return core.Diagnostic{}
}

// Test 49 — review fold (architecture HIGH/MEDIUM): the three artifact names
// the gate reads have ONE production spelling each — the ledger and the
// dispositions file in the leaf's schema.go, the manifest in
// continuation.go — and every other non-test file in the module names them
// through the owner (defectDispositionFile, continuation.ManifestName). A
// rename that leaves the SecondaryArtifacts hold, the self-cite denylist or
// the prompt-degrade path field behind fails here, not in a cycle. String
// LITERALS only: prose in comments and doc strings is not a spelling.
func TestArtifactNames_HaveOneProductionSpellingEach(t *testing.T) {
	owners := map[string]string{
		defectledger.LedgerFile:       "internal/core/defectledger/schema.go",
		defectledger.DispositionsFile: "internal/core/defectledger/schema.go",
		continuation.ManifestName:     "internal/continuation/continuation.go",
	}
	for name, owner := range owners {
		if offenders := nonTestStringLiteralsEqualTo(t, name); len(offenders) != 1 || offenders[0] != owner {
			t.Errorf("%q is spelled as a string literal in %v; its ONE production spelling is %s — name it through the owner", name, offenders, owner)
		}
	}
}

// nonTestStringLiteralsEqualTo lists the module's non-test Go files (path
// relative to the module root, sorted) holding a string literal equal to name.
func nonTestStringLiteralsEqualTo(t *testing.T, name string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	fset := token.NewFileSet()
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		found := false
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, uerr := strconv.Unquote(lit.Value); uerr == nil && s == name {
					found = true
				}
			}
			return !found
		})
		if found {
			hits = append(hits, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	sort.Strings(hits)
	return hits
}
