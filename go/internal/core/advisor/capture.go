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

// Span is the OTel-GenAI decision span persisted per call as advisor-span-<kind>.json (core.AdvisorSpan).
type Span struct {
	Model       string                `json:"gen_ai.request.model"`
	System      string                `json:"gen_ai.system"`
	PromptSHA   string                `json:"prompt_sha"`
	ResponseSHA string                `json:"response_sha"`
	DurationMS  int64                 `json:"duration_ms"`
	ReplanDepth int                   `json:"replan_depth"`
	Tokens      cyclestate.TokenUsage `json:"tokens"`
}

// capture is fail-open: a capture fault is reported and never fails the decision.
// Only the persisted copy is redacted; the live prompt is untouched.
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

func (a *Advisor) persist(in router.RouteInput, d decision, artifact, path string, data []byte, marshalErr error) {
	op, err := opMarshal, marshalErr
	if err == nil {
		op, err = opWrite, a.writeArtifact(path, data)
	}
	if err != nil {
		a.warn(in, d, CodeCaptureWriteFailed, err.Error(), map[string]string{"step": stepCapture, "artifact": artifact, "op": op, "path": path})
	}
}

// sha256OfString must equal the ledger's bindArtifactSHA of the same persisted bytes.
func sha256OfString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
