package interaction

// askbroker.go — ADR-0045 I3: answer the agent's blocking question when the
// KERNEL already knows the answer, instead of failing the whole phase to a
// cross-family re-dispatch (cycle-267: a stuck prompt escalated exit 85 and
// the fallback CLI re-did the entire phase to get past a question one injected
// line would have cleared).
//
// KernelAnswerer is a Strategy over a CLOSED fact vocabulary — only facts
// already present in the agent's own dispatch contract (artifact_path,
// workspace, worktree, cycle). It is STRUCTURALLY incapable of saying anything
// the agent's prompt didn't already contain, so a manipulated pane asking for
// privileged facts gets nothing it couldn't already see (threat S7). A
// question it cannot map to a known fact is a MISS — never an improvised
// string — and the caller falls through to the existing exit-85 → cross-family
// fallback chain (the unconditional floor; I3 must never suppress it).
//
// The vocabulary is intentionally limited to what bridge.Config can supply
// today; goal_hash / required_sections are additional dispatch facts the
// design names but the bridge does not yet carry, so they are deliberately
// absent rather than declared-but-dead (add the fact + its keywords together
// when Config grows them).
//
// Leaf: pure string mapping, no I/O, no LLM. The quarantined-advisor tail for
// the residue (novel questions) is the FailureAdviser port shape, dispatched
// by the caller — out of this leaf.

import "strings"

// KernelFacts is the closed vocabulary: the dispatch facts the kernel may
// disclose to unstick a blocked agent. Empty fields are simply unanswerable
// (never guessed).
type KernelFacts struct {
	ArtifactPath string
	Workspace    string
	Worktree     string
	Cycle        string // pre-rendered (leaf takes no int formatting policy)
}

// KernelAnswerer answers blocking questions from a closed fact set.
type KernelAnswerer struct {
	facts KernelFacts
}

// NewKernelAnswerer builds the answerer over one dispatch's facts.
func NewKernelAnswerer(f KernelFacts) *KernelAnswerer {
	return &KernelAnswerer{facts: f}
}

// questionTopic maps a question to the closed fact-key vocabulary by keyword.
// Order matters: more specific topics first (an "artifact path" question
// mentions both "path" and possibly "workspace").
var questionTopics = []struct {
	key      string
	keywords []string
}{
	// Most-specific topics first. "directory" phrasing maps to the WORKTREE
	// (the code root a source-writing agent edits in); the workspace is the
	// scratch dir, keyed by its own explicit words so a "scratch directory"
	// question is not hijacked by the worktree's directory synonyms.
	{"cycle", []string{"cycle number", "which cycle", "what cycle"}},
	{"workspace", []string{"workspace", "scratch dir", "scratch directory", "scratch space"}},
	{"worktree", []string{"worktree", "work tree", "working directory", "which directory", "what directory", "where do i work", "edit in"}},
	{"artifact_path", []string{"artifact", "deliverable", "report", "output file", "write the", "write to", "where should i write", "what path", "which path", "what file"}},
}

// Answer returns the kernel's answer to a blocking question and ok=true, or
// ("", false) when the question maps to no known fact (or the mapped fact is
// empty). A miss is the caller's signal to fall through to the chain — never
// to improvise.
func (a *KernelAnswerer) Answer(question string) (string, bool) {
	if a == nil || strings.TrimSpace(question) == "" {
		return "", false
	}
	q := strings.ToLower(question)
	for _, topic := range questionTopics {
		for _, kw := range topic.keywords {
			if strings.Contains(q, kw) {
				if v := a.factFor(topic.key); v != "" {
					return v, true
				}
				// The topic matched but the kernel has no value — a MISS,
				// not an empty answer (don't inject a blank).
				return "", false
			}
		}
	}
	return "", false
}

// factFor resolves a closed-vocabulary key to its value. An unknown key is
// structurally impossible to reach (questionTopics is the only caller), but
// the default returns "" so the answerer can never disclose anything off-list.
func (a *KernelAnswerer) factFor(key string) string {
	switch key {
	case "artifact_path":
		return a.facts.ArtifactPath
	case "workspace":
		return a.facts.Workspace
	case "worktree":
		return a.facts.Worktree
	case "cycle":
		return a.facts.Cycle
	default:
		return ""
	}
}
