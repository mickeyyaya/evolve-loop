package gotypes

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

const indexVersion = 2

type encodeFunc func(w io.Writer, v any) error

func encodeGob(w io.Writer, v any) error { return gob.NewEncoder(w).Encode(v) }

type callKind uint8

const (
	kindStatic callKind = iota
	kindInterface
	kindFuncValue
)

var kindNames = [...]string{kindStatic: "static", kindInterface: "interface", kindFuncValue: "func-value"}

func (k callKind) String() string { return kindNames[k] }

type funcRecord struct {
	Package, Recv, Name string
	File, Line          int
	Interface, Module   bool
}

func (f funcRecord) qualified() string {
	if f.Recv == "" {
		return f.Package + "." + f.Name
	}
	return f.Package + "." + f.Recv + "." + f.Name
}

func (f funcRecord) answers(symbol string) bool {
	if f.Recv == "" {
		return symbol == f.Name || symbol == f.qualified()
	}
	return symbol == f.Name || symbol == f.Recv+"."+f.Name || symbol == f.qualified()
}

type callRecord struct {
	Callee, Caller     int
	Value              string
	File, Line, Column int
	Kind               callKind
}

type declRecord struct {
	Name, Kind string
	Line       int
}

type implementsRecord struct {
	Concrete, Interface int
}

type indexData struct {
	Version    int
	Files      []string
	Funcs      []funcRecord
	Calls      []callRecord
	Decls      map[int][]declRecord
	Implements []implementsRecord
	Warnings   []string
}

func writeIndex(path string, data indexData, encode encodeFunc) error {
	var buf bytes.Buffer
	if err := encode(&buf, data); err != nil {
		return fmt.Errorf("gotypes: encode the index: %w", err)
	}
	if err := atomicwrite.Bytes(path, buf.Bytes()); err != nil {
		return fmt.Errorf("gotypes: write the index %s: %w", path, err)
	}
	return nil
}

func readIndex(path string) (indexData, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return indexData{}, fmt.Errorf("%w: the gotypes index %s does not exist (the boundary builds it)", codeindex.ErrAbsent, path)
	}
	if err != nil {
		return indexData{}, fmt.Errorf("gotypes: read the index %s: %w", path, err)
	}
	var data indexData
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&data); err != nil {
		return indexData{}, fmt.Errorf("gotypes: decode the index %s: %w", path, err)
	}
	if data.Version != indexVersion {
		return indexData{}, fmt.Errorf("%w: the gotypes index %s has version %d, want %d (the boundary rebuilds it)", codeindex.ErrAbsent, path, data.Version, indexVersion)
	}
	return data, nil
}

func IndexPath(indexDir string) string {
	clean := filepath.Clean(indexDir)
	return filepath.Join(filepath.Dir(clean), filepath.Base(clean)+"-gotypes", "index.gob")
}
