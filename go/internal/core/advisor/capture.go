package advisor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Span is the OTel-GenAI decision span (ADR-0052 WS3-S3) persisted per
// advisor call as advisor-span-<kind>.json. Field keys follow the OTel GenAI
// semantic conventions so a collector can ingest the file directly. PromptSHA/
// ResponseSHA bind the REDACTED capture artifacts — the same identity the
// ledger (WS3-S2) and the replay path (WS3-S5) key off, so all three agree.
// ReplanDepth (WS1-S3) varies: 0 for the initial Plan, 1 for the post-scout
// RePlan — so it records real behavior, not locked surface. It is always
// emitted (no omitempty): a depth of 0 is a meaningful "initial plan", not
// absence. core.AdvisorSpan is an alias of it.
type Span struct {
	Model       string `json:"gen_ai.request.model"`
	System      string `json:"gen_ai.system"`
	PromptSHA   string `json:"prompt_sha"`
	ResponseSHA string `json:"response_sha"`
	DurationMS  int64  `json:"duration_ms"`
	ReplanDepth int    `json:"replan_depth"`
	// Tokens (S5, token-telemetry) records the advisor call's LLM token usage —
	// non-phase burn that was previously dropped on the floor (only CostUSD was
	// captured elsewhere, never the advisor's own tokens). Always emitted: a
	// zero value is a meaningful "no telemetry resolved", not fabricated absence.
	Tokens cyclestate.TokenUsage `json:"tokens"`
}

// capture persists the secret-redacted prompt and response plus the decision
// span for the decision (WS3-S1 + WS3-S3). Best-effort / fail-open: a
// forensic-capture failure must never become a routing outage (the advisor
// would otherwise degrade the whole cycle to the static path over a full
// disk) — each failed artifact is one ADVISOR_CAPTURE_WRITE_FAILED and the
// remaining artifacts are still attempted. Only the PERSISTED COPY is
// redacted; the live prompt the advisor reasoned over is untouched. The
// response is redacted but not otherwise transformed, so WS3-S5 can reparse
// it to the same plan. A nil writer is the Null Object (no forensics); the
// workspace is non-empty here because preflight refused an empty one.
func (a *Advisor) capture(in router.RouteInput, d decision, prompt string, resp LaunchResponse) {
	if a.writeArtifact == nil {
		return
	}
	kind := d.captureKind()
	rp := panetrust.RedactSecrets(prompt)
	rr := panetrust.RedactSecrets(resp.Stdout)
	a.persist(in, d, "prompt", filepath.Join(in.Workspace, "advisor-prompt-"+kind+".txt"), []byte(rp), nil)
	a.persist(in, d, "response", filepath.Join(in.Workspace, "advisor-response-"+kind+".txt"), []byte(rr), nil)
	buf, err := json.Marshal(Span{
		Model:       a.identity.Model,
		System:      llmroute.Family(a.identity.CLI),
		PromptSHA:   sha256OfString(rp),
		ResponseSHA: sha256OfString(rr),
		DurationMS:  resp.DurationMS,
		ReplanDepth: d.replanDepth(),
		Tokens:      resp.Tokens,
	})
	a.persist(in, d, "span", filepath.Join(in.Workspace, "advisor-span-"+kind+".json"), buf, err)
}

// persist writes one artifact, folding its marshal fault (dormant — a Span
// always marshals) and its write fault into ONE warn under fields.op.
func (a *Advisor) persist(in router.RouteInput, d decision, artifact, path string, data []byte, marshalErr error) {
	op, err := opMarshal, marshalErr
	if err == nil {
		op, err = opWrite, a.writeArtifact(path, data)
	}
	if err != nil {
		a.warn(in, d, CodeCaptureWriteFailed, err.Error(), map[string]string{"step": stepCapture, "artifact": artifact, "op": op, "path": path})
	}
}

// sha256OfString returns the hex sha256 of s — the in-memory twin of the
// ledger's bindArtifactSHA (which reads a file), so the span's bound SHA
// equals the ledger's bound SHA for the same redacted bytes.
func sha256OfString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
