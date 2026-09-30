//go:build acs

package cycle334

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const quotaresetSrcRel = "go/internal/quotareset/quotareset.go"

var refNow = time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)

func writeHint(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "quota-reset-hint.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write hint fixture: %v", err)
	}
	return dir
}

func assertNoCompileInParseHint(t *testing.T) {
	t.Helper()
	src := filepath.Join(acsassert.RepoRoot(t), quotaresetSrcRel)
	n, err := acsassert.CountInGoFunc(src, "parseHint", "regexp.MustCompile")
	if err != nil {
		t.Fatalf("RED: cannot scan parseHint body in %s: %v", quotaresetSrcRel, err)
	}
	if n != 0 {
		t.Errorf("RED: parseHint body still compiles %d regexp(s) — hoist the pattern to a package-level var (want 0)", n)
	}
}

func TestC334_001_ParseHintBehaviourPreservedViaCompute(t *testing.T) {
	fixedNow := quotareset.Options{Now: func() time.Time { return refNow }}
	withHours := func(h float64) quotareset.Options {
		return quotareset.Options{Now: func() time.Time { return refNow }, HoursFn: func() float64 { return h }}
	}

	if r, err := quotareset.Compute(writeHint(t, "resets 8:30pm"), fixedNow); err != nil {
		t.Fatalf("Compute(8:30pm): %v", err)
	} else if r.Source != "parsed" || r.WakeAt.Hour() != 20 || r.WakeAt.Minute() != 30 {
		t.Errorf("8:30pm: got Source=%q %02d:%02d, want parsed 20:30", r.Source, r.WakeAt.Hour(), r.WakeAt.Minute())
	}

	if r, err := quotareset.Compute(writeHint(t, "5:20am"), fixedNow); err != nil {
		t.Fatalf("Compute(5:20am): %v", err)
	} else if r.Source != "parsed" {
		t.Errorf("5:20am: Source=%q want parsed", r.Source)
	} else if r.WakeAt.Day() != refNow.AddDate(0, 0, 1).Day() {
		t.Errorf("5:20am: WakeAt day=%d want %d (tomorrow)", r.WakeAt.Day(), refNow.AddDate(0, 0, 1).Day())
	}

	if r, err := quotareset.Compute(writeHint(t, "garbage no time"), withHours(5.0)); err != nil {
		t.Fatalf("Compute(garbage): %v", err)
	} else if r.Source != "default" {
		t.Errorf("garbage: Source=%q want default (no regexp match)", r.Source)
	}

	if r, err := quotareset.Compute(writeHint(t, "99:99am"), withHours(5.0)); err != nil {
		t.Fatalf("Compute(99:99am): %v", err)
	} else if r.Source != "default" {
		t.Errorf("99:99am: Source=%q want default (minutes out of range)", r.Source)
	}

	assertNoCompileInParseHint(t)
}

// acs-predicate: config-check — this gate inherently asserts a SOURCE-STRUCTURE
func TestC334_002_QuotaresetHintRegexpHoisted(t *testing.T) {
	src := filepath.Join(acsassert.RepoRoot(t), quotaresetSrcRel)

	if !acsassert.FileMatchesRegex(t, src, `var\s+hintTimeRE\s*=\s*regexp\.MustCompile`) {
		t.Errorf("RED: no package-level `var hintTimeRE = regexp.MustCompile(...)` in %s — the pattern was not hoisted", quotaresetSrcRel)
	}

	n, err := acsassert.CountInGoFunc(src, "parseHint", "regexp.MustCompile")
	if err != nil {
		t.Fatalf("RED: cannot scan parseHint body: %v", err)
	}
	if n != 0 {
		t.Errorf("RED: parseHint body still compiles %d regexp(s) — want 0 (use the package-level hintTimeRE)", n)
	}
}
