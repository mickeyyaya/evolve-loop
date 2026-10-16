package modelcatalog

import (
	"fmt"
	"io"
)

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
