package shiperr

// signalcodes_test.go — ADR-0101 S2: every ShipErrorCode is projected, not
// copied, into the Signal Center's code space as SHIP_<code>, registered under
// the ship module with a one-line doc so docs/architecture/signal-codes.md and
// the ship.error events share ONE vocabulary.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestSignalCode_ProjectsEveryShipErrorCodeAndRegistersItWithADoc(t *testing.T) {
	codes := AllCodes()
	if len(codes) == 0 {
		t.Fatal("AllCodes lists the vocabulary")
	}
	docs := map[signalcenter.Code]string{}
	for _, d := range signalcenter.RegisteredCodes()[signalcenter.ModuleShip] {
		docs[d.Code] = d.Doc
	}
	for _, c := range codes {
		sc := SignalCode(c)
		if string(sc) != "SHIP_"+string(c) || !sc.Valid() || !sc.BelongsTo(signalcenter.ModuleShip) {
			t.Errorf("SignalCode(%s) = %s: must be the SHIP_-prefixed, valid, ship-owned projection", c, sc)
		}
		if m, ok := signalcenter.IsRegistered(sc); !ok || m != signalcenter.ModuleShip {
			t.Errorf("%s is not registered under the ship module", sc)
		}
		if docs[sc] == "" {
			t.Errorf("%s has no doc", sc)
		}
	}
	if SignalCode(CodeGitFleetRebaseNeeded) != "SHIP_GIT_FLEET_REBASE_NEEDED" {
		t.Error("the projection is the literal prefix, nothing clever")
	}
}

func TestSignalSeverity_IntegrityIsAnIncidentEverythingElseAWarn(t *testing.T) {
	for _, tc := range []struct {
		class ShipErrorClass
		want  signalcenter.Severity
	}{
		{ShipClassIntegrity, signalcenter.SeverityIncident},
		{ShipClassTransient, signalcenter.SeverityWarn},
		{ShipClassPrecondition, signalcenter.SeverityWarn},
		{ShipClassConfig, signalcenter.SeverityWarn},
		{ShipErrorClass("not-a-class"), signalcenter.SeverityWarn},
	} {
		if got := tc.class.SignalSeverity(); got != tc.want {
			t.Errorf("SignalSeverity(%q) = %s, want %s", tc.class, got, tc.want)
		}
	}
}

// Completeness by construction: every ShipErrorCode constant DECLARED in
// shiperr.go has a doc, and nothing is documented that is not declared —
// parsed from the source, so a new constant cannot slip into the vocabulary
// undocumented (and therefore unregistered and absent from signal-codes.md).
func TestCodeDocs_CoverEveryDeclaredShipErrorCode(t *testing.T) {
	t.Parallel()
	declared := declaredStringConsts(t, "ShipErrorCode")
	for s := range declared {
		if codeDocs[ShipErrorCode(s)] == "" {
			t.Errorf("%s is declared but has no doc in codeDocs", s)
		}
	}
	for c := range codeDocs {
		if !declared[string(c)] {
			t.Errorf("%s is documented but not declared", c)
		}
	}
	if len(AllCodes()) != len(declared) {
		t.Errorf("AllCodes lists %d codes, shiperr.go declares %d", len(AllCodes()), len(declared))
	}
}

// A new ShipErrorClass constant needs a row in the severity table: the
// classes are walked from source, not from a hand-kept list.
func TestSignalSeverity_EveryDeclaredClassHasARow(t *testing.T) {
	t.Parallel()
	rows := map[string]bool{string(ShipClassIntegrity): true, string(ShipClassTransient): true, string(ShipClassPrecondition): true, string(ShipClassConfig): true}
	for s := range declaredStringConsts(t, "ShipErrorClass") {
		if !rows[s] {
			t.Errorf("class %q is declared in shiperr.go but the severity table test has no row for it", s)
		}
	}
}

// declaredStringConsts parses shiperr.go and returns the string values of
// every constant declared with the named type.
func declaredStringConsts(t *testing.T, typeName string) map[string]bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "shiperr.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != typeName {
				continue
			}
			for _, v := range vs.Values {
				if lit, ok := v.(*ast.BasicLit); ok {
					s, _ := strconv.Unquote(lit.Value)
					declared[s] = true
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatalf("no %s constants parsed from shiperr.go", typeName)
	}
	return declared
}
