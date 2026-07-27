package modelcatalog

import (
	"fmt"
	"io"
)

// Commit persists next to evolveDir, carrying forward the operator-authored
// state from the catalog currently on disk (today: TierFallbacks, via
// MergeFallbacks gap-fill — a fresh entry that brings its own chains keeps
// them). It is the ONE write seam for a refresh; callers must not compose
// Read/MergeFallbacks/Write themselves (they had drifted: `evolve models
// refresh` merged, the cycle-start auto-refresh did not, silently destroying
// chains nothing regenerates — the catalog file is gitignored).
//
// A corrupt prior catalog is WARNed and treated as empty rather than returned
// as an error: a refresh IS the repair path for a corrupt cache, so refusing
// to write would strand the operator with an unusable file. A nil warn writer
// discards.
//
// Returns the catalog as PERSISTED — next with the carried-forward chains
// merged in, not next itself. Callers that report the result (`evolve models
// refresh` prints it) must use this value: printing next would show the
// operator a catalog missing the very chains Commit just preserved.
func Commit(evolveDir string, next Catalog, warn io.Writer) (Catalog, error) {
	if warn == nil {
		warn = io.Discard
	}
	prior, err := Read(evolveDir)
	if err != nil {
		fmt.Fprintf(warn, "[modelcatalog] WARN prior catalog unreadable — operator-authored tier_fallbacks cannot be carried forward; "+
			"this refresh repairs the file: %v\n", err)
		prior = Catalog{}
	}
	merged := MergeFallbacks(prior, next)
	if werr := Write(evolveDir, merged); werr != nil {
		return Catalog{}, werr
	}
	return merged, nil
}
