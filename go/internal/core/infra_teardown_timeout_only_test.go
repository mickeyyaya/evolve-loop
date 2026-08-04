package core

// RED contract for cycle-1267 Task 2
// (`verify-infra-teardown-predicate-consolidation`, inbox
// infra-teardown-predicate-single-source, w=0.86) — the acceptance criterion
// that has NO pin today:
//
//	"NO site that is timeout-only or transient-only was incorrectly widened to
//	 the union predicate"
//
// The item calls this its whole risk: "this item's whole risk is a blind widen
// of a timeout-only or transient-only site into the union."
//
// The transient-ONLY half is already pinned
// (infra_teardown_single_source_test.go: TestIsTransientBridgeError_StaysTransientOnly),
// and the uniqueness of the union spelling is pinned there too. The TIMEOUT-only
// half is not pinned anywhere: the two sites the inbox item and the cycle-1267
// scout report both name —
//
//	failure_learning.go  writePhaseFailureDiag        (exit-code 81 mapping)
//	failure_hook.go      adviseOnUnclassifiedFailure  (pane-classification gate)
//
// — check ErrArtifactTimeout ALONE and must keep doing so. Nothing stops a
// future "consolidation" from folding either into IsInfraTeardownError, at
// which point a quota bounce (ErrTransientBridgeFailure) would be recorded as
// exit 81 and fed to the fatal-signature classifier as if it carried a
// diagnosable pane. This file makes that regression fail.
//
// This is a behaviour-PRESERVING contract: both pins encode "you did not change
// semantics", which is exactly what the item asks for ("no verdict changes").

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

// TestWritePhaseFailureDiag_TimeoutOnlyNotWidened — AC9, the behavioural pin on
// the timeout-only exit-code mapping. Exit 81 is the artifact-wait timeout's
// code specifically; a transient bridge failure is exit 80/85/86/124 and must
// NOT be relabelled 81, or every quota bounce in the failure-diag record
// becomes indistinguishable from a stalled agent.
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
			writePhaseFailureDiag(ws, "build", 1267, tc.err, 1, now)

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
}

// TestTimeoutOnlySites_NotWidenedToUnion — AC10, the structural half. AC9 pins
// the one timeout-only site whose behaviour is observable from a unit test;
// adviseOnUnclassifiedFailure's gate is reachable only through a fully wired
// orchestrator with a real failure adviser, so it is pinned structurally
// instead: its body must still mention ErrArtifactTimeout and must NOT mention
// the transient sentinel, its transient-only component, or the union helper.
//
// This is a NON-degenerate scan for the same reason
// TestInfraTeardownUnion_SpelledExactlyOnce is: it can only be satisfied by NOT
// adding text. Adding the magic string is precisely what makes it fail, so it
// cannot be gamed the way a "source contains X" predicate can.
func TestTimeoutOnlySites_NotWidenedToUnion(t *testing.T) {
	sites := []timeoutOnlySite{
		{
			file: "failure_hook.go", fn: "adviseOnUnclassifiedFailure",
			why: "only the timeout family carries a pane worth classifying; a transient bounce " +
				"has no diagnosable final pane, so feeding it to the fatal-signature detector " +
				"would promote noise into the instinct store",
		},
		{
			file: "failure_learning.go", fn: "writePhaseFailureDiag",
			why: "exit 81 is the artifact-timeout code specifically — see AC9",
		},
	}

	for _, site := range sites {
		t.Run(site.file+":"+site.fn, func(t *testing.T) {
			body, err := funcBodyText(site.file, site.fn)
			if err != nil {
				t.Fatalf("locate %s in %s: %v", site.fn, site.file, err)
			}
			if !strings.Contains(body, "ErrArtifactTimeout") {
				t.Fatalf("%s no longer references ErrArtifactTimeout — it was the timeout-ONLY gate this "+
					"pin exists to protect; if the gate genuinely moved, move this pin with it", site.fn)
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
