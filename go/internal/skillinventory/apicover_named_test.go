package skillinventory

import (
	"testing"
	"time"
)

func TestDefaultTTL_Value(t *testing.T) {
	if DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", DefaultTTL)
	}
}

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
