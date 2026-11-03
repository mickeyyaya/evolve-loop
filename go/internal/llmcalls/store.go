package llmcalls

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

const maxRecordBytes = 1 << 20

const maxScanBytes = maxRecordBytes + 1

type ReadResult struct {
	Records []Record
	Skipped int
}

type ImportResult struct {
	Imported   int
	Duplicates int
	Skipped    int
}

func Path(workspace string) string { return filepath.Join(workspace, Filename) }

func AppendWorkspace(workspace string, rec Record) error { return Append(Path(workspace), rec) }

func Append(path string, rec Record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal model-attempt record: %w", err)
	}
	if len(line) > maxRecordBytes {
		return fmt.Errorf("model-attempt record is %d bytes; limit is %d", len(line), maxRecordBytes)
	}
	if err := flock.WithPathLock(path, func() error { return appendLineLocked(path, line) }); err != nil {
		return fmt.Errorf("append model-attempt ledger %q: %w", path, err)
	}
	return nil
}

func appendLineLocked(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	prefix, err := separatorFor(f)
	if err != nil {
		return err
	}
	payload := make([]byte, 0, len(prefix)+len(line)+1)
	payload = append(payload, prefix...)
	payload = append(payload, line...)
	payload = append(payload, '\n')
	_, err = f.Write(payload)
	return err
}

func separatorFor(f *os.File) ([]byte, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, nil
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], stat.Size()-1); err != nil {
		return nil, err
	}
	if last[0] == '\n' {
		return nil, nil
	}
	return []byte{'\n'}, nil
}

func Read(path string) (ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return ReadResult{}, err
	}
	defer func() { _ = f.Close() }()
	var out ReadResult
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil || !validRecord(rec) {
			out.Skipped++
			continue
		}
		out.Records = append(out.Records, rec)
	}
	if err := scanner.Err(); err != nil {
		return out, fmt.Errorf("read model-attempt ledger %q: %w", path, err)
	}
	return out, nil
}

func ReadWorkspace(workspace string) (ReadResult, error) { return Read(Path(workspace)) }

func Import(destination, source string) (ImportResult, error) {
	return importWith(destination, source, func(_ Record, line []byte) ([]byte, error) { return line, nil })
}

func ImportTagged(destination, source, runID, profile string) (ImportResult, error) {
	return importWith(destination, source, func(rec Record, _ []byte) ([]byte, error) {
		rec.RunID, rec.Profile = runID, profile
		return json.Marshal(rec)
	})
}

func importWith(destination, source string, rewrite func(Record, []byte) ([]byte, error)) (ImportResult, error) {
	var result ImportResult
	err := flock.WithPathLock(destination, func() error {
		identities, err := indexIdentities(destination)
		if err != nil {
			return fmt.Errorf("index destination model-attempt ledger %q: %w", destination, err)
		}

		var writeErr error
		readErr := scanRaw(source, func(line []byte) {
			if writeErr != nil {
				return
			}
			rec, ok := parseRecord(line)
			if !ok {
				result.Skipped++
				return
			}
			id := identityOf(rec, line)
			if _, exists := identities[id]; exists {
				result.Duplicates++
				return
			}
			out, err := rewrite(rec, line)
			if err == nil {
				err = appendLineLocked(destination, out)
			}
			if err != nil {
				writeErr = err
				return
			}
			identities[id] = struct{}{}
			result.Imported++
		})
		if readErr != nil {
			return fmt.Errorf("read source model-attempt ledger %q: %w", source, readErr)
		}
		if writeErr != nil {
			return fmt.Errorf("append imported model-attempt ledger %q: %w", destination, writeErr)
		}
		return nil
	})
	return result, err
}

func indexIdentities(path string) (map[string]struct{}, error) {
	identities := make(map[string]struct{})
	err := scanRaw(path, func(line []byte) {
		if rec, ok := parseRecord(line); ok {
			identities[identityOf(rec, line)] = struct{}{}
		}
	})
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return identities, err
}

func scanRaw(path string, visit func([]byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) > 0 {
			visit(append([]byte(nil), line...))
		}
	}
	return scanner.Err()
}

func parseRecord(line []byte) (Record, bool) {
	var rec Record
	if err := json.Unmarshal(line, &rec); err != nil || !validRecord(rec) {
		return Record{}, false
	}
	return rec, true
}

func identityOf(rec Record, line []byte) string {
	if rec.CallID != "" {
		return "call:" + rec.CallID
	}
	digest := sha256.Sum256(line)
	return "legacy:" + hex.EncodeToString(digest[:])
}

func validRecord(rec Record) bool { return rec.Phase != "" }
