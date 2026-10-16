// Package skillinventory builds .evolve/skill-inventory.json — a cache of
// all available skills indexed by category.
// See docs/architecture/packages/internal-skillinventory.md.
package skillinventory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const DefaultTTL = time.Hour

type Inventory struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	ProjectRoot   string              `json:"project_root"`
	SkillCount    int                 `json:"skill_count"`
	Skills        []SkillEntry        `json:"skills"`
	CategoryIndex map[string][]string `json:"category_index"`
}

type SkillEntry struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description,omitempty"`
	Categories  []string `json:"categories,omitempty"`
}

type Options struct {
	ProjectRoot string
	TTL         time.Duration
	NowFn       func() time.Time
	Force       bool
}

type Result struct {
	OutputPath string
	CacheHit   bool
	SkillCount int
}

func Build(opts Options) (Result, error) {
	if opts.ProjectRoot == "" {
		return Result{}, fmt.Errorf("skillinventory: ProjectRoot required")
	}
	ttl := opts.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}

	outDir := filepath.Join(opts.ProjectRoot, ".evolve")
	outPath := filepath.Join(outDir, "skill-inventory.json")

	if !opts.Force {
		if info, err := os.Stat(outPath); err == nil {
			age := nowFn().Sub(info.ModTime())
			if age < ttl {
				return Result{OutputPath: outPath, CacheHit: true}, nil
			}
		}
	}

	inv, err := scan(opts.ProjectRoot, nowFn())
	if err != nil {
		return Result{}, fmt.Errorf("skillinventory: scan: %w", err)
	}

	if err := atomicwrite.JSON(outPath, inv); err != nil {
		return Result{}, fmt.Errorf("skillinventory: write: %w", err)
	}
	return Result{OutputPath: outPath, SkillCount: inv.SkillCount}, nil
}

func scan(projectRoot string, now time.Time) (Inventory, error) {
	loader := prompts.NewFromDir(projectRoot)
	names, err := loader.Skills()
	if err != nil {
		if os.IsNotExist(err) {
			return Inventory{
				GeneratedAt:   now,
				ProjectRoot:   projectRoot,
				CategoryIndex: map[string][]string{},
			}, nil
		}
		return Inventory{}, err
	}

	inv := Inventory{
		GeneratedAt:   now,
		ProjectRoot:   projectRoot,
		Skills:        make([]SkillEntry, 0, len(names)),
		CategoryIndex: map[string][]string{},
	}

	for _, name := range names {
		entry := SkillEntry{
			Name: name,
			Path: filepath.Join("skills", name, "SKILL.md"),
		}
		if prompt, err := loader.Skill(name); err == nil {
			if d, ok := prompt.Frontmatter["description"].(string); ok {
				entry.Description = d
			}
			entry.Categories = extractCategories(prompt.Frontmatter)
		}
		if len(entry.Categories) == 0 {
			entry.Categories = []string{"uncategorized"}
		}
		for _, cat := range entry.Categories {
			inv.CategoryIndex[cat] = append(inv.CategoryIndex[cat], name)
		}
		inv.Skills = append(inv.Skills, entry)
	}
	inv.SkillCount = len(inv.Skills)
	sort.Slice(inv.Skills, func(i, j int) bool { return inv.Skills[i].Name < inv.Skills[j].Name })
	for _, list := range inv.CategoryIndex {
		sort.Strings(list)
	}
	return inv, nil
}

func extractCategories(fm map[string]any) []string {
	v, ok := fm["categories"]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if x == "" {
			return nil
		}
		parts := strings.Split(x, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}
