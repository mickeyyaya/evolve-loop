package guidance

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const rulesDir = "guidance/rules"

func Load(root string) ([]Rule, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rulesDir)))
	if err != nil {
		return nil, fmt.Errorf("read the rule registry: %w", err)
	}
	var rules []Rule
	var decodeErrs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rel := path.Join(rulesDir, e.Name())
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		r, err := ParseRule(rel, strings.TrimSuffix(e.Name(), ".md"), raw)
		if err != nil {
			decodeErrs = append(decodeErrs, err)
			continue
		}
		rules = append(rules, r)
	}
	return rules, errors.Join(decodeErrs...)
}

func IsDecodeError(err error) bool {
	var d *DecodeError
	return errors.As(err, &d)
}

func Active(rules []Rule) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Status == StatusActive {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return lessRule(out[i], out[j]) })
	return out
}

func lessRule(a, b Rule) bool {
	if a.Layer != b.Layer {
		return a.Layer < b.Layer
	}
	if a.Order != b.Order {
		return orderKey(a.Order) < orderKey(b.Order)
	}
	return a.ID < b.ID
}

func orderKey(order int) int {
	if order == 0 {
		return math.MaxInt
	}
	return order
}

func BodyPath(id string) string { return path.Join(rulesDir, id+".md") }
