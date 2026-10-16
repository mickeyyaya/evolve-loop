package landing

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func (l *Landing) WriteBinding(dir string, b dossier.ShipBinding) error {
	path := filepath.Join(dir, dossier.ShipBindingFile)
	if err := writeJSON(path, "ship-binding.*.tmp", b); err != nil {
		l.warn("Landing.WriteBinding", CodeBindingWriteFailed, "ship-binding.json write failed: "+err.Error(),
			map[string]string{shiperr.StepKey: stepBinding, "path": path, "err": err.Error()})
		return err
	}
	return nil
}

func writeJSON(path, pattern string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	var tmp *os.File
	if err == nil {
		tmp, err = os.CreateTemp(dir, pattern)
	}
	if err != nil {
		return err
	}
	return commitTemp(tmp, append(buf, '\n'), path)
}

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
