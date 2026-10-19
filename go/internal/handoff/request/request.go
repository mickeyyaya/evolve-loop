package request

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	PairScoutTriage = "scout-triage"
	PairTriageTDD   = "triage-tdd"
	PairTDDBuild    = "tdd-build"
)

var (
	ErrNoCommittedSet = errors.New("no committed set")
	ErrUnresolvedItem = errors.New("committed item has no readable inbox record")
	ErrNoPredicates   = errors.New("no predicate found")
)

type PathResolver func(id string) string

func ScoutTriage(slugs []string) handoff.Request {
	req := handoff.Request{Pair: PairScoutTriage}
	seen := make(map[string]bool, len(slugs))
	for _, raw := range slugs {
		slug := strings.TrimSpace(raw)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		req.Asks = append(req.Asks, handoff.Ask{ID: slug, Text: slug, Source: "scout:selected-tasks", EvidenceKind: "disposition", Required: true})
	}
	return req
}

func TriageTDD(workspace string, resolve PathResolver) (handoff.Request, error) {
	ids, decided := committedset.Committed(workspace)
	if !decided || len(ids) == 0 {
		return handoff.Request{}, ErrNoCommittedSet
	}
	items := make([]inboxbatch.Item, 0, len(ids))
	for _, id := range ids {
		path := resolve(id)
		if path == "" {
			return handoff.Request{}, fmt.Errorf("%w: %s", ErrUnresolvedItem, id)
		}
		item, _, err := inboxbatch.LoadFile(path)
		if err != nil {
			return handoff.Request{}, fmt.Errorf("%w: %s: %v", ErrUnresolvedItem, id, err)
		}
		items = append(items, item)
	}
	return TriageTDDFromItems(items), nil
}

func TriageTDDFromItems(items []inboxbatch.Item) handoff.Request {
	req := handoff.Request{Pair: PairTriageTDD}
	for _, item := range items {
		for i, entry := range item.Acceptance {
			text := strings.TrimSpace(entry)
			if text == "" {
				continue
			}
			req.Asks = append(req.Asks, handoff.Ask{
				ID:           fmt.Sprintf("%s#%d", item.ID, i+1),
				Text:         text,
				Source:       fmt.Sprintf("inbox:%s#acceptance/%d", item.ID, i+1),
				EvidenceKind: "predicate",
				Required:     true,
			})
		}
	}
	return req
}

func TDDBuild(dir string) (handoff.Request, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return handoff.Request{}, err
	}
	pkg := filepath.Base(dir)
	req := handoff.Request{Pair: PairTDDBuild}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file := filepath.Join(dir, entry.Name())
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return handoff.Request{}, err
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isPredicate(fn) {
				continue
			}
			name := fn.Name.Name
			req.Asks = append(req.Asks, handoff.Ask{
				ID:           name,
				Text:         name,
				Source:       fmt.Sprintf("acs/%s/%s:%s", pkg, filepath.Base(file), name),
				EvidenceKind: "green_run",
				Required:     true,
			})
		}
	}
	if len(req.Asks) == 0 {
		return handoff.Request{}, fmt.Errorf("%w in %s", ErrNoPredicates, dir)
	}
	return req, nil
}

func isPredicate(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
		return false
	}
	params := fn.Type.Params.List
	return len(params) == 1 && isTestingT(params[0].Type)
}

func isTestingT(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && sel.Sel.Name == "T"
}
