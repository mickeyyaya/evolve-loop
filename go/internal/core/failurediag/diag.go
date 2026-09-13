// Package failurediag is unit 02 of the component breakdown (ADR-0103): the
// phase failure diagnostic — the <phase>-failure-diag.json sidecar written
// when a mandatory phase aborts — and the delivery-failure classifier the
// sidecar and the retro relaunch key on. The orchestrator injects its clock
// and its artifact-timeout gate (a predicate, so the unit never imports the
// core sentinel); the unit's own failure mode is a failurediag.warning signal
// under module failurediag. Design:
// docs/architecture/decomposition/02-failure-diagnostics.md.
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

// CodeSidecarWriteFailed is the unit's one code: the sidecar could not be
// landed (temp write or rename); the phase abort proceeds unchanged.
const CodeSidecarWriteFailed signalcenter.Code = "FAILUREDIAG_SIDECAR_WRITE_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleFailureDiag, CodeSidecarWriteFailed, "the phase's <phase>-failure-diag.json could not be written (temp write or rename); the reason names the step and the error, fields name the path; the phase abort proceeds unchanged and the diagnosis is lost from disk")
}

// Sidecar is the declared on-disk contract of <phase>-failure-diag.json:
// field order IS wire order (compact json.Marshal, no omitempty, every field
// always present).
type Sidecar struct {
	Phase           string `json:"phase"`
	Cycle           int    `json:"cycle"`
	ErrorMessage    string `json:"error_message"`    // phaseErr.Error() verbatim
	DeliveryFailure string `json:"delivery_failure"` // DeliveryFailureCause, or ""
	ExitCode        int    `json:"exit_code"`        // 81 iff the timeout gate; else *exec.ExitError.ExitCode(); else 1
	AttemptCount    int    `json:"attempt_count"`
	Timestamp       string `json:"timestamp"` // now().UTC() at second precision
}

// SidecarPath is where a phase's failure diagnostic lives in a cycle workspace.
func SidecarPath(workspace, phase string) string {
	return filepath.Join(workspace, phase+"-failure-diag.json")
}

// Writer renders and lands the sidecar. Construct it once per orchestrator;
// every collaborator is read live (the clock through a closure, the Center
// through an accessor), so an option applied later still takes.
type Writer struct {
	now       func() time.Time
	isTimeout func(error) bool
	signals   func() *signalcenter.Center
}

// Option configures a Writer at construction (functional options).
type Option func(*Writer)

// WithSignals installs the accessor of the Signal Center the unit reports its
// own failure mode through — an accessor, not a value, because the
// orchestrator's Center is itself an option tests apply after construction.
// A nil accessor, or one returning nil, is the Null Object (tests only; the
// production roots always wire a Center, and SignalsWired proves it).
func WithSignals(c func() *signalcenter.Center) Option {
	return func(w *Writer) {
		if c != nil {
			w.signals = c
		}
	}
}

// NewWriter injects the orchestrator's clock and its artifact-timeout gate —
// the ONE predicate that decides both exit_code 81 and whether a delivery
// cause may be attributed. isTimeout must be non-nil: a nil gate panics at
// first use, never "nothing is a timeout".
func NewWriter(now func() time.Time, isTimeout func(error) bool, opts ...Option) *Writer {
	w := &Writer{now: now, isTimeout: isTimeout}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// SignalsWired reports whether a Center is reachable now — the root's wiring proof.
func (w *Writer) SignalsWired() bool { return w.center() != nil }

func (w *Writer) center() *signalcenter.Center {
	if w.signals == nil {
		return nil
	}
	return w.signals()
}

// Write renders the Sidecar and lands it at SidecarPath through <path>.tmp
// and os.Rename (0o644). Best-effort: a temp-write or rename failure is one
// FAILUREDIAG_SIDECAR_WRITE_FAILED WARN; the caller's phase error is never
// masked, and nothing is returned.
func (w *Writer) Write(workspace, phase string, cycle int, phaseErr error, attempts int) {
	// Strings and ints only: encoding/json has no error path for this shape
	// (it fails on unsupported types, non-finite floats, cycles and Marshaler
	// errors), so the pre-extraction marshal branch could never fire and is
	// gone rather than excused (ADR-0103 item 3). A float field added later
	// must bring the branch back with a code (unit 01's precedent).
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

// diagnose is the pure diagnosis over the injected clock and gate.
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

// exitCode projects the abort's exit-code class: the artifact-timeout code
// for the gate, a subprocess's own code, else 1.
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

// warn is the unit's one producer: a failurediag.warning WARN under module
// failurediag. A nil Center is the Null Object (Emit on nil is a no-op).
func (w *Writer) warn(origin string, cycle int, phase string, code signalcenter.Code, reason string, fields map[string]string) {
	w.center().Emit(signalcenter.Event{
		Cycle: cycle, Phase: phase, Module: signalcenter.ModuleFailureDiag, Origin: origin, Kind: signalcenter.KindFailureDiagWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
