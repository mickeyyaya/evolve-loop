package stageevidence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	WorkingDirName   = "stage-evidence"
	FaultWriteFailed = "write-failed"
	counterPrefix    = "counter."
	faultField       = "fault"
	buildField       = "build"
	workingSuffix    = ".ndjson"
	workingMode      = 0o644
	workingDirMode   = 0o755
)

func WorkingPath(evolveDir string, scope ScopeKey) string {
	return filepath.Join(evolveDir, WorkingDirName, scope.String()+workingSuffix)
}

func ParseWorkingName(name string) (ScopeKey, bool) {
	label, suffixed := strings.CutSuffix(name, workingSuffix)
	if !suffixed {
		return ScopeKey{}, false
	}
	return ParseScope(label)
}

type RecorderConfig struct {
	EvolveDir string
	Scope     ScopeKey
	Session   string
	Build     string
	Lock      func(path string) (release func(), err error)
	Version   func(key string) (string, error)
	OnFault   func(error)
}

type Recorder struct {
	cfg      RecorderConfig
	mu       sync.Mutex
	counters map[string]int
	versions map[string]string
	opened   bool
	closed   bool
	faulted  bool
}

func NewRecorder(cfg RecorderConfig) *Recorder {
	return &Recorder{cfg: cfg, counters: map[string]int{}, versions: map[string]string{}}
}

func (r *Recorder) Record(key string, fields map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[key]++
	kind := KindDecision
	if r.closed {
		kind = KindLate
	}
	version, err := r.versionOf(key)
	if err != nil {
		r.fault(fmt.Errorf("stageevidence: decision version of %s: %w", key, err))
		return
	}
	withBuild := make(map[string]string, len(fields)+1)
	for k, v := range fields {
		withBuild[k] = v
	}
	withBuild[buildField] = r.cfg.Build
	r.fault(r.write(Record{Key: key, Version: version, Kind: kind, Fields: withBuild}))
}

func (r *Recorder) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	if !r.opened && len(r.counters) == 0 {
		return
	}
	fields := make(map[string]string, len(r.counters)+1)
	for key, n := range r.counters {
		fields[counterPrefix+key] = strconv.Itoa(n)
	}
	if r.faulted {
		fields[faultField] = FaultWriteFailed
	}
	r.fault(r.write(Record{Kind: KindClose, Fields: fields}))
}

func (r *Recorder) versionOf(key string) (string, error) {
	if v, ok := r.versions[key]; ok {
		return v, nil
	}
	v, err := r.cfg.Version(key)
	if err != nil {
		return "", err
	}
	r.versions[key] = v
	return v, nil
}

func (r *Recorder) fault(err error) {
	if err == nil {
		return
	}
	r.faulted = true
	r.cfg.OnFault(err)
}

func (r *Recorder) write(draft Record) error {
	path := WorkingPath(r.cfg.EvolveDir, r.cfg.Scope)
	if err := os.MkdirAll(filepath.Dir(path), workingDirMode); err != nil {
		return fmt.Errorf("stageevidence: working dir: %w", err)
	}
	release, err := r.cfg.Lock(path + ".lock")
	if err != nil {
		return fmt.Errorf("stageevidence: lock working file: %w", err)
	}
	defer release()
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stageevidence: read working file: %w", err)
	}
	var lines []byte
	last := lastRecord(raw)
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		lines = append(lines, '\n')
	}
	if !r.opened {
		open := r.link(last, Record{Kind: KindOpen})
		lines = appendLine(lines, open)
		last = open
	}
	rec := r.link(last, draft)
	lines = appendLine(lines, rec)
	if err := appendBytes(path, lines); err != nil {
		return err
	}
	r.opened = true
	return nil
}

func (r *Recorder) link(prev, draft Record) Record {
	draft.Scope, draft.ID, draft.Session, draft.Seq = r.cfg.Scope.Kind, r.cfg.Scope.ID, r.cfg.Session, prev.Seq+1
	return Link(prev.Hash, draft)
}

func appendBytes(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, workingMode)
	if err != nil {
		return fmt.Errorf("stageevidence: open working file: %w", err)
	}
	_, writeErr := f.Write(body)
	return errors.Join(writeErr, f.Close())
}
