//go:build acs

package cycle335

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/textutil"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	logfilterSrcRel       = "go/internal/logfilter/streamjson.go"
	phasestreamSrcRel     = "go/internal/phasestream/classify.go"
	textutilSrcRel        = "go/internal/textutil/textutil.go"
	semvercheckSrcRel     = "go/internal/semvercheck/semvercheck.go"
	changeloggenSrcRel    = "go/internal/changeloggen/changeloggen.go"
	versionbumpSrcRel     = "go/internal/versionbump/versionbump.go"
	marketplacepollSrcRel = "go/internal/marketplacepoll/marketplacepoll.go"
	releasepipelineSrcRel = "go/internal/releasepipeline/releasepipeline.go"
)

func TestC335_001_TextutilTruncateBehaviour(t *testing.T) {
	if got := textutil.TruncateInline("hello", 100); got != "hello" {
		t.Errorf("TruncateInline short: got %q, want %q (must return input unchanged)", got, "hello")
	}

	if got, want := textutil.TruncateInline("0123456789", 4), "0123… (6 bytes elided)"; got != want {
		t.Errorf("TruncateInline long: got %q, want %q", got, want)
	}

	if got := textutil.TruncateMiddle("short", 5, 5); got != "short" {
		t.Errorf("TruncateMiddle short: got %q, want %q (must return input unchanged)", got, "short")
	}

	s := "HEAD!" + strings.Repeat("x", 40) + "!TAIL"
	if got, want := textutil.TruncateMiddle(s, 5, 5), "HEAD!… (40 bytes elided) …!TAIL"; got != want {
		t.Errorf("TruncateMiddle long: got %q, want %q", got, want)
	}
}

// acs-predicate: config-check — this gate asserts a SOURCE-STRUCTURE outcome of
func TestC335_002_TruncateLocalDefsRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)

	if !acsassert.FileExists(t, filepath.Join(root, textutilSrcRel)) {
		t.Errorf("RED: %s missing — the extracted truncation helpers have no home", textutilSrcRel)
	}

	for _, rel := range []string{logfilterSrcRel, phasestreamSrcRel} {
		src := filepath.Join(root, rel)
		if n := acsassert.CountOccurrencesAny(src, "func truncateInline", "func truncateMiddle"); n != 0 {
			t.Errorf("RED: %s still defines %d local truncate helper(s) — delegate to textutil instead (want 0)", rel, n)
		}
	}
}

func TestC335_003_SemverIsSemverBehaviour(t *testing.T) {
	valid := []string{"1.2.3", "0.0.0", "10.20.30"}
	invalid := []string{"v1.2.3", "1.2", "1.2.3.4", "", "abc", "1.2.3-rc1"}

	for _, s := range valid {
		if !semvercheck.IsSemver(s) {
			t.Errorf("semvercheck.IsSemver(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if semvercheck.IsSemver(s) {
			t.Errorf("semvercheck.IsSemver(%q) = true, want false (must reject non-MAJOR.MINOR.PATCH)", s)
		}
	}

	for _, s := range valid {
		if !changeloggen.IsSemver(s) {
			t.Errorf("changeloggen.IsSemver(%q) = false, want true (delegation regressed)", s)
		}
	}
	for _, s := range invalid {
		if changeloggen.IsSemver(s) {
			t.Errorf("changeloggen.IsSemver(%q) = true, want false (delegation regressed)", s)
		}
	}
}

// acs-predicate: config-check — asserts the SOURCE-STRUCTURE outcome: the local
func TestC335_004_SemverRELocalDefsRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)

	if !acsassert.FileExists(t, filepath.Join(root, semvercheckSrcRel)) {
		t.Errorf("RED: %s missing — the extracted semverRE/IsSemver have no home", semvercheckSrcRel)
	}

	for _, rel := range []string{
		changeloggenSrcRel, versionbumpSrcRel, marketplacepollSrcRel, releasepipelineSrcRel,
	} {
		src := filepath.Join(root, rel)
		if n := acsassert.CountOccurrencesAny(src, "var semverRE"); n != 0 {
			t.Errorf("RED: %s still declares `var semverRE` (%d) — delegate to semvercheck instead (want 0)", rel, n)
		}
	}
}
