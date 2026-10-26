package guidance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	MaxL0Rules = 30
	MaxL0Lines = 60
)

func Check(root string, families []string) ([]string, error) {
	rules, loadErr := Load(root)
	var problems []string
	if loadErr != nil {
		if !IsDecodeError(loadErr) {
			return nil, loadErr
		}
		problems = append(problems, strings.Split(loadErr.Error(), "\n")...)
	}
	staleOut, err := staleProblems(root, rules)
	if err != nil {
		return nil, err
	}
	enforced, err := enforcedProblems(root, Active(rules))
	if err != nil {
		return nil, err
	}
	problems = append(problems, staleOut...)
	problems = append(problems, enforced...)
	problems = append(problems, duplicateProblems(rules)...)
	problems = append(problems, budgetProblems(rules)...)
	problems = append(problems, conflictProblems(rules)...)
	problems = append(problems, familyProblems(Active(rules), families)...)
	sort.Strings(problems)
	return problems, nil
}

func staleProblems(root string, rules []Rule) ([]string, error) {
	outs, err := Outputs(root, rules)
	if errors.Is(err, ErrNoL0Region) {
		return []string{"MISSING: " + err.Error()}, nil
	}
	if err != nil {
		return nil, err
	}
	drifted, err := stale(root, outs)
	if err != nil {
		return nil, err
	}
	var problems []string
	for _, o := range drifted {
		problems = append(problems, "STALE: "+o.Path+" differs from the registry; run `evolve guidance build`, then `evolve skills generate`")
	}
	return problems, nil
}

func duplicateProblems(rules []Rule) []string {
	seen := map[string]string{}
	var problems []string
	for _, r := range rules {
		key := strings.ToLower(strings.Join(strings.Fields(r.Statement), " "))
		if first, ok := seen[key]; ok {
			problems = append(problems, fmt.Sprintf("DUPLICATE: %s and %s have the same statement", first, r.ID))
			continue
		}
		seen[key] = r.ID
	}
	return problems
}

func budgetProblems(rules []Rule) []string {
	count := 0
	for _, r := range Active(rules) {
		if r.Layer == "L0" {
			count++
		}
	}
	var problems []string
	if count > MaxL0Rules {
		problems = append(problems, fmt.Sprintf("BUDGET: L0 has %d active rules; the limit is %d", count, MaxL0Rules))
	}
	if lines := strings.Count(L0Block(rules), "\n"); lines > MaxL0Lines {
		problems = append(problems, fmt.Sprintf("BUDGET: the L0 region has %d lines; the limit is %d", lines, MaxL0Lines))
	}
	return problems
}

func conflictProblems(rules []Rule) []string {
	status := map[string]string{}
	for _, r := range rules {
		status[r.ID] = r.Status
	}
	var problems []string
	for _, r := range Active(rules) {
		for _, other := range r.ConflictsWith {
			switch st, ok := status[other]; {
			case !ok:
				problems = append(problems, fmt.Sprintf("CONFLICT: %s names the unknown rule %s in conflicts_with", r.ID, other))
			case st == StatusActive:
				problems = append(problems, fmt.Sprintf("CONFLICT: %s and %s are both active; retire or supersede one", r.ID, other))
			}
		}
	}
	return problems
}

func enforcedProblems(root string, rules []Rule) ([]string, error) {
	var problems []string
	for _, r := range rules {
		for _, p := range r.EnforcedBy {
			if !filepath.IsLocal(filepath.FromSlash(p)) {
				problems = append(problems, fmt.Sprintf("ENFORCED_BY: %s names %s, which is not a path inside the repository", r.ID, p))
				continue
			}
			_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
			switch {
			case errors.Is(err, os.ErrNotExist):
				problems = append(problems, fmt.Sprintf("ENFORCED_BY: %s names %s, which does not exist", r.ID, p))
			case err != nil:
				return nil, fmt.Errorf("stat %s: %w", p, err)
			}
		}
	}
	return problems, nil
}

func familyProblems(rules []Rule, families []string) []string {
	var problems []string
	for _, r := range rules {
		if r.Layer == "L0" && len(r.Families) > 0 {
			problems = append(problems, fmt.Sprintf("FAMILIES: %s is L0, so it applies to each family; remove families", r.ID))
		}
		for _, f := range r.Families {
			if !slices.Contains(families, f) {
				problems = append(problems, fmt.Sprintf("FAMILIES: %s names %s, which no bridge manifest names", r.ID, f))
			}
		}
	}
	return problems
}
