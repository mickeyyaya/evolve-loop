package publishmirror

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// trackedBinary is the only binary tracked in the private tree (for self-deploy).
// It must be dropped from the public mirror — it embeds DWARF source paths and is
// rebuildable from source. go/bin/** is gitignored and never staged.
const trackedBinary = "go/evolve"

// commitPrefixScopePath is the per-repo commit-prefix policy file. Its
// chore(build) entry requires paths that are gitignored in the public mirror, so
// it becomes an unsatisfiable "dead prefix" and is removed.
const commitPrefixScopePath = ".evolve/commit-prefix-scope.json"

// removeBuildPrefix returns the commit-prefix-scope JSON with the "chore(build)"
// entry removed. An absent entry is not an error; invalid JSON is. Top-level keys
// are re-emitted in sorted order (deterministic) — acceptable because the mirror
// tree is regenerated, history-free, each release.
func removeBuildPrefix(jsonContent string) (string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonContent), &m); err != nil {
		return "", fmt.Errorf("parse commit-prefix-scope: %w", err)
	}
	delete(m, "chore(build)")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return "", fmt.Errorf("re-encode commit-prefix-scope: %w", err)
	}
	return buf.String(), nil
}

// readStagedFiles reads each listed path under dir into a map for the sanitizer.
// Missing paths are skipped (the staged list is authoritative but a path may have
// been removed by a transform). Files containing a NUL byte are treated as binary
// and returned in skipped (NOT scanned — the deterministic scan is a text scan);
// the caller surfaces them so a binary text-scan bypass is never silent.
func readStagedFiles(dir string, paths []string) (files map[string]string, skipped []string, err error) {
	files = make(map[string]string, len(paths))
	for _, rel := range paths {
		b, rerr := os.ReadFile(filepath.Join(dir, rel))
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			return nil, nil, fmt.Errorf("read staged %s: %w", rel, rerr)
		}
		if bytes.IndexByte(b, 0) >= 0 {
			skipped = append(skipped, rel)
			continue
		}
		files[rel] = string(b)
	}
	return files, skipped, nil
}
