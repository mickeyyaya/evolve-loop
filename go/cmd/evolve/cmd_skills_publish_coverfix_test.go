package main

import "testing"

func TestRenderAgy_ReturnsErrorWhenASkillTreeCannotBeRead(t *testing.T) {
	t.Parallel()
	project := t.TempDir()

	files, err := renderAgy(project, []canonicalSkill{{Name: "missing-skill"}})

	if err == nil {
		t.Fatalf("renderAgy with no skills/missing-skill dir returned nil error, files=%v", files)
	}
	if files != nil {
		t.Fatalf("renderAgy must not return a partial file set on error, got %d files", len(files))
	}
}
