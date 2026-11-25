package bridge

import (
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func TestSetModelCatalogDirFn(t *testing.T) {
	const sentinel = "t6d-live-deep-sentinel"
	dir := t.TempDir()
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{"claude": {Source: modelcatalog.SourceLive,
		TierModels: map[string]string{"deep": sentinel}}}}
	if err := modelcatalog.Write(dir, cat); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { SetModelCatalogDirFn(func() string { return "" }) })
	SetModelCatalogDirFn(func() string { return dir })
	m, err := gobridge.LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.ModelTierMap["deep"]; got != sentinel {
		t.Errorf("deep tier = %q, want the catalog model %q read through the adapter setter", got, sentinel)
	}
}
