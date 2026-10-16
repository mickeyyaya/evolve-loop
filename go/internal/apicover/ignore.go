package apicover

import (
	"fmt"
	"go/ast"
	"strings"
)

const ignoreMarker = "apicover:ignore"

func parseIgnore(doc *ast.CommentGroup) (ignored bool, reason string, err error) {
	if doc == nil {
		return false, "", nil
	}
	for _, c := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if !strings.HasPrefix(text, ignoreMarker) {
			continue
		}
		rest := strings.TrimSpace(text[len(ignoreMarker):])
		if idx := strings.Index(rest, "reason="); idx >= 0 {
			reason = strings.TrimSpace(rest[idx+len("reason="):])
		}
		if reason == "" {
			return false, "", fmt.Errorf("//apicover:ignore requires a non-empty reason= (got %q)", strings.TrimSpace(c.Text))
		}
		return true, reason, nil
	}
	return false, "", nil
}

func MissingDoc(syms []Symbol) []Symbol {
	var out []Symbol
	for _, s := range syms {
		if s.Ignored {
			continue
		}
		if !s.HasDoc {
			out = append(out, s)
		}
	}
	return out
}
