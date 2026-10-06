package plane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHub_IsTheBareStoresParentWithItsDevDirectory(t *testing.T) {
	root, gitdir := linkedFixture(t, "main")
	if err := os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Clean(filepath.Join(gitdir, "..", ".."))

	hub, err := ResolveHub(root)

	if err != nil {
		t.Fatal(err)
	}
	if want := (Hub{Root: filepath.Dir(store), Store: store}); hub != want {
		t.Errorf("hub = %+v, want %+v", hub, want)
	}
	if got := hub.DevDir(); got != filepath.Join(filepath.Dir(store), "dev") {
		t.Errorf("DevDir = %s, want <root>/dev", got)
	}
	if OriginMainRef != "refs/remotes/origin/main" {
		t.Errorf("OriginMainRef = %q: dev worktrees are cut from, and judged against, the fetched origin main", OriginMainRef)
	}
}

func TestResolveHub_RefusesAPrimaryCheckoutAndANonRepository(t *testing.T) {
	_, err := ResolveHub(primaryFixture(t, "main"))
	if err == nil || !strings.Contains(err.Error(), "bare-store layout") {
		t.Errorf("ResolveHub(primary checkout) err = %v, want the bare-store refusal", err)
	}
	if _, err := ResolveHub(t.TempDir()); err == nil {
		t.Error("ResolveHub accepted a directory that is not a checkout")
	}
}
