//go:build acs

// Package cycle1460 holds the cycle-1460 ACS predicates for the fleet-assigned
// inbox item tokenopt-role-scoped-instruction-digests.
//
// Two tasks (see .evolve/runs/cycle-1460/scout-report.md and api-contract.md):
//
//   - digest-materialize-role-instructions: the pure cycle-1391 projector
//     (digest.ProjectDigest) has no production caller. This task adds
//     digest.Materialize/Result/Outcome plus the runner-side integration seam
//     so BaseRunner.Run derives the dispatched instruction body from the
//     role-tagged SSOT source, excludes untagged and other-role content, and
//     fails BEFORE bridge.Launch on an unterminated marker.
//   - digest-shadow-size-parity: digest.ShadowRecord/NewShadowRecord plus
//     runner.FormatDigestShadowLog record full-versus-digest byte counts and a
//     parity verdict for every dispatch, and no empty, malformed, or
//     non-reducing projection may claim a saving or replace the live prompt.
//     No profile default flip happens until that baseline exists.
//
// Every predicate below drives real production code — BaseRunner.Run, the
// digest package's exported functions, or the real profiles loader. None is a
// source grep (the cycle-85 degenerate-predicate failure mode); predicate 006
// carries an explicit config-check waiver for its declarative half.
package cycle1460

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- shared harness --------------------------------------------------------
//
// The harness constructs a REAL *runner.BaseRunner via the exported
// runner.New(runner.Options{...}) constructor and calls its real Run method.
// That is the reachability proof the house rules demand: the seam under test
// must be reached from the production dispatch path, not called directly.

// recHooks is a minimal runner.Hooks implementation that records the agent
// body BaseRunner hands to ComposePrompt — i.e. exactly the instruction block
// that would be dispatched.
type recHooks struct {
	phase        string
	composedBody string
	composeCalls int
}

func (h *recHooks) PhaseName() string       { return h.phase }
func (h *recHooks) AgentPromptName() string { return "evolve-" + h.phase }
func (h *recHooks) ArtifactFilename(req core.PhaseRequest) string {
	return h.phase + "-report.md"
}
func (h *recHooks) DefaultModel() string { return "sonnet" }
func (h *recHooks) ComposePrompt(agentBody string, req core.PhaseRequest) string {
	h.composeCalls++
	h.composedBody = agentBody
	return "COMPOSED PROMPT"
}
func (h *recHooks) Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, ""
}

// recBridge counts Launch calls so a predicate can prove a fail-closed path
// aborted BEFORE dispatch (launches == 0) rather than merely returning FAIL.
type recBridge struct {
	launches int
	artifact string
}

func (b *recBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.launches++
	if b.artifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(b.artifact), 0o644)
	}
	return core.BridgeResponse{Stdout: b.artifact}, nil
}

func (b *recBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func agentFS(agentName, body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/" + agentName + ".md": &fstest.MapFile{
			Data: []byte("---\nname: " + agentName + "\n---\n" + body),
		},
	})
}

// taggedDoc is the representative SSOT fixture: untagged prose, one block
// tagged for role=scout, one block tagged for role=build.
const taggedDoc = `UNTAGGED-PREAMBLE cross-cutting ship-gate detail scout never acts on.
<!-- digest:role=scout -->
SCOUT-ONLY-INSTRUCTIONS
<!-- /digest -->
<!-- digest:role=build -->
BUILD-ONLY-INSTRUCTIONS
<!-- /digest -->
UNTAGGED-TRAILER more cross-cutting detail.
`

// unterminatedDoc opens a role marker that never closes before EOF.
const unterminatedDoc = `<!-- digest:role=scout -->
SCOUT-ONLY-INSTRUCTIONS with no closing marker.
`

// runPhase drives a real BaseRunner.Run for phase with the given agent doc and
// returns the recorded hooks, bridge, diag output, response, and error.
func runPhase(t *testing.T, phase, doc string) (*recHooks, *recBridge, string, core.PhaseResponse, error) {
	t.Helper()
	hk := &recHooks{phase: phase}
	br := &recBridge{artifact: "# " + phase + " artifact\n"}
	var diagBuf bytes.Buffer
	r := runner.New(runner.Options{
		Hooks:   hk,
		Bridge:  br,
		Prompts: agentFS("evolve-"+phase, doc),
		Diag:    log.Console{Out: &diagBuf, Err: &diagBuf},
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1460, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	return hk, br, diagBuf.String(), resp, err
}

// --- digest-materialize-role-instructions ----------------------------------

// TestC1460_001_DigestMaterializationInRunnerUsesRoleScopedDigest is the
// primary wiring predicate for AC "The runner derives its injected instruction
// block from a tagged SSOT source for the requested role". It drives the real
// BaseRunner.Run dispatch path and asserts the body handed to ComposePrompt —
// the block that actually reaches the CLI — is the role's projection.
//
// A pass-through (no-op) integration fails this: the composed body would still
// carry the untagged preamble, so the equality assertion below breaks.
func TestC1460_001_DigestMaterializationInRunnerUsesRoleScopedDigest(t *testing.T) {
	hk, _, _, _, _ := runPhase(t, "scout", taggedDoc)

	if hk.composeCalls != 1 {
		t.Fatalf("ComposePrompt called %d times, want exactly 1 (the runner must reach the prompt-composition seam once per dispatch)", hk.composeCalls)
	}
	if !strings.Contains(hk.composedBody, "SCOUT-ONLY-INSTRUCTIONS") {
		t.Errorf("dispatched body lost the role=scout block; got %q", hk.composedBody)
	}
	// The projection is the WHOLE body — no blending of digest and full source.
	want, err := digest.ProjectDigest([]byte(taggedDoc), "scout")
	if err != nil {
		t.Fatalf("fixture is malformed: %v", err)
	}
	if strings.TrimSpace(hk.composedBody) != strings.TrimSpace(string(want)) {
		t.Errorf("dispatched body is not exactly the role-scoped projection\n got: %q\nwant: %q", hk.composedBody, string(want))
	}
	if len(hk.composedBody) >= len(taggedDoc) {
		t.Errorf("dispatched body (%d bytes) is not smaller than the SSOT source (%d bytes) — no projection occurred", len(hk.composedBody), len(taggedDoc))
	}
}

// TestC1460_002_RoleScopedDigestExcludesUntaggedAndOtherRoleContent is the
// cross-role isolation predicate for AC "Untagged and other-role content never
// reaches the digest". It asserts on the live dispatch body (runner path) AND
// on digest.Materialize's classified Result, so neither layer can leak.
func TestC1460_002_RoleScopedDigestExcludesUntaggedAndOtherRoleContent(t *testing.T) {
	hk, _, _, _, _ := runPhase(t, "scout", taggedDoc)

	for _, leak := range []string{"BUILD-ONLY-INSTRUCTIONS", "UNTAGGED-PREAMBLE", "UNTAGGED-TRAILER"} {
		if strings.Contains(hk.composedBody, leak) {
			t.Errorf("dispatched body for role=scout leaked %q — projection is copying the doc, not projecting; got %q", leak, hk.composedBody)
		}
	}

	res := digest.Materialize([]byte(taggedDoc), "scout")
	if res.Outcome != digest.OutcomeMatched {
		t.Fatalf("Materialize(taggedDoc, scout).Outcome = %v, want OutcomeMatched", res.Outcome)
	}
	if res.Err != nil {
		t.Errorf("Materialize returned Err=%v on a well-formed source, want nil", res.Err)
	}
	for _, leak := range []string{"BUILD-ONLY-INSTRUCTIONS", "UNTAGGED-PREAMBLE", "UNTAGGED-TRAILER"} {
		if strings.Contains(string(res.Digest), leak) {
			t.Errorf("Materialize digest for role=scout leaked %q; got %q", leak, string(res.Digest))
		}
	}
}

// TestC1460_003_DigestInjectionMalformedFailsBeforeLaunchAndNoMatchNeverYieldsFullSource
// is the negative/edge predicate for AC "Unterminated markers fail before
// launch; a no-match role never silently receives the full source".
//
// Clause A (fail-closed): a doc whose opening marker never closes must abort
// the phase with a hard error and MUST NOT call bridge.Launch. The control leg
// (well-formed doc → launches == 1) is what makes launches == 0 meaningful —
// without it, any unrelated early error would false-green this predicate.
//
// Clause B (no silent fallback): a role with no matching block gets an EMPTY
// digest classified OutcomeNoMatch — never the full source — while the live
// prompt is preserved unchanged so behavior does not regress.
func TestC1460_003_DigestInjectionMalformedFailsBeforeLaunchAndNoMatchNeverYieldsFullSource(t *testing.T) {
	// Clause A — malformed input.
	//
	// NOTE (harness fact, measured): this minimal harness has no phase
	// registry on disk, so resp.Verdict is FAIL even on a clean dispatch.
	// Verdict is therefore NON-DISCRIMINATING here and is deliberately not
	// asserted — the load-bearing signals are (a) a non-nil error attributed
	// to the digest seam and (b) launches == 0, both contrasted against the
	// control leg below, which measurably returns err == nil / launches == 1.
	_, br, _, _, err := runPhase(t, "scout", unterminatedDoc)
	if err == nil {
		t.Errorf("Run returned nil error for an unterminated digest marker, want a hard failure")
	}
	if br.launches != 0 {
		t.Errorf("bridge.Launch called %d times on a malformed instruction body, want 0 (must fail BEFORE launch)", br.launches)
	}
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "digest") {
		t.Errorf("failure is not attributed to the digest seam: %v", err)
	}

	// Control leg — a well-formed doc through the same harness DOES dispatch,
	// proving launches == 0 above is caused by the malformed marker and not by
	// an unrelated early abort in this harness.
	_, okBr, _, _, okErr := runPhase(t, "scout", taggedDoc)
	if okErr != nil {
		t.Fatalf("control leg: well-formed doc must dispatch cleanly, got %v", okErr)
	}
	if okBr.launches != 1 {
		t.Fatalf("control leg: bridge.Launch called %d times on a well-formed body, want 1 (harness cannot prove fail-before-launch otherwise)", okBr.launches)
	}

	// Clause B — no-match role.
	res := digest.Materialize([]byte(taggedDoc), "audit")
	if res.Outcome != digest.OutcomeNoMatch {
		t.Errorf("Materialize(taggedDoc, audit).Outcome = %v, want OutcomeNoMatch", res.Outcome)
	}
	if len(res.Digest) != 0 {
		t.Errorf("no-match digest must be empty, got %d bytes: %q", len(res.Digest), string(res.Digest))
	}
	if strings.Contains(string(res.Digest), "SCOUT-ONLY-INSTRUCTIONS") || strings.Contains(string(res.Digest), "UNTAGGED-PREAMBLE") {
		t.Errorf("no-match role silently received source content: %q", string(res.Digest))
	}
	hk, _, _, _, _ := runPhase(t, "audit", taggedDoc)
	if !strings.Contains(hk.composedBody, "UNTAGGED-PREAMBLE") || !strings.Contains(hk.composedBody, "SCOUT-ONLY-INSTRUCTIONS") {
		t.Errorf("a no-match role must keep the LIVE prompt unchanged (no substitution of an empty digest); got %q", hk.composedBody)
	}
}

// --- digest-shadow-size-parity ---------------------------------------------

// TestC1460_004_DigestShadowRecordsByteCountsAndParityVerdict covers AC
// "Shadow data records full-versus-digest byte counts and a parity verdict".
// Two legs: the pure derivation (NewShadowRecord over a real Materialize) and
// the dispatch-time emission captured off the runner's injected diagnostics
// sink — the reachability proof that telemetry is produced by the live path.
func TestC1460_004_DigestShadowRecordsByteCountsAndParityVerdict(t *testing.T) {
	res := digest.Materialize([]byte(taggedDoc), "scout")
	rec := digest.NewShadowRecord([]byte(taggedDoc), res)

	if rec.FullBytes != len(taggedDoc) {
		t.Errorf("FullBytes = %d, want %d (the exact source a live dispatch would have sent)", rec.FullBytes, len(taggedDoc))
	}
	if rec.DigestBytes != len(res.Digest) {
		t.Errorf("DigestBytes = %d, want %d", rec.DigestBytes, len(res.Digest))
	}
	if rec.DigestBytes == 0 || rec.DigestBytes >= rec.FullBytes {
		t.Errorf("fixture must record a real reduction: digest=%d full=%d", rec.DigestBytes, rec.FullBytes)
	}
	if !rec.Parity {
		t.Errorf("Parity = false for a well-formed, strictly-smaller match, want true")
	}
	if rec.Outcome != digest.OutcomeMatched {
		t.Errorf("ShadowRecord.Outcome = %v, want OutcomeMatched", rec.Outcome)
	}

	line := runner.FormatDigestShadowLog("scout", rec)
	for _, want := range []string{"digest-shadow", "outcome=matched", "full_bytes=", "digest_bytes=", "parity=true"} {
		if !strings.Contains(line, want) {
			t.Errorf("FormatDigestShadowLog missing %q; got %q", want, line)
		}
	}

	// Dispatch-time emission: the runner must log this record for a real run.
	_, _, diagOut, _, _ := runPhase(t, "scout", taggedDoc)
	if !strings.Contains(diagOut, "digest-shadow") {
		t.Errorf("no digest-shadow telemetry emitted by a live dispatch; diag output was %q", diagOut)
	}
	if !strings.Contains(diagOut, "parity=true") || !strings.Contains(diagOut, "outcome=matched") {
		t.Errorf("dispatch telemetry did not record the match/parity verdict; got %q", diagOut)
	}
}

// TestC1460_005_DigestShadowUnsafeProjectionsPreserveLivePromptAndCannotClaimSaving
// covers AC "Empty, malformed, or parity-failing projections preserve the live
// prompt and are explicitly non-successful". All three unsafe shapes must
// collapse to Parity == false, and the no-match dispatch must keep the full
// live prompt while still emitting non-successful telemetry.
func TestC1460_005_DigestShadowUnsafeProjectionsPreserveLivePromptAndCannotClaimSaving(t *testing.T) {
	cases := []struct {
		name string
		full []byte
		res  digest.Result
	}{
		{
			name: "empty/no-match projection",
			full: []byte(taggedDoc),
			res:  digest.Materialize([]byte(taggedDoc), "audit"),
		},
		{
			name: "malformed projection",
			full: []byte(unterminatedDoc),
			res:  digest.Materialize([]byte(unterminatedDoc), "scout"),
		},
		{
			name: "matched but not strictly smaller",
			full: []byte("ab"),
			res:  digest.Result{Outcome: digest.OutcomeMatched, Digest: []byte("abcd")},
		},
	}
	for _, tc := range cases {
		rec := digest.NewShadowRecord(tc.full, tc.res)
		if rec.Parity {
			t.Errorf("%s: Parity = true, want false — an unsafe projection must never claim a saving (full=%d digest=%d)", tc.name, rec.FullBytes, rec.DigestBytes)
		}
		if rec.FullBytes != len(tc.full) {
			t.Errorf("%s: FullBytes = %d, want %d", tc.name, rec.FullBytes, len(tc.full))
		}
	}

	// Explicitly non-successful classification for the malformed shape.
	mal := digest.Materialize([]byte(unterminatedDoc), "scout")
	if mal.Outcome != digest.OutcomeMalformed {
		t.Errorf("Materialize(unterminatedDoc).Outcome = %v, want OutcomeMalformed", mal.Outcome)
	}
	if mal.Err == nil {
		t.Errorf("OutcomeMalformed must carry a non-nil Err")
	}
	if mal.Digest != nil {
		t.Errorf("OutcomeMalformed must carry a nil Digest, got %q", string(mal.Digest))
	}

	// Live-prompt preservation + non-successful telemetry on the empty shape.
	hk, br, diagOut, _, err := runPhase(t, "audit", taggedDoc)
	if err != nil {
		t.Fatalf("a no-match projection must not fail the phase: %v", err)
	}
	if br.launches != 1 {
		t.Errorf("bridge.Launch called %d times, want 1 — an empty projection must still dispatch the live prompt", br.launches)
	}
	if strings.TrimSpace(hk.composedBody) != strings.TrimSpace(taggedDoc) {
		t.Errorf("live prompt was altered by an empty projection\n got: %q\nwant: %q", hk.composedBody, taggedDoc)
	}
	if !strings.Contains(diagOut, "parity=false") || !strings.Contains(diagOut, "outcome=no_match") {
		t.Errorf("empty projection must be recorded as explicitly non-successful; diag output was %q", diagOut)
	}
	if strings.Contains(diagOut, "digest_bytes=0 parity=true") {
		t.Errorf("an empty projection recorded a saving; diag output was %q", diagOut)
	}
}

// TestC1460_006_DigestShadowNoProfileDefaultFlipBeforeBaseline covers AC "No
// profile default flip is made until the telemetry baseline is recorded".
//
// The load-bearing half is behavioral: it drives the REAL profiles loader over
// the repository's real .evolve/profiles directory and asserts every parsed
// Profile leaves DigestFile empty. That exercises production JSON parsing, not
// a text grep, so adding the string "digest_file" to a comment cannot pass it.
//
// acs-predicate: config-check — the assertion's subject is a declarative
// configuration surface (no profile may opt into a pre-generated digest yet);
// there is no runtime behavior to invoke beyond loading it.
func TestC1460_006_DigestShadowNoProfileDefaultFlipBeforeBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profileDir := filepath.Join(root, ".evolve", "profiles")
	loader := profiles.NewFromDir(profileDir)
	if loader == nil {
		t.Fatalf("profiles loader nil for %s", profileDir)
	}
	names, err := loader.List()
	if err != nil {
		t.Fatalf("profiles.List(%s): %v", profileDir, err)
	}
	if len(names) == 0 {
		t.Fatalf("no profiles found under %s — predicate would vacuously pass", profileDir)
	}
	flipped := []string{}
	for _, name := range names {
		prof, err := loader.Get(name)
		if err != nil {
			continue
		}
		if prof.DigestFile != "" {
			flipped = append(flipped, name+"="+prof.DigestFile)
		}
	}
	if len(flipped) != 0 {
		t.Errorf("profile default flip performed before a recorded telemetry baseline: %v", flipped)
	}
}
