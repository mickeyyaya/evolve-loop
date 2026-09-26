package interaction

import "strings"

// KernelFacts is the closed set of dispatch facts the kernel may disclose; an empty field is unanswerable.
type KernelFacts struct {
	ArtifactPath string
	Workspace    string
	Worktree     string
	Cycle        string // pre-rendered, so the leaf carries no int formatting policy
}

// KernelAnswerer answers blocking questions from a closed fact set.
type KernelAnswerer struct {
	facts KernelFacts
}

// NewKernelAnswerer builds the answerer over one dispatch's facts.
func NewKernelAnswerer(f KernelFacts) *KernelAnswerer {
	return &KernelAnswerer{facts: f}
}

// questionTopics maps a question to a fact key by keyword; the first matching topic wins.
var questionTopics = []struct {
	key      string
	keywords []string
}{
	// "Directory" phrasing means the worktree. The workspace matches only its own words and
	// comes first, so a "scratch directory" question is not taken by the worktree's synonyms.
	{"cycle", []string{"cycle number", "which cycle", "what cycle"}},
	{"workspace", []string{"workspace", "scratch dir", "scratch directory", "scratch space"}},
	{"worktree", []string{"worktree", "work tree", "working directory", "which directory", "what directory", "where do i work", "edit in"}},
	{"artifact_path", []string{"artifact", "deliverable", "report", "output file", "write the", "write to", "where should i write", "what path", "which path", "what file"}},
}

// Answer returns the fact a blocking question asks for, or ("", false) when it maps to no
// non-empty fact. A miss sends the caller down the fallback chain; it never improvises.
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
				// A matched topic with no value is a miss, never a blank answer.
				return "", false
			}
		}
	}
	return "", false
}

// factFor returns "" for any key outside the closed vocabulary, so nothing off-list is disclosed.
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
