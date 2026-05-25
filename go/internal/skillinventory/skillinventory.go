// Package skillinventory builds .evolve/skill-inventory.json — a cache
// of all available skills indexed by category. Scout uses the
// categoryIndex subset matching project language/framework/task-types
// as routing hints (`skillCategories`).
//
// Cache semantics: the output file's mtime determines freshness. When
// it's younger than TTL, Build is a no-op (returns the cached path).
// Operators force a rebuild via --force or by deleting the file.
//
// Skill discovery: walks <project_root>/skills/*/SKILL.md and parses
// the YAML frontmatter for `categories: [...]` and `name: ...` fields.
// Skills without frontmatter are listed under category "uncategorized"
// so they're still discoverable.
//
// v12.1 Phase 2A port. CLI: `evolve skill-inventory build [--ttl 1h]`.
package skillinventory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// DefaultTTL is the cache freshness window. 1h matches the bash
// script's behavior so operators see no change in cadence.
const DefaultTTL = time.Hour

// Inventory is the on-disk JSON shape. Stable; the schema is the
// public contract for Scout's skillCategories input.
type Inventory struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	ProjectRoot   string              `json:"project_root"`
	SkillCount    int                 `json:"skill_count"`
	Skills        []SkillEntry        `json:"skills"`
	CategoryIndex map[string][]string `json:"category_index"` // category → []skill name
}

// SkillEntry is a single skill's summary line.
type SkillEntry struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description,omitempty"`
	Categories  []string `json:"categories,omitempty"`
}

// Options configures Build. ProjectRoot is required. TTL defaults to
// DefaultTTL. NowFn is the time source (override for deterministic
// tests). Force forces a rebuild regardless of cache freshness.
type Options struct {
	ProjectRoot string
	TTL         time.Duration
	NowFn       func() time.Time
	Force       bool
}

// Result captures the outcome of a Build call.
type Result struct {
	OutputPath string
	CacheHit   bool // true when the cache was fresh enough to skip rebuild
	SkillCount int
}

// Build produces the inventory at <project_root>/.evolve/skill-inventory.json.
// Returns Result describing what was written (or that the cache was reused).
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

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("skillinventory: mkdir: %w", err)
	}
	if err := writeAtomic(outPath, inv); err != nil {
		return Result{}, fmt.Errorf("skillinventory: write: %w", err)
	}
	return Result{OutputPath: outPath, SkillCount: inv.SkillCount}, nil
}

// scan walks the skills directory and assembles the Inventory.
func scan(projectRoot string, now time.Time) (Inventory, error) {
	loader := prompts.NewForProject(projectRoot)
	names, err := loader.Skills()
	if err != nil {
		// No skills dir is not fatal — emit an empty inventory.
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
	// Stable ordering for deterministic diffs.
	sort.Slice(inv.Skills, func(i, j int) bool { return inv.Skills[i].Name < inv.Skills[j].Name })
	for _, list := range inv.CategoryIndex {
		sort.Strings(list)
	}
	return inv, nil
}

// extractCategories parses frontmatter for category hints. Handles
// every shape the prompts.ParseFrontmatter helper can emit:
//   - `categories: [a, b]` → []string (the common path)
//   - `categories: "a, b"`  → string (comma-split fallback)
//   - `categories: [a, b]` parsed as []any (defensive — third-party loaders)
//
// Empty values and unsupported types degrade to nil so the caller
// maps the skill to "uncategorized" rather than crashing.
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

// writeAtomic uses the project's mv-of-tempfile convention so a
// half-written inventory never appears on disk.
func writeAtomic(path string, inv Inventory) error {
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
