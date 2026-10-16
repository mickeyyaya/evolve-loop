// Package failurediag writes the phase failure diagnostic sidecar and classifies prompt-delivery failures.
// See docs/architecture/packages/internal-core-failurediag.md.
package failurediag

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeSidecarWriteFailed signalcenter.Code = "FAILUREDIAG_SIDECAR_WRITE_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleFailureDiag, CodeSidecarWriteFailed, "the phase's <phase>-failure-diag.json could not be written (temp write or rename); the reason names the step and the error, fields name the path; the phase abort proceeds unchanged and the diagnosis is lost from disk")
}

type Sidecar struct {
	Phase           string `json:"phase"`
	Cycle           int    `json:"cycle"`
	ErrorMessage    string `json:"error_message"`
	DeliveryFailure string `json:"delivery_failure"`
	ExitCode        int    `json:"exit_code"`
	AttemptCount    int    `json:"attempt_count"`
	Timestamp       string `json:"timestamp"`
}

func SidecarPath(workspace, phase string) string {
	return filepath.Join(workspace, phase+"-failure-diag.json")
}

type Writer struct {
	now       func() time.Time
	isTimeout func(error) bool
	signals   func() *signalcenter.Center
}

type Option func(*Writer)

func WithSignals(c func() *signalcenter.Center) Option {
	return func(w *Writer) {
		if c != nil {
			w.signals = c
		}
	}
}

func NewWriter(now func() time.Time, isTimeout func(error) bool, opts ...Option) *Writer {
	w := &Writer{now: now, isTimeout: isTimeout}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Writer) SignalsWired() bool { return w.center() != nil }

func (w *Writer) center() *signalcenter.Center {
	if w.signals == nil {
		return nil
	}
	return w.signals()
}

func (w *Writer) Write(workspace, phase string, cycle int, phaseErr error, attempts int) {
	data, _ := json.Marshal(w.diagnose(phase, cycle, phaseErr, attempts))
	path := SidecarPath(workspace, phase)
	fields := map[string]string{"path": path}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		w.warn("Writer.Write", cycle, phase, CodeSidecarWriteFailed, "failure-diag temp write failed: "+err.Error(), fields)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		w.warn("Writer.Write", cycle, phase, CodeSidecarWriteFailed, "failure-diag rename failed: "+err.Error(), fields)
	}
}

func (w *Writer) diagnose(phase string, cycle int, phaseErr error, attempts int) Sidecar {
	return Sidecar{
		Phase:           phase,
		Cycle:           cycle,
		ErrorMessage:    phaseErr.Error(),
		DeliveryFailure: DeliveryFailureCause(phaseErr, w.isTimeout),
		ExitCode:        exitCode(phaseErr, w.isTimeout),
		AttemptCount:    attempts,
		Timestamp:       w.now().UTC().Format(time.RFC3339),
	}
}

func exitCode(err error, isTimeout func(error) bool) int {
	if isTimeout(err) {
		return ExitCodeArtifactTimeout
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func (w *Writer) warn(origin string, cycle int, phase string, code signalcenter.Code, reason string, fields map[string]string) {
	w.center().Emit(signalcenter.Event{
		Cycle: cycle, Phase: phase, Module: signalcenter.ModuleFailureDiag, Origin: origin, Kind: signalcenter.KindFailureDiagWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
