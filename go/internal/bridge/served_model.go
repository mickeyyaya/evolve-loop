package bridge

import "strings"

func modelNameOf(token string) string {
	name, _, _ := strings.Cut(token, " (")
	return strings.TrimSpace(name)
}

func sameModelName(served, picked string) bool {
	return picked == "" || strings.EqualFold(bareModelName(served), bareModelName(picked))
}

func bareModelName(name string) string {
	return strings.TrimPrefix(strings.ToLower(name), "claude ")
}

func transcriptServedPick(picked, served string) bool {
	picked, served = strings.ToLower(modelNameOf(picked)), strings.ToLower(served)
	if picked == "" || !strings.HasPrefix(served, "claude-") {
		return true
	}
	alias, _, _ := strings.Cut(picked, "[")
	if !strings.ContainsAny(alias, "0123456789") {
		return strings.Contains(served, alias)
	}
	return served == alias || strings.HasPrefix(served, alias+"-")
}

func (c attemptLogContext) servedModelCheck(picked, served string) {
	if transcriptServedPick(picked, served) {
		return
	}
	c.launchWarn("attemptLogContext.servedModelCheck", "", CodeDispatchModelMismatch,
		c.cli+" dispatched "+picked+" but the transcript shows "+served+" served the run",
		map[string]string{"expected_model": picked, "served_model": served})
}
