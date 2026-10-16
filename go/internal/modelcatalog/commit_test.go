package modelcatalog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedCatalog(t *testing.T, dir string) Catalog {
	t.Helper()
	prior := Catalog{
		FetchedAt: time.Now().Add(-48 * time.Hour),
		CLIs: map[string]CLIEntry{
			"claude": {
				TierModels:    map[string]string{"deep": "opus", "top": "opus"},
				TierFallbacks: map[string][]string{"deep": {"opus", "sonnet"}, "top": {"opus", "sonnet"}},
				Source:        SourceLive,
			},
		},
	}
	if err := Write(dir, prior); err != nil {
		t.Fatalf("seed Write: %v", err)
	}
	return prior
}

func TestCommit_PreservesPriorTierFallbacks(t *testing.T) {
	dir := t.TempDir()
	seedCatalog(t, dir)

	fresh := Catalog{
		FetchedAt: time.Now(),
		CLIs: map[string]CLIEntry{
			"claude": {TierModels: map[string]string{"deep": "opus", "top": "opus"}, Source: SourceLive},
		},
	}
	committed, err := Commit(dir, fresh, nil)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := committed.CLIs["claude"].TierFallbacks["deep"]; len(got) != 2 {
		t.Errorf("Commit returned tier_fallbacks.deep = %v, want the merged chain — callers print this value, so returning the unmerged input would hide the preserved chains", got)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	chain := got.CLIs["claude"].TierFallbacks["deep"]
	if len(chain) != 2 || chain[0] != "opus" || chain[1] != "sonnet" {
		t.Errorf("claude tier_fallbacks.deep = %v, want [opus sonnet] — a refresh must carry operator-authored chains forward; "+
			"the catalog is gitignored, so a dropped chain is unrecoverable and leaves a walled tier with no alternative model", chain)
	}
}

func TestCommit_CorruptPriorIsWarnedNotFatal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}

	var warn bytes.Buffer
	fresh := Catalog{FetchedAt: time.Now(), CLIs: map[string]CLIEntry{"claude": {TierModels: map[string]string{"deep": "opus"}, Source: SourceLive}}}
	if _, err := Commit(dir, fresh, &warn); err != nil {
		t.Fatalf("Commit over a corrupt prior must succeed (refresh is the repair path), got: %v", err)
	}
	if warn.Len() == 0 {
		t.Error("a corrupt prior catalog must be WARNed — silently discarding operator state is the failure mode this seam exists to prevent")
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read after repair: %v", err)
	}
	if got.CLIs["claude"].TierModels["deep"] != "opus" {
		t.Error("Commit did not repair the corrupt catalog")
	}
}

func TestCommit_NilWarnWritesWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	if _, err := Commit(dir, Catalog{FetchedAt: time.Now()}, nil); err != nil {
		t.Fatalf("Commit with nil warn: %v", err)
	}
}

func TestWrite_RetainsPreviousCatalogForRollback(t *testing.T) {
	dir := t.TempDir()
	first := Catalog{FetchedAt: time.Now(), CLIs: map[string]CLIEntry{"claude": {TierModels: map[string]string{"deep": "opus"}, Source: SourceLive}}}
	if err := Write(dir, first); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	second := Catalog{FetchedAt: time.Now(), CLIs: map[string]CLIEntry{"claude": {TierModels: map[string]string{"deep": "sonnet"}, Source: SourceLive}}}
	if err := Write(dir, second); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, PrevFileName))
	if err != nil {
		t.Fatalf("reading %s: %v — the overwritten catalog must be retained for rollback", PrevFileName, err)
	}
	if !bytes.Contains(raw, []byte(`"opus"`)) {
		t.Errorf("%s does not hold the overwritten catalog (want deep=opus), got:\n%s", PrevFileName, raw)
	}
	live, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if live.CLIs["claude"].TierModels["deep"] != "sonnet" {
		t.Error("retaining the previous catalog must not disturb the live write")
	}
}

func TestWrite_FirstWriteSucceedsWithNoPriorToRetain(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Catalog{FetchedAt: time.Now()}); err != nil {
		t.Fatalf("first Write with no prior must succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, PrevFileName)); err == nil {
		t.Errorf("%s must not be fabricated when there was no prior catalog", PrevFileName)
	}
}

func TestWrite_RetainedCopyIsNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	c := Catalog{FetchedAt: time.Now(), CLIs: map[string]CLIEntry{"claude": {TierModels: map[string]string{"deep": "opus"}, Source: SourceLive}}}
	if err := Write(dir, c); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if err := Write(dir, c); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	live, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("stat live: %v", err)
	}
	prev, err := os.Stat(filepath.Join(dir, PrevFileName))
	if err != nil {
		t.Fatalf("stat prev: %v", err)
	}
	if prev.Mode().Perm()&^live.Mode().Perm() != 0 {
		t.Errorf("%s mode %v is more permissive than the catalog it copies (%v) — a retained copy must never widen access",
			PrevFileName, prev.Mode().Perm(), live.Mode().Perm())
	}
}

func TestWrite_RetainedCopyDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("ORIGINAL"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, PrevFileName)); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	c := Catalog{FetchedAt: time.Now(), CLIs: map[string]CLIEntry{"claude": {TierModels: map[string]string{"deep": "opus"}, Source: SourceLive}}}
	if err := Write(dir, c); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if err := Write(dir, c); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(got) != "ORIGINAL" {
		t.Errorf("catalog retention wrote THROUGH a planted symlink into %s (now %q) — retention must replace the "+
			"link atomically, never follow it", victim, string(got)[:min(40, len(got))])
	}
}
