//go:build acs

package cycle420

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC420_001_WriteCatalogNoEnumeration(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_OverflowNotEnumerated",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestWriteCatalog_OverflowNotEnumerated failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_002_WriteCatalogPointerPresent(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_PointerLinePresent",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestWriteCatalog_PointerLinePresent failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_003_WriteCatalogPointerRequired_Negative(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_PointerLineRequired_Negative",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestWriteCatalog_PointerLineRequired_Negative failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_004_WriteCatalogNoOverflowNoPointer(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestWriteCatalog_NoOverflow_NoPointer",
		"./internal/core/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestWriteCatalog_NoOverflow_NoPointer failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_005_RouterCompactionRegression(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterCompaction",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterCompaction failed after writeCatalog change (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_006_RouterPersonaTSCMarker(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_TSCMarkerPresent",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestRouterPersona_TSCMarkerPresent failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_007_RouterPersonaProseReduction(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_ProseRegionByteReduction",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestRouterPersona_ProseRegionByteReduction failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_008_RouterPersonaCatalogIdentical_Negative(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_CatalogByteIdentical_Negative",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterPersona_CatalogByteIdentical_Negative failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_009_RouterPersonaDomainVocabPreserved(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_DomainVocabPreserved",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterPersona_DomainVocabPreserved failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

func TestC420_010_RouterPersonaLoaderAndRenderGreen(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRouterPersona_LoaderAndRenderParseGreen",
		"./internal/prompts/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRouterPersona_LoaderAndRenderParseGreen failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}
