package phaseinventory

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultTTL_Value(t *testing.T) {
	t.Parallel()
	if DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", DefaultTTL)
	}
}

func TestOutputFile_DrivesBuildOutputPath(t *testing.T) {
	t.Parallel()
	root := fixtureProject(t)
	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := filepath.Join(root, filepath.FromSlash(OutputFile))
	if res.OutputPath != want {
		t.Errorf("OutputPath = %q, want %q (built from OutputFile)", res.OutputPath, want)
	}
}
