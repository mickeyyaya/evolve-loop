package isolation

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestRows_AreTheNineteenRowsOfSpec14InOrder(t *testing.T) {
	t.Parallel()
	var ids []string
	for _, r := range Rows() {
		ids = append(ids, r.ID)
	}
	want := []string{"M1", "M2", "M3", "M4", "M5", "N1", "N2", "N3", "N4", "N5", "N6", "N7", "N8", "N9", "N10", "N11", "N12", "N13", "N14"}
	if !slices.Equal(ids, want) {
		t.Fatalf("matrix rows = %v, want %v", ids, want)
	}
}

func TestRows_OnlyTheRowsOfLandedComponentsAreGreenAndEveryOpenRowNamesItsComponent(t *testing.T) {
	t.Parallel()
	greenBy := map[string]string{"M1": "X1", "N9": "X1", "N13": "X6", "N1": "X2", "N2": "X3", "N3": "X4", "N4": "X4", "M2": "X1b", "M3": "X1b", "M4": "X1b", "N12": "X8", "N5": "X5", "N6": "X5", "N7": "X5", "N8": "X5", "N10": "X7", "N11": "X7"}
	for _, r := range Rows() {
		closer, green := greenBy[r.ID]
		if r.Open == green {
			t.Errorf("row %s Open = %v; only %v are green after X1, X1b, X2, X3, X4, X5, X6, X7 and X8", r.ID, r.Open, greenBy)
		}
		if !strings.HasPrefix(r.ClosedBy, "X") || r.Check == "" || r.Owner == "" {
			t.Errorf("row %s must name its check, its owner test file and the component that turns it green: %+v", r.ID, r)
		}
		if green && r.ClosedBy != closer {
			t.Errorf("green row %s is closed by %s, want %s", r.ID, r.ClosedBy, closer)
		}
	}
}

func TestRows_ReturnsACopy(t *testing.T) {
	t.Parallel()
	Rows()[0].Open = true
	if r, _ := Lookup("M1"); r.Open {
		t.Fatal("a caller changed the matrix through the slice that Rows returned")
	}
}

func TestLookup_FindsARowAndRefusesAnUnknownID(t *testing.T) {
	t.Parallel()
	if r, ok := Lookup("M5"); !ok || r.ClosedBy != "X9" || !r.Open {
		t.Fatalf("Lookup(M5) = %+v, %v; want the open row closed by X9", r, ok)
	}
	if _, ok := Lookup("N99"); ok {
		t.Fatal("Lookup found a row that the matrix does not hold")
	}
}

func TestJudge_AnOpenRowThatFailsIsTheExpectedFailureAndNamesItsComponent(t *testing.T) {
	t.Parallel()
	note, err := Judge("M5", errors.New("cliroute: cli_routing.clis: ollama-tmux cannot use tools, so it cannot run a phase"))
	if err != nil {
		t.Fatalf("an open row that fails is expected, got %v", err)
	}
	for _, want := range []string{"M5", "expected failure", "X9", "ollama-tmux cannot use tools"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note %q must name %q", note, want)
		}
	}
}

func TestJudge_AnOpenRowThatFailsForAnotherReasonIsInconclusive(t *testing.T) {
	t.Parallel()
	_, err := Judge("M5", errors.New("evolve cycle run: timed out"))
	if err == nil {
		t.Fatal("an open row that fails for a reason other than its gap must fail the test")
	}
	for _, want := range []string{"M5", "inconclusive", "ollama-tmux cannot use tools", "timed out"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error %q must name %q", err, want)
		}
	}
}

func TestRows_EveryOpenRowPinsTheCauseOfItsGapAndNoGreenRowDoes(t *testing.T) {
	t.Parallel()
	for _, r := range Rows() {
		if r.Open == (r.Cause == "") {
			t.Errorf("row %s: Open=%v Cause=%q; an open row must name the text of its gap, a green row none", r.ID, r.Open, r.Cause)
		}
	}
}

func TestJudge_AnOpenRowThatPassesFailsTheTestSoTheFixIsNeverMissed(t *testing.T) {
	t.Parallel()
	_, err := Judge("N14", nil)
	if err == nil {
		t.Fatal("an open row that passes must fail the test")
	}
	for _, want := range []string{"N14", "passed", "X9", "Open"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error %q must name %q", err, want)
		}
	}
}

func TestJudge_AGreenRowMustPass(t *testing.T) {
	t.Parallel()
	if note, err := Judge("M1", nil); err != nil || !strings.Contains(note, "M1") {
		t.Fatalf("a green row that passes: note=%q err=%v", note, err)
	}
	cause := errors.New("ledger missing role ship")
	_, err := Judge("N9", cause)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "N9") {
		t.Fatalf("a green row that fails must fail with its cause, got %v", err)
	}
}

func TestJudge_AnUnknownRowFails(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{nil, errors.New("x")} {
		if _, err := Judge("Z1", failure); err == nil || !strings.Contains(err.Error(), "Z1") {
			t.Fatalf("an unknown row must fail, got %v", err)
		}
	}
}

func judgedIDs(path string) (map[string]bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	ids, fields := map[string]bool{}, map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isJudge(call.Fun) || len(call.Args) == 0 {
				return true
			}
			switch arg := call.Args[0].(type) {
			case *ast.BasicLit:
				if id, err := strconv.Unquote(arg.Value); err == nil {
					ids[id] = true
				}
			case *ast.SelectorExpr:
				fields[arg.Sel.Name] = true
			}
			return true
		})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, keyOK := kv.Key.(*ast.Ident)
		lit, litOK := kv.Value.(*ast.BasicLit)
		if keyOK && litOK && fields[key.Name] {
			if id, err := strconv.Unquote(lit.Value); err == nil {
				ids[id] = true
			}
		}
		return true
	})
	return ids, nil
}

func isJudge(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "isolation" && sel.Sel.Name == "Judge"
}

func TestJudgedIDs_CountsOnlyAJudgeCallInATestWithTheIDOrATableFieldItReads(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "owner_test.go")
	src := `package p

// isolation.Judge("N5", nil)
var rows = []row{{id: "N3", name: "N7"}}
var stray = "N8"

func TestRows(t *testing.T) {
	for _, r := range rows {
		isolation.Judge(r.id, nil)
	}
	isolation.Judge("N1", nil)
	other.Judge("N2", nil)
	isolation.Lookup("N4")
}

func helper() { isolation.Judge("N6", nil) }
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ids, err := judgedIDs(path)
	if err != nil {
		t.Fatal(err)
	}
	got := slices.Sorted(maps.Keys(ids))
	if !slices.Equal(got, []string{"N1", "N3"}) {
		t.Fatalf("judged ids = %v, want [N1 N3]: a comment, a stray literal, another field, another package, Lookup and a non-test function do not count", got)
	}
	if err := os.WriteFile(path, []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := judgedIDs(path); err == nil {
		t.Fatal("an owner file that does not parse must fail")
	}
}

func TestRows_EachOwnerTestJudgesItsRowSoNoRowIsSilentlyDropped(t *testing.T) {
	t.Parallel()
	owners := map[string]map[string]bool{}
	for _, r := range Rows() {
		ids, ok := owners[r.Owner]
		if !ok {
			var err error
			if ids, err = judgedIDs(filepath.Join("..", "..", r.Owner)); err != nil {
				t.Fatalf("row %s names owner %s: %v", r.ID, r.Owner, err)
			}
			owners[r.Owner] = ids
		}
		if !ids[r.ID] {
			t.Errorf("row %s: a test function of %s must call isolation.Judge with %q, or with a table field that holds it", r.ID, r.Owner, r.ID)
		}
	}
}

func TestRow_N9IsTheNetworkLimitWithNoInvariantOwnedByTheSandboxContract(t *testing.T) {
	t.Parallel()
	want := Row{ID: "N9", Check: "the network row records today's behavior (allowed)", Invariant: "none", ClosedBy: "X1", Owner: ownerContract}
	if got, _ := Lookup("N9"); got != want {
		t.Fatalf("N9 = %+v, want %+v", got, want)
	}
}
