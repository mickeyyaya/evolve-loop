package modelcatalog

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteFailurePreservesExistingCatalogAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	original := amplificationCatalog("claude-sonnet")
	replacement := amplificationCatalog("claude-opus")

	if err := Write(dir, original); err != nil {
		t.Fatalf("write original catalog: %v", err)
	}

	restoreCreateTemp := createTemp
	t.Cleanup(func() { createTemp = restoreCreateTemp })
	createTemp = func(dir, pattern string) (tempFile, error) {
		f, err := restoreCreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return &amplificationWriteFailFile{tempFile: f}, nil
	}

	if err := Write(dir, replacement); err == nil {
		t.Fatal("Write succeeded after injected temp write failure")
	}

	assertAmplificationCatalogModel(t, dir, "claude-sonnet")
	assertAmplificationNoTemps(t, dir)
}

func TestWriteCloseFailurePreservesExistingCatalogAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	original := amplificationCatalog("claude-sonnet")
	replacement := amplificationCatalog("claude-opus")

	if err := Write(dir, original); err != nil {
		t.Fatalf("write original catalog: %v", err)
	}

	restoreCreateTemp := createTemp
	t.Cleanup(func() { createTemp = restoreCreateTemp })
	createTemp = func(dir, pattern string) (tempFile, error) {
		f, err := restoreCreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return &amplificationCloseFailFile{tempFile: f}, nil
	}

	if err := Write(dir, replacement); err == nil {
		t.Fatal("Write succeeded after injected temp close failure")
	}

	assertAmplificationCatalogModel(t, dir, "claude-sonnet")
	assertAmplificationNoTemps(t, dir)
}

func amplificationCatalog(model string) Catalog {
	return Catalog{
		FetchedAt: time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
		CLIs: map[string]CLIEntry{
			"claude": {
				TierModels: map[string]string{"balanced": model},
				Available:  []string{model},
				Source:     SourceLive,
			},
		},
	}
}

func assertAmplificationCatalogModel(t *testing.T, dir, want string) {
	t.Helper()

	gotCatalog, err := Read(dir)
	if err != nil {
		t.Fatalf("read catalog after failed write: %v", err)
	}
	got, ok := gotCatalog.Lookup("claude", "balanced")
	if !ok {
		t.Fatal("catalog entry missing after failed write")
	}
	if got != want {
		t.Fatalf("catalog was replaced after failed write: got model %q, want %q", got, want)
	}
}

func assertAmplificationNoTemps(t *testing.T, dir string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, FileName+".*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("failed write leaked temp files: %v", matches)
	}
}

type amplificationWriteFailFile struct {
	tempFile
}

func (f *amplificationWriteFailFile) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, errors.New("injected temp write failure")
	}
	return len(p) / 2, errors.New("injected temp write failure")
}

type amplificationCloseFailFile struct {
	tempFile
}

func (f *amplificationCloseFailFile) Close() error {
	_ = f.tempFile.Close()
	return errors.New("injected temp close failure")
}
