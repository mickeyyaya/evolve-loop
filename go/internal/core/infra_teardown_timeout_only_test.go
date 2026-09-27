package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWritePhaseFailureDiag_TimeoutOnlyNotWidened(t *testing.T) {
	at := time.Date(2026, 8, 4, 7, 0, 0, 0, time.UTC)
	now := func() time.Time { return at }

	for _, tc := range []struct {
		name     string
		err      error
		wantCode int
		why      string
	}{
		{
			name: "artifact timeout keeps its own code", err: ErrArtifactTimeout, wantCode: 81,
			why: "81 IS the artifact-wait timeout code; this half must stay green across any consolidation",
		},
		{
			name: "wrapped artifact timeout keeps its own code",
			err:  fmt.Errorf("bridge dispatch: %w", ErrArtifactTimeout), wantCode: 81,
			why: "the sentinel is matched through wrapping, as everywhere else in this package",
		},
		{
			name: "transient bridge failure is NOT relabelled 81", err: ErrTransientBridgeFailure, wantCode: 1,
			why: "this site is TIMEOUT-ONLY; widening it to IsInfraTeardownError would record every quota " +
				"bounce as a stalled-agent timeout — the exact blind-widen the item warns about",
		},
		{
			name: "plain logic error is NOT relabelled 81", err: errors.New("index out of range"), wantCode: 1,
			why: "a defect is neither sentinel and must fall through to the default code",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			(&Orchestrator{now: now}).writePhaseFailureDiag(ws, "build", 1267, tc.err, 1)

			raw, err := os.ReadFile(filepath.Join(ws, "build-failure-diag.json"))
			if err != nil {
				t.Fatalf("read failure diag: %v", err)
			}
			var diag struct {
				ExitCode int `json:"exit_code"`
			}
			if jerr := json.Unmarshal(raw, &diag); jerr != nil {
				t.Fatalf("parse failure diag: %v\n%s", jerr, raw)
			}
			if diag.ExitCode != tc.wantCode {
				t.Errorf("writePhaseFailureDiag(%v) recorded exit_code=%d, want %d — %s",
					tc.err, diag.ExitCode, tc.wantCode, tc.why)
			}
		})
	}
}

// timeoutOnlySite names one function that the inbox item explicitly excludes
// from the union predicate, together with why it is excluded.
type timeoutOnlySite struct {
	file string
	fn   string
	why  string
	// gate is the timeout-only expression the body must reference; "" means
	// the sentinel itself. internal/core/failurediag cannot import either
	// sentinel, so the injected isArtifactTimeout gate is what the pin reads.
	gate string
}

func TestTimeoutOnlySites_NotWidenedToUnion(t *testing.T) {
	sites := []timeoutOnlySite{
		{
			file: "failure_hook.go", fn: "adviseOnUnclassifiedFailure",
			why: "only the timeout family carries a pane worth classifying; a transient bounce " +
				"has no diagnosable final pane, so feeding it to the fatal-signature detector " +
				"would promote noise into the instinct store",
		},
		{
			file: "errors.go", fn: "isArtifactTimeout",
			why: "exit 81 is the artifact-timeout code specifically — see AC9; this is the ONE gate unit 02 injects",
		},
		{
			file: "failure_diag.go", fn: "wiredFailureDiag", gate: "isArtifactTimeout",
			why: "the sidecar writer is constructed with the timeout-only gate, never the union",
		},
		{
			file: "failure_diag.go", fn: "DeliveryFailureCause", gate: "isArtifactTimeout",
			why: "delivery attribution is gated on the timeout family alone",
		},
	}

	for _, site := range sites {
		t.Run(site.file+":"+site.fn, func(t *testing.T) {
			body, err := funcBodyText(site.file, site.fn)
			if err != nil {
				t.Fatalf("locate %s in %s: %v", site.fn, site.file, err)
			}
			gate := site.gate
			if gate == "" {
				gate = "ErrArtifactTimeout"
			}
			if !strings.Contains(body, gate) {
				t.Fatalf("%s no longer references %s — it was the timeout-ONLY gate this "+
					"pin exists to protect; if the gate genuinely moved, move this pin with it", site.fn, gate)
			}
			for _, banned := range []string{"ErrTransientBridgeFailure", "isTransientBridgeError", "IsInfraTeardownError"} {
				if strings.Contains(body, banned) {
					t.Errorf("%s (%s) now references %s — a TIMEOUT-ONLY site was widened to the union. %s",
						site.fn, site.file, banned, site.why)
				}
			}
		})
	}
}

// funcBodyText returns the source text of the named function's body from the
// named non-test file in this package's directory.
func funcBodyText(file, fnName string) (string, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != fnName || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Body.Pos()).Offset
		end := fset.Position(fn.Body.End()).Offset
		if start < 0 || end > len(data) || start >= end {
			return "", fmt.Errorf("body offsets out of range for %s", fnName)
		}
		return string(data[start:end]), nil
	}
	return "", fmt.Errorf("function %s not found in %s", fnName, file)
}
