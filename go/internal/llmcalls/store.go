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
	var result ImportResult
	err := flock.WithPathLock(destination, func() error {
		identities := make(map[string]struct{})
		if err := scanRaw(destination, func(line []byte) {
			if id, ok := recordIdentity(line); ok {
				identities[id] = struct{}{}
			}
		}); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("index destination model-attempt ledger %q: %w", destination, err)
		}

		var writeErr error
		readErr := scanRaw(source, func(line []byte) {
			if writeErr != nil {
				return
			}
			id, ok := recordIdentity(line)
			if !ok {
				result.Skipped++
				return
			}
			if _, exists := identities[id]; exists {
				result.Duplicates++
				return
			}
			if err := appendLineLocked(destination, line); err != nil {
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
	if err != nil {
		return result, err
	}
	return result, nil
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

func recordIdentity(line []byte) (string, bool) {
	var rec Record
	if err := json.Unmarshal(line, &rec); err != nil || !validRecord(rec) {
		return "", false
	}
	if rec.CallID != "" {
		return "call:" + rec.CallID, true
	}
	digest := sha256.Sum256(line)
	return "legacy:" + hex.EncodeToString(digest[:]), true
}

func validRecord(rec Record) bool { return rec.Phase != "" }
