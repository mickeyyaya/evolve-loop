package journeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const RegistryPath = "go/test/journeys/journeys.json"

type Retired struct {
	Reason string `json:"reason"`
}

type Row struct {
	ID      string   `json:"id"`
	Journey string   `json:"journey"`
	Test    string   `json:"test"`
	Issue   string   `json:"issue"`
	Retired *Retired `json:"retired,omitempty"`
}

type Registry struct {
	Rows []Row `json:"rows"`
}

var idRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func ValidID(id string) bool { return idRE.MatchString(id) }

func Parse(raw []byte) (Registry, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var reg Registry
	if err := dec.Decode(&reg); err != nil {
		return Registry{}, fmt.Errorf("journeys: %w", err)
	}
	seen := map[string]bool{}
	for _, row := range reg.Rows {
		if err := row.validate(seen); err != nil {
			return Registry{}, fmt.Errorf("journeys: row %q: %w", row.ID, err)
		}
	}
	return reg, nil
}

func (r Row) validate(seen map[string]bool) error {
	_, name := r.split()
	switch {
	case !ValidID(r.ID):
		return fmt.Errorf("the id is not a slug")
	case seen[r.ID]:
		return fmt.Errorf("the id is a duplicate")
	case strings.TrimSpace(r.Journey) == "", strings.TrimSpace(r.Issue) == "":
		return fmt.Errorf("the journey and the issue must not be empty")
	case !strings.HasPrefix(name, "Test"):
		return fmt.Errorf("the test %q is not <package dir>.Test<Name>", r.Test)
	case r.Retired != nil && strings.TrimSpace(r.Retired.Reason) == "":
		return fmt.Errorf("a retired row needs a reason")
	}
	seen[r.ID] = true
	return nil
}

func (r Row) split() (dir, name string) {
	i := strings.LastIndex(r.Test, ".")
	if i <= 0 {
		return "", ""
	}
	return r.Test[:i], r.Test[i+1:]
}

func Load(root string) (Registry, error) {
	raw, err := os.ReadFile(filepath.Join(root, RegistryPath))
	if err != nil {
		return Registry{}, err
	}
	return Parse(raw)
}

func (r Registry) Active() []Row {
	var out []Row
	for _, row := range r.Rows {
		if row.Retired == nil {
			out = append(out, row)
		}
	}
	return out
}

func (r Registry) Known(id string) bool {
	for _, row := range r.Active() {
		if row.ID == id {
			return true
		}
	}
	return false
}

func (r Registry) Removed(base Registry) []string {
	kept := map[string]bool{}
	for _, row := range r.Rows {
		kept[row.ID] = true
	}
	var out []string
	for _, row := range base.Rows {
		if !kept[row.ID] {
			out = append(out, row.ID)
		}
	}
	return out
}

func MissingTests(goRoot string, rows []Row) ([]string, error) {
	var out []string
	for _, row := range rows {
		if row.Retired != nil {
			continue
		}
		dir, name := row.split()
		found, err := declaresTest(filepath.Join(goRoot, filepath.FromSlash(dir)), name)
		if err != nil {
			return nil, err
		}
		if !found {
			out = append(out, row.ID)
		}
	}
	return out, nil
}

func declaresTest(dir, name string) (bool, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
				return true, nil
			}
		}
	}
	return false, nil
}
