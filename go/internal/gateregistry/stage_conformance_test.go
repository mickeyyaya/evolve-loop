package gateregistry

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	ruleNoEntry       = "no-registry-entry"
	ruleStringTyped   = "string-typed-stage"
	ruleNoSignal      = "no-signal"
	ruleUnregistered  = "signal-unregistered"
	ruleNoEmitSite    = "no-emit-site"
	ruleNoWouldFields = "no-would-fields"
	ruleFieldMissing  = "would-field-missing"
	baselineCeiling   = 88
	moduleRoot        = "../.."
)

type violation struct {
	Key  string `json:"key"`
	Rule string `json:"rule"`
}

type schemaStage struct {
	path     string
	typeName string
}

func (v violation) String() string { return v.Key + " " + v.Rule }

func schemaStages() []schemaStage {
	var out []schemaStage
	walkSchema(reflect.TypeOf(policy.Policy{}), "", map[reflect.Type]bool{}, &out)
	return out
}

func walkSchema(t reflect.Type, prefix string, active map[reflect.Type]bool, out *[]schemaStage) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || active[t] {
		return
	}
	active[t] = true
	defer delete(active, t)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := prefix + name
		if strings.HasSuffix(name, "stage") {
			*out = append(*out, schemaStage{path: path, typeName: field.Type.String()})
		}
		walkSchema(field.Type, path+".", active, out)
	}
}

func entryByKey(entries []gatecontract.Entry) map[string]gatecontract.Entry {
	byKey := map[string]gatecontract.Entry{}
	for _, e := range entries {
		if e.Shadow != nil {
			byKey[e.Shadow.PolicyKey] = e
		}
	}
	return byKey
}

func emitSiteProblem(root string, site gatecontract.EmitSite, signal signalcenter.Code, fields []string) string {
	if site.Path == "" || strings.HasSuffix(site.Path, "_test.go") {
		return "emit site " + site.Path + " is not a non-test file"
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, site.Path), nil, 0)
	if err != nil {
		return "emit site " + site.Path + " does not parse: " + err.Error()
	}
	body := funcBody(file, site.Func)
	if body == nil {
		return "emit site " + site.Path + " has no function " + site.Func
	}
	if !calledInDir(filepath.Dir(filepath.Join(root, site.Path)), site.Func) {
		return "emit site " + site.Func + " is never called"
	}
	names, literals := bodyWords(body)
	if !names[site.Ref] && !literals[string(signal)] {
		return "emit site " + site.Func + " does not use " + site.Ref
	}
	for _, field := range fields {
		if !literals[field] {
			return "emit site " + site.Func + " lacks the field " + field
		}
	}
	return ""
}

func funcBody(file *ast.File, name string) *ast.BlockStmt {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn.Body
		}
	}
	return nil
}

func calledInDir(dir, name string) bool {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err == nil && callsFunc(file, name) {
			return true
		}
	}
	return false
}

func callsFunc(file *ast.File, name string) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			found = found || fn.Name == name
		case *ast.SelectorExpr:
			found = found || fn.Sel.Name == name
		}
		return !found
	})
	return found
}

func bodyWords(body *ast.BlockStmt) (map[string]bool, map[string]bool) {
	names, literals := map[string]bool{}, map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			names[node.Name] = true
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				if text, err := strconv.Unquote(node.Value); err == nil {
					literals[text] = true
				}
			}
		}
		return true
	})
	return names, literals
}

func signalViolations(key string, e gatecontract.Entry) []violation {
	shadow := e.Shadow
	if shadow.Signal == "" {
		return []violation{{key, ruleNoSignal}}
	}
	var out []violation
	if _, ok := signalcenter.IsRegistered(shadow.Signal); !ok {
		out = append(out, violation{key, ruleUnregistered})
	}
	if len(shadow.Fields) == 0 {
		out = append(out, violation{key, ruleNoWouldFields})
	}
	if len(shadow.EmitSites) == 0 {
		return append(out, violation{key, ruleNoEmitSite})
	}
	for _, site := range shadow.EmitSites {
		problem := emitSiteProblem(moduleRoot, site, shadow.Signal, nil)
		if problem != "" {
			out = append(out, violation{key, ruleNoEmitSite})
		} else if emitSiteProblem(moduleRoot, site, shadow.Signal, shadow.Fields) != "" {
			out = append(out, violation{key, ruleFieldMissing})
		}
	}
	return out
}

func liveViolations(stages []schemaStage, entries []gatecontract.Entry) []violation {
	byKey := entryByKey(entries)
	var out []violation
	for _, stage := range stages {
		if stage.typeName == "string" {
			out = append(out, violation{stage.path, ruleStringTyped})
		}
		entry, ok := byKey[stage.path]
		if !ok {
			out = append(out, violation{stage.path, ruleNoEntry})
			continue
		}
		out = append(out, signalViolations(stage.path, entry)...)
	}
	return out
}

func loadBaseline(t *testing.T) []violation {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "stage_violations.json"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var doc struct {
		Violations []violation `json:"violations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	return doc.Violations
}

func newViolations(actual, baseline []violation, rules ...string) []string {
	var out []string
	for _, v := range actual {
		if slices.Contains(rules, v.Rule) && !slices.Contains(baseline, v) {
			out = append(out, v.String())
		}
	}
	return out
}

func staleBaseline(actual, baseline []violation) []string {
	var out []string
	for _, v := range baseline {
		if !slices.Contains(actual, v) {
			out = append(out, v.String())
		}
	}
	return out
}

func assertNoNewViolations(t *testing.T, rules ...string) {
	t.Helper()
	actual := liveViolations(schemaStages(), StageEntries())
	if fresh := newViolations(actual, loadBaseline(t), rules...); len(fresh) > 0 {
		t.Errorf("stage keys break the shadow contract and are not in testdata/stage_violations.json:\n%s", strings.Join(fresh, "\n"))
	}
}

func TestEveryStageKeyHasRegistryEntry(t *testing.T) {
	t.Parallel()
	assertNoNewViolations(t, ruleNoEntry)
	known := map[string]bool{}
	for _, stage := range schemaStages() {
		known[stage.path] = true
	}
	for _, e := range StageEntries() {
		if !known[e.Shadow.PolicyKey] {
			t.Errorf("entry %s names %s, which is not a stage key of the policy schema", e.Name, e.Shadow.PolicyKey)
		}
	}
}

func TestNoStringTypedStageField(t *testing.T) {
	t.Parallel()
	assertNoNewViolations(t, ruleStringTyped)
}

func TestEverySignalCodeHasNonTestEmitter(t *testing.T) {
	t.Parallel()
	assertNoNewViolations(t, ruleNoSignal, ruleUnregistered, ruleNoEmitSite)
}

func TestShadowSignalCarriesWouldFields(t *testing.T) {
	t.Parallel()
	assertNoNewViolations(t, ruleNoWouldFields, ruleFieldMissing)
}

func TestViolationBaselineOnlyShrinks(t *testing.T) {
	t.Parallel()
	baseline := loadBaseline(t)
	for _, problem := range baselineProblems(baseline, baselineCeiling) {
		t.Error(problem)
	}
	actual := liveViolations(schemaStages(), StageEntries())
	if stale := staleBaseline(actual, baseline); len(stale) > 0 {
		t.Errorf("the baseline lists violations that no longer exist; remove them and lower baselineCeiling:\n%s", strings.Join(stale, "\n"))
	}
}

func baselineProblems(baseline []violation, ceiling int) []string {
	var out []string
	if len(baseline) > ceiling {
		out = append(out, "the baseline holds more violations than the ceiling; it may only shrink")
	}
	if len(baseline) < ceiling {
		out = append(out, "the baseline holds fewer violations than the ceiling; lower baselineCeiling in the same change")
	}
	seen := map[violation]bool{}
	for _, v := range baseline {
		if seen[v] {
			out = append(out, "the baseline lists "+v.String()+" twice")
		}
		seen[v] = true
	}
	return out
}

func TestBaselineProblems_FlagsAnOverCeilingAnUnderCeilingAndADuplicate(t *testing.T) {
	t.Parallel()
	a, b := violation{"a.stage", ruleNoEntry}, violation{"b.stage", ruleNoEntry}
	if got := baselineProblems([]violation{a, b}, 2); len(got) != 0 {
		t.Errorf("exact baseline problems = %q", got)
	}
	if got := baselineProblems([]violation{a, b}, 1); len(got) != 1 || !strings.Contains(got[0], "more violations") {
		t.Errorf("over-ceiling problems = %q", got)
	}
	if got := baselineProblems([]violation{a}, 2); len(got) != 1 || !strings.Contains(got[0], "fewer violations") {
		t.Errorf("under-ceiling problems = %q", got)
	}
	if got := baselineProblems([]violation{a, a}, 2); len(got) != 1 || !strings.Contains(got[0], "twice") {
		t.Errorf("duplicate problems = %q", got)
	}
}

func TestBaselineCeiling_EqualsTheBaselineFile(t *testing.T) {
	t.Parallel()
	if n := len(loadBaseline(t)); n != baselineCeiling {
		t.Errorf("baseline holds %d pairs, baselineCeiling is %d", n, baselineCeiling)
	}
}

func TestStageEntries_AreWellFormed(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, e := range StageEntries() {
		problems := stageEntryProblems(e)
		if seen[e.Name] {
			problems = append(problems, "duplicate name")
		}
		seen[e.Name] = true
		if len(problems) > 0 {
			t.Errorf("%s: %s", e.Name, strings.Join(problems, "; "))
		}
	}
}

func stageEntryProblems(e gatecontract.Entry) []string {
	var problems []string
	if e.Kind != gatecontract.KindStageFeature {
		problems = append(problems, "kind "+e.Kind)
	}
	if e.Shadow == nil || e.Shadow.PolicyKey != e.Name {
		return append(problems, "no shadow descriptor named by the policy key")
	}
	if e.Stage != "" {
		problems = append(problems, "a seed stage beside the policy key")
	}
	if e.Shadow.PolicySubtree == "" || e.Shadow.PolicySubtree != subtreeOf(e.Shadow.PolicyKey) {
		problems = append(problems, "policy subtree "+e.Shadow.PolicySubtree)
	}
	for _, pkg := range e.Shadow.Closure {
		if info, err := os.Stat(filepath.Join(moduleRoot, pkg)); err != nil || !info.IsDir() {
			problems = append(problems, "closure package "+pkg+" is not a directory")
		}
	}
	return problems
}

func TestStageEntryProblems_FlagsEachMalformedShape(t *testing.T) {
	t.Parallel()
	bad := gatecontract.Entry{Name: "a.stage", Kind: gatecontract.KindScanner, Stage: gatecontract.StageShadow,
		Shadow: &gatecontract.Shadow{PolicyKey: "a.stage", PolicySubtree: "zzz", Closure: []string{"internal/nope"}}}
	got := strings.Join(stageEntryProblems(bad), "|")
	for _, want := range []string{"kind scanner", "a seed stage", "policy subtree zzz", "closure package internal/nope"} {
		if !strings.Contains(got, want) {
			t.Errorf("problems = %q, want %q", got, want)
		}
	}
	if got := stageEntryProblems(gatecontract.Entry{Name: "x", Kind: gatecontract.KindStageFeature}); len(got) != 1 {
		t.Errorf("shadowless entry problems = %q", got)
	}
}

func TestNewViolations_FailsForAKeyOutsideTheBaseline(t *testing.T) {
	t.Parallel()
	actual := []violation{{"a.stage", ruleNoEntry}, {"b.stage", ruleNoEntry}, {"b.stage", ruleNoSignal}}
	baseline := []violation{{"a.stage", ruleNoEntry}, {"gone.stage", ruleNoEntry}}
	if got := newViolations(actual, baseline, ruleNoEntry); !slices.Equal(got, []string{"b.stage no-registry-entry"}) {
		t.Errorf("newViolations = %q", got)
	}
	if got := staleBaseline(actual, baseline); !slices.Equal(got, []string{"gone.stage no-registry-entry"}) {
		t.Errorf("staleBaseline = %q", got)
	}
}

func TestLiveViolations_NamesAKeyWithNoEntryAndAPlainStringField(t *testing.T) {
	t.Parallel()
	got := liveViolations([]schemaStage{{"x.stage", "string"}, {"y.stage", "gatecontract.Stage"}}, nil)
	want := []violation{{"x.stage", ruleStringTyped}, {"x.stage", ruleNoEntry}, {"y.stage", ruleNoEntry}}
	if !slices.Equal(got, want) {
		t.Errorf("liveViolations = %v, want %v", got, want)
	}
}

func writeGo(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEmitSiteProblem_FlagsATestFileAMissingFuncAMissingRefAndAMissingField(t *testing.T) {
	t.Parallel()
	dir := writeGo(t, "emit.go", "package p\nconst Code = \"X_SHADOW\"\nfunc emit() { _ = map[string]string{\"a\": Code} }\nfunc broken(")
	ok := writeGo(t, "emit.go", "package p\nconst Code = \"X_SHADOW\"\nfunc emit() { _ = map[string]string{\"a\": Code} }\nfunc lit() { _ = \"X_SHADOW\" }\nfunc use() { emit(); x.lit() }")
	cases := map[string]struct {
		root, path, fn, ref string
		fields              []string
		want                string
	}{
		"test file":    {ok, "emit_test.go", "emit", "Code", nil, "not a non-test file"},
		"empty path":   {ok, "", "emit", "Code", nil, "not a non-test file"},
		"no parse":     {dir, "emit.go", "emit", "Code", nil, "does not parse"},
		"no func":      {ok, "emit.go", "gone", "Code", nil, "has no function gone"},
		"never called": {ok, "emit.go", "use", "Code", nil, "is never called"},
		"no ref":       {ok, "emit.go", "emit", "Other", nil, "does not use Other"},
		"no field":     {ok, "emit.go", "emit", "Code", []string{"b"}, "lacks the field b"},
		"ref only":     {ok, "emit.go", "emit", "Code", []string{"a"}, ""},
		"literal ref":  {ok, "emit.go", "lit", "Other", nil, ""},
	}
	for name, c := range cases {
		got := emitSiteProblem(c.root, gatecontract.EmitSite{Path: c.path, Func: c.fn, Ref: c.ref}, "X_SHADOW", c.fields)
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: problem = %q, want %q", name, got, c.want)
		}
	}
}

func TestCalledInDir_IgnoresTestFilesAndDeclarations(t *testing.T) {
	t.Parallel()
	dir := writeGo(t, "a.go", "package p\nfunc dead() {}\nfunc live() {}\nfunc use() { live(); s.sel() }")
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package p\nfunc T() { dead() }"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{"dead": false, "live": true, "sel": true, "absent": false}
	for name, want := range cases {
		if got := calledInDir(dir, name); got != want {
			t.Errorf("calledInDir(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestSignalViolations_NameEachBrokenShape(t *testing.T) {
	t.Parallel()
	entry := func(shadow gatecontract.Shadow) gatecontract.Entry { return gatecontract.Entry{Shadow: &shadow} }
	real := gatecontract.EmitSite{Path: "internal/phases/ship/held_proposal_gate.go", Func: "emitShadow", Ref: "CodeHoldShadow"}
	cases := map[string]struct {
		shadow gatecontract.Shadow
		want   []violation
	}{
		"no signal":     {gatecontract.Shadow{}, []violation{{"k", ruleNoSignal}}},
		"unregistered":  {gatecontract.Shadow{Signal: "NOPE_NEVER"}, []violation{{"k", ruleUnregistered}, {"k", ruleNoWouldFields}, {"k", ruleNoEmitSite}}},
		"missing field": {gatecontract.Shadow{Signal: "SHIP_HOLD_SHADOW", Fields: []string{"nope"}, EmitSites: []gatecontract.EmitSite{real}}, []violation{{"k", ruleFieldMissing}}},
		"bad site":      {gatecontract.Shadow{Signal: "SHIP_HOLD_SHADOW", Fields: []string{"tree"}, EmitSites: []gatecontract.EmitSite{{Path: "x_test.go"}}}, []violation{{"k", ruleNoEmitSite}}},
		"conforming":    {gatecontract.Shadow{Signal: "SHIP_HOLD_SHADOW", Fields: []string{"tree"}, EmitSites: []gatecontract.EmitSite{real}}, nil},
	}
	for name, c := range cases {
		if got := signalViolations("k", entry(c.shadow)); !slices.Equal(got, c.want) {
			t.Errorf("%s: violations = %v, want %v", name, got, c.want)
		}
	}
}

func TestWalkSchema_FindsNestedStageKeysAndSkipsCycles(t *testing.T) {
	t.Parallel()
	type node struct {
		Stage string `json:"stage"`
		Next  *node  `json:"next"`
		Skip  string `json:"-"`
		Bare  string
	}
	var got []schemaStage
	walkSchema(reflect.TypeOf(node{}), "", map[reflect.Type]bool{}, &got)
	if want := []schemaStage{{"stage", "string"}}; !slices.Equal(got, want) {
		t.Errorf("walk = %v, want %v", got, want)
	}
	if n := len(schemaStages()); n < 41 {
		t.Errorf("the policy schema walk found %d stage keys, want at least 41", n)
	}
}

func TestSubtreeOf_DropsTheLastSegmentAndKeepsABareKey(t *testing.T) {
	t.Parallel()
	if got := subtreeOf("a.b.stage"); got != "a.b" {
		t.Errorf("subtreeOf = %q", got)
	}
	if got := subtreeOf("bare"); got != "bare" {
		t.Errorf("subtreeOf bare = %q", got)
	}
}

type seedRow struct {
	signal  signalcenter.Code
	site    gatecontract.EmitSite
	fields  []string
	closure []string
}

func TestStageSeeds_PinTheDescriptorOfEachPopulatedRow(t *testing.T) {
	t.Parallel()
	want := map[string]seedRow{
		"workflow.retry_ladder.stage": {"LADDER_DECISION", gatecontract.EmitSite{Path: "internal/core/retry_ladder.go", Func: "emitLadderDecision", Ref: "CodeLadderDecision"}, []string{"next", "action", "fingerprint", "tier"}, []string{"internal/retryladder"}},
		"workflow.cycle_spend.stage":  {"CYCLE_SPEND_SHADOW", gatecontract.EmitSite{}, nil, []string{"internal/cyclespend"}},
		"parallel_evaluate.stage":     {"PARALLEL_EVALUATE_SHADOW", gatecontract.EmitSite{}, nil, nil},
		"regression_tia.stage":        {"TIA_SHADOW", gatecontract.EmitSite{}, nil, []string{"internal/regressiontia"}},
		"held_proposals.stage":        {"SHIP_HOLD_SHADOW", gatecontract.EmitSite{Path: "internal/phases/ship/held_proposal_gate.go", Func: "emitShadow", Ref: "CodeHoldShadow"}, []string{"cycle", "tree", "reasons"}, []string{"internal/heldproposal"}},
		"audit_cascade.stage":         {"AUDIT_CASCADE_SHADOW", gatecontract.EmitSite{}, nil, []string{"internal/auditcascade"}},
	}
	for _, e := range StageEntries() {
		w := want[e.Name]
		s := e.Shadow
		if s.Signal != w.signal || !slices.Equal(s.Fields, w.fields) || !slices.Equal(s.Closure, w.closure) {
			t.Errorf("%s: signal %q fields %q closure %q, want %+v", e.Name, s.Signal, s.Fields, s.Closure, w)
		}
		if w.site == (gatecontract.EmitSite{}) && len(s.EmitSites) != 0 {
			t.Errorf("%s: emit sites %v, want none", e.Name, s.EmitSites)
		}
		if w.site != (gatecontract.EmitSite{}) && !slices.Equal(s.EmitSites, []gatecontract.EmitSite{w.site}) {
			t.Errorf("%s: emit sites %v, want %v", e.Name, s.EmitSites, w.site)
		}
	}
}

func TestStageEntries_ReturnSlicesThatDoNotAliasTheSeedTable(t *testing.T) {
	t.Parallel()
	for _, e := range StageEntries() {
		if len(e.Shadow.Closure) > 0 {
			e.Shadow.Closure[0] = "mutated"
		}
		if len(e.Shadow.Fields) > 0 {
			e.Shadow.Fields[0] = "mutated"
		}
	}
	for _, e := range StageEntries() {
		if slices.Contains(e.Shadow.Closure, "mutated") || slices.Contains(e.Shadow.Fields, "mutated") {
			t.Fatalf("%s aliases the seed table", e.Name)
		}
	}
}
