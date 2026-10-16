package interaction

import "strings"

type KernelFacts struct {
	ArtifactPath string
	Workspace    string
	Worktree     string
	Cycle        string
}

type KernelAnswerer struct {
	facts KernelFacts
}

func NewKernelAnswerer(f KernelFacts) *KernelAnswerer {
	return &KernelAnswerer{facts: f}
}

var questionTopics = []struct {
	key      string
	keywords []string
}{
	{"cycle", []string{"cycle number", "which cycle", "what cycle"}},
	{"workspace", []string{"workspace", "scratch dir", "scratch directory", "scratch space"}},
	{"worktree", []string{"worktree", "work tree", "working directory", "which directory", "what directory", "where do i work", "edit in"}},
	{"artifact_path", []string{"artifact", "deliverable", "report", "output file", "write the", "write to", "where should i write", "what path", "which path", "what file"}},
}

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
				return "", false
			}
		}
	}
	return "", false
}

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
