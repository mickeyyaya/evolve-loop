package isolation

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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

func TestRows_OnlyM1AndN9AreGreenAtX1AndEveryOpenRowNamesItsComponent(t *testing.T) {
	t.Parallel()
	for _, r := range Rows() {
		green := r.ID == "M1" || r.ID == "N9"
		if r.Open == green {
			t.Errorf("row %s Open = %v; at X1 only M1 and N9 are green", r.ID, r.Open)
		}
		if !strings.HasPrefix(r.ClosedBy, "X") || r.Check == "" || r.Owner == "" {
			t.Errorf("row %s must name its check, its owner test file and the component that turns it green: %+v", r.ID, r)
		}
		if green && r.ClosedBy != "X1" {
			t.Errorf("green row %s is closed by %s, want X1", r.ID, r.ClosedBy)
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
	if r, ok := Lookup("M2"); !ok || r.ClosedBy != "X6" || !r.Open {
		t.Fatalf("Lookup(M2) = %+v, %v; want the open row closed by X6", r, ok)
	}
	if _, ok := Lookup("N99"); ok {
		t.Fatal("Lookup found a row that the matrix does not hold")
	}
}

func TestJudge_AnOpenRowThatFailsIsTheExpectedFailureAndNamesItsComponent(t *testing.T) {
	t.Parallel()
	note, err := Judge("M2", errors.New("cli_routing.clis: claude must be listed"))
	if err != nil {
		t.Fatalf("an open row that fails is expected, got %v", err)
	}
	for _, want := range []string{"M2", "expected failure", "X6", "claude must be listed"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note %q must name %q", note, want)
		}
	}
}

func TestJudge_AnOpenRowThatPassesFailsTheTestSoTheFixIsNeverMissed(t *testing.T) {
	t.Parallel()
	_, err := Judge("N3", nil)
	if err == nil {
		t.Fatal("an open row that passes must fail the test")
	}
	for _, want := range []string{"N3", "passed", "X4", "Open"} {
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

type ownerCode struct {
	callsJudge bool
	literals   map[string]bool
}

func parseOwner(path string) (ownerCode, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return ownerCode{}, err
	}
	code := ownerCode{literals: map[string]bool{}}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "isolation" && n.Sel.Name == "Judge" {
				code.callsJudge = true
			}
		case *ast.BasicLit:
			if s, err := strconv.Unquote(n.Value); n.Kind == token.STRING && err == nil {
				code.literals[s] = true
			}
		}
		return true
	})
	return code, nil
}

func TestParseOwner_SeesCodeNotComments(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "owner_test.go")
	src := "package p\n\n// isolation.Judge(\"N5\")\nvar rows = []string{\"N3\"}\nvar _ = isolation.Judge\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	code, err := parseOwner(path)
	if err != nil || !code.callsJudge || !code.literals["N3"] || code.literals["N5"] {
		t.Fatalf("code = %+v, err = %v; want Judge and N3 from code, not N5 from a comment", code, err)
	}
	if err := os.WriteFile(path, []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseOwner(path); err == nil {
		t.Fatal("an owner file that does not parse must fail")
	}
}

func TestRows_EachOwnerTestJudgesItsRowSoNoRowIsSilentlyDropped(t *testing.T) {
	t.Parallel()
	owners := map[string]ownerCode{}
	for _, r := range Rows() {
		code, ok := owners[r.Owner]
		if !ok {
			var err error
			if code, err = parseOwner(filepath.Join("..", "..", r.Owner)); err != nil {
				t.Fatalf("row %s names owner %s: %v", r.ID, r.Owner, err)
			}
			owners[r.Owner] = code
		}
		if !code.callsJudge || !code.literals[r.ID] {
			t.Errorf("row %s: the code of %s must run the row through isolation.Judge with the id %q", r.ID, r.Owner, r.ID)
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
