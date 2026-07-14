package apicover

import (
	"go/ast"
	"testing"
)

func comments(lines ...string) *ast.CommentGroup {
	g := &ast.CommentGroup{}
	for _, l := range lines {
		g.List = append(g.List, &ast.Comment{Text: l})
	}
	return g
}

func TestIgnoreDirective_MarksSymbolWithReason(t *testing.T) {
	doc := comments("// SomeFunc does a thing.", "//apicover:ignore reason=legacy shim, removed in Phase 6")
	ignored, reason, err := parseIgnore(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ignored {
		t.Fatal("expected ignored=true")
	}
	if reason != "legacy shim, removed in Phase 6" {
		t.Errorf("reason = %q, want %q", reason, "legacy shim, removed in Phase 6")
	}
}

func TestIgnoreDirective_EmptyReasonIsError(t *testing.T) {
	for _, line := range []string{"//apicover:ignore", "//apicover:ignore reason=", "//apicover:ignore reason=   "} {
		_, _, err := parseIgnore(comments(line))
		if err == nil {
			t.Errorf("line %q: expected error for missing/empty reason, got nil", line)
		}
	}
}

func TestIgnoreDirective_NoDirectiveReturnsFalse(t *testing.T) {
	ignored, _, err := parseIgnore(comments("// just an ordinary doc comment"))
	if err != nil || ignored {
		t.Fatalf("got (ignored=%v, err=%v), want (false, nil)", ignored, err)
	}
	// nil doc must be safe too.
	if ig, _, err := parseIgnore(nil); ig || err != nil {
		t.Fatalf("nil doc: got (ignored=%v, err=%v), want (false, nil)", ig, err)
	}
}

func TestRequireDoc_FlagsMissingGodoc(t *testing.T) {
	syms := []Symbol{
		{Name: "Documented", HasDoc: true},
		{Name: "Undocumented", HasDoc: false},
		{Name: "IgnoredUndoc", HasDoc: false, Ignored: true},
	}
	missing := MissingDoc(syms)
	if containsName(missing, "Documented") {
		t.Error("a documented symbol must not be flagged")
	}
	if !containsName(missing, "Undocumented") {
		t.Error("an undocumented exported symbol must be flagged")
	}
	if containsName(missing, "IgnoredUndoc") {
		t.Error("an //apicover:ignore symbol must not be flagged for missing doc")
	}
}
