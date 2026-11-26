package structure

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestStructure_ProctreeAndFaketmuxRunNoPS(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")

	proctreeRunners, execErr := ExecImporters(root, "internal/proctree")
	faketmuxPS, psErr := ProgramLiteralFiles(root, "ps", "internal/faketmux")

	if execErr != nil || psErr != nil {
		t.Fatalf("scan errors = %v, %v", execErr, psErr)
	}
	if want := []string{filepath.Join("internal", "proctree", "cwd.go")}; !slices.Equal(proctreeRunners, want) {
		t.Errorf("proctree files that run a process = %v, want %v (the lsof cwd lister, until its follow-up lands): the table reads the kernel", proctreeRunners, want)
	}
	if len(faketmuxPS) != 0 {
		t.Errorf("faketmux files that run the ps program = %v, want none", faketmuxPS)
	}
}

func TestProgramLiteralFiles_KeepsOnlyFilesThatRunAProcessAndNameTheProgramWhole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a/runs.go":    "package a\nimport \"os/exec\"\nvar _ = exec.Command\nconst bin = \"ps\"\n",
		"a/substr.go":  "package a\nimport \"os/exec\"\nvar _ = exec.Command\nconst bin = \"steps\"\n",
		"a/noexec.go":  "package a\nconst bin = \"ps\"\n",
		"a/x_test.go":  "package a\nimport \"os/exec\"\nvar _ = exec.Command\nconst bin = \"ps\"\n",
		"a/sysexec.go": "package a\nimport \"" + sysexecImport + "\"\nvar _ = sysexec.Command\nconst bin = \"ps\"\n",
	})

	got, err := ProgramLiteralFiles(root, "ps", ".")

	want := []string{filepath.Join("a", "runs.go"), filepath.Join("a", "sysexec.go")}
	slices.Sort(got)
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("ProgramLiteralFiles = %v, %v, want %v", got, err, want)
	}
}
