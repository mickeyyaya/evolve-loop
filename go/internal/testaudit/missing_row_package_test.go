package testaudit

import "testing"

func TestMissingRowPackage_RoundTripsEveryRenderedMissingRow(t *testing.T) {
	t.Parallel()
	for _, pkg := range []string{"acs/cycle10", "internal/core", "internal/some-dash"} {
		rendered := (&RowError{Kind: RowMissing, Finding: Finding{Package: pkg, Test: "TestX", Rule: "error-text-match"}}).Error()

		got, ok := MissingRowPackage(rendered)

		if !ok || got != pkg {
			t.Errorf("MissingRowPackage(%q) = %q, %t; want %q, true", rendered, got, ok, pkg)
		}
	}
}

func TestMissingRowPackage_RefusesAStaleRowAndFreeText(t *testing.T) {
	t.Parallel()
	stale := (&RowError{Kind: RowStale, Finding: Finding{Package: "acs/cycle10", Test: "TestX", Rule: "error-text-match"}}).Error()
	for _, text := range []string{stale, "gofmt: 1 file(s) unformatted", "", "testaudit: acs/cycle10"} {
		if got, ok := MissingRowPackage(text); ok || got != "" {
			t.Errorf("MissingRowPackage(%q) = %q, %t; want no package", text, got, ok)
		}
	}
}
