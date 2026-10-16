package apicover

import (
	"context"
	"go/ast"
	"os"
	"path/filepath"
	"testing"
)

func TestEnumerate_UnreadableGoFileIsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "missing-target.go"), filepath.Join(dir, "dangling.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	syms, err := Enumerate(context.Background(), dir)

	if err == nil {
		t.Fatalf("a Go file that cannot be matched must fail the walk, got %v", syms)
	}
}

func TestRecvTypeName_UnwrapsEveryReceiverShape(t *testing.T) {
	t.Parallel()
	ident := ast.NewIdent("Box")
	recv := func(expr ast.Expr) *ast.FieldList {
		return &ast.FieldList{List: []*ast.Field{{Type: expr}}}
	}
	cases := []struct {
		name string
		recv *ast.FieldList
		want string
	}{
		{"nil list", nil, ""},
		{"empty list", &ast.FieldList{}, ""},
		{"value receiver", recv(ident), "Box"},
		{"pointer receiver", recv(&ast.StarExpr{X: ident}), "Box"},
		{"generic one parameter", recv(&ast.IndexExpr{X: ident, Index: ast.NewIdent("T")}), "Box"},
		{"pointer generic one parameter", recv(&ast.StarExpr{X: &ast.IndexExpr{X: ident, Index: ast.NewIdent("T")}}), "Box"},
		{"generic two parameters", recv(&ast.IndexListExpr{X: ident, Indices: []ast.Expr{ast.NewIdent("K"), ast.NewIdent("V")}}), "Box"},
		{"index over a selector", recv(&ast.IndexExpr{X: &ast.SelectorExpr{X: ident, Sel: ast.NewIdent("T")}, Index: ast.NewIdent("P")}), ""},
		{"index list over a selector", recv(&ast.IndexListExpr{X: &ast.SelectorExpr{X: ident, Sel: ast.NewIdent("T")}}), ""},
		{"unsupported expression", recv(&ast.ArrayType{Elt: ident}), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := recvTypeName(tc.recv); got != tc.want {
				t.Errorf("recvTypeName=%q want %q", got, tc.want)
			}
		})
	}
}
