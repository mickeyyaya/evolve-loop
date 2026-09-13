package landing

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// WriteBinding writes the landing witness <dir>/ship-binding.json atomically
// (MkdirAll 0o755 → MarshalIndent + newline → a temp file → rename). The
// shared dossier type, not an inline map: every reader of the sidecar (the
// dossier's delivery record, the idempotency check, the lost-landing floor)
// binds to the same declaration, so a field rename is a compile error rather
// than a silently empty record. Any failure is SHIP_LANDING_BINDING_WRITE_FAILED
// AND the error is returned — the callers keep their WARN log lines; the push
// already landed.
func (l *Landing) WriteBinding(dir string, b dossier.ShipBinding) error {
	path := filepath.Join(dir, dossier.ShipBindingFile)
	if err := writeBinding(dir, path, b); err != nil {
		l.warn("Landing.WriteBinding", CodeBindingWriteFailed, "ship-binding.json write failed: "+err.Error(),
			map[string]string{shiperr.StepKey: stepBinding, "path": path, "err": err.Error()})
		return err
	}
	return nil
}

func writeBinding(dir, path string, b dossier.ShipBinding) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(b, "", "  ")
	var tmp *os.File
	if err == nil { // MarshalIndent cannot fail for dossier.ShipBinding (plain fields only — TestWriteBinding_ShipBindingCannotFailToMarshal guards the shape); its check folds into CreateTemp's chain so every line stays reachable
		tmp, err = os.CreateTemp(dir, "ship-binding.*.tmp")
	}
	if err != nil {
		return err
	}
	return commitTemp(tmp, append(buf, '\n'), path)
}

// commitTemp writes buf to the temp file, closes it and renames it into
// place. On the first failure the temp file is removed and that error
// returned — the Write and Close branches folded into one chain. A failed
// rename leaves the temp file behind, as before the move.
func commitTemp(tmp *os.File, buf []byte, path string) error {
	_, err := tmp.Write(buf)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
