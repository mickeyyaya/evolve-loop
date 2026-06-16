package skillinventory

import (
	"testing"
	"time"
)

// TestDefaultTTL_Value names DefaultTTL and pins it to one hour — the cache
// cadence the package doc promises. It is the freshness window Build uses when
// Options.TTL is left zero. Mirrors the sibling phaseinventory graduation.
func TestDefaultTTL_Value(t *testing.T) {
	if DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", DefaultTTL)
	}
}

// TestResult_PopulatedByBuild names the Result type and pins that a forced
// (non-cache-hit) Build returns the written path, CacheHit=false, and the
// scanned skill count — Result is Build's structured outcome.
func TestResult_PopulatedByBuild(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha
categories: [testing]`)

	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var got Result = res

	if got.CacheHit {
		t.Error("Result.CacheHit = true, want false on a forced rebuild")
	}
	if got.SkillCount != 1 {
		t.Errorf("Result.SkillCount = %d, want 1", got.SkillCount)
	}
	if got.OutputPath == "" {
		t.Error("Result.OutputPath is empty, want the written inventory path")
	}
}

// TestSkillEntry_FullEquality names the SkillEntry type and pins its shape: the
// scanned alpha skill must equal a fully-specified SkillEntry literal (name,
// project-relative SKILL.md path, parsed description, ordered categories) — the
// per-skill summary line Scout consumes from the inventory.
func TestSkillEntry_FullEquality(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha
description: first
categories: [testing, python]`)

	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	inv := readInventory(t, res.OutputPath)
	if len(inv.Skills) != 1 {
		t.Fatalf("Skills = %+v, want exactly 1 entry", inv.Skills)
	}
	got := inv.Skills[0]

	want := SkillEntry{
		Name:        "alpha",
		Path:        "skills/alpha/SKILL.md",
		Description: "first",
		Categories:  []string{"testing", "python"},
	}
	if got.Name != want.Name || got.Path != want.Path || got.Description != want.Description {
		t.Errorf("SkillEntry = %+v, want %+v", got, want)
	}
	if len(got.Categories) != len(want.Categories) {
		t.Fatalf("SkillEntry.Categories = %v, want %v", got.Categories, want.Categories)
	}
	for i := range want.Categories {
		if got.Categories[i] != want.Categories[i] {
			t.Errorf("Categories[%d] = %q, want %q", i, got.Categories[i], want.Categories[i])
		}
	}
}
