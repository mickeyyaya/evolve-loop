package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var notInboxItemIDReaders = map[string]string{
	"cmd/evolve/cmd_release_promote.go verifiedReleaseID":                    "a GitHub release",
	"internal/committedset/committedset.go DecisionTopN":                     "the triage decision",
	"internal/committedset/committedset.go Deferred":                         "the triage decision",
	"internal/core/task_contract.go BoundTaskIDs":                            "the triage decision",
	"internal/inboxmover/outcome.go CommittedIDs":                            "the triage decision",
	"internal/inboxmover/outcome.go DeferredIDs":                             "the triage decision",
	"internal/phases/retro/explanation_review_gate.go requireCorrectionTodo": "carryover-todos.json",
	"internal/phases/ship/consume.go validatedUnifiedMembers":                "the triage decision",
	"internal/phases/triage/unified.go applyUnified":                         "the triage decision",
}

func TestInboxItemIDReaders_ResolveTheIDThroughInboxbatchItemID(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	decoders := localIDDecoders(t, root)
	for key, callsItemID := range decoders {
		if _, documented := notInboxItemIDReaders[key]; !documented && !callsItemID {
			t.Errorf("%s decodes an item id from JSON without inboxbatch.ItemID: call it, or list the function in notInboxItemIDReaders with the document it reads", key)
		}
	}
	for key := range notInboxItemIDReaders {
		if _, found := decoders[key]; !found {
			t.Errorf("notInboxItemIDReaders lists %s, which no longer decodes a local id; remove it", key)
		}
	}
}

func localIDDecoders(t *testing.T, root string) map[string]bool {
	t.Helper()
	decoders := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && slices.Contains([]string{"testdata", "vendor", "acs"}, d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Body != nil && declaresLocalID(fn.Body) {
				decoders[filepath.ToSlash(rel)+" "+fn.Name.Name] = callsItemID(fn.Body)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return decoders
}

func declaresLocalID(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if st, ok := node.(*ast.StructType); ok {
			for _, field := range st.Fields.List {
				if field.Tag != nil && (strings.Contains(field.Tag.Value, `json:"id"`) || strings.Contains(field.Tag.Value, `json:"id,`)) {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func callsItemID(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && slices.Contains([]string{"inboxbatch.ItemID", "ItemID"}, expressionName(call.Fun)) {
			found = true
		}
		return !found
	})
	return found
}
