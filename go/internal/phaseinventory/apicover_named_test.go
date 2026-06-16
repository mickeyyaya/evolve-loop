package phaseinventory

import (
	"path/filepath"
	"testing"
	"time"
)

// TestDefaultTTL_Value names DefaultTTL and pins it to one hour — the cache
// cadence the package doc promises ("matches skillinventory's cadence"). The
// value is load-bearing: it is the TTL used when Options.TTL is left zero.
func TestDefaultTTL_Value(t *testing.T) {
	t.Parallel()
	if DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", DefaultTTL)
	}
}

// TestOutputFile_DrivesBuildOutputPath names OutputFile and pins that Build
// writes the inventory to <ProjectRoot>/<OutputFile> — i.e. OutputFile is the
// project-relative location the result path is composed from, not an unused
// constant.
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
