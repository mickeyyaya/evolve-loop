package commitgate

import "strings"

type Attestation struct {
	TreeStateSHA string   `json:"tree_state_sha"`
	TS           string   `json:"ts"`
	ChecksPassed []string `json:"checks_passed"`
	ReviewersRun []string `json:"reviewers_run"`
	Tool         string   `json:"tool"`
}

func (a *Attestation) Marshal() []byte {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "tree_state_sha": "` + a.TreeStateSHA + "\",\n")
	b.WriteString(`  "ts": "` + a.TS + "\",\n")
	b.WriteString(`  "checks_passed": [` + jsonStringArray(a.ChecksPassed) + "],\n")
	b.WriteString(`  "reviewers_run": [` + jsonStringArray(a.ReviewersRun) + "],\n")
	b.WriteString(`  "tool": "` + a.Tool + "\"\n")
	b.WriteString("}\n")
	return []byte(b.String())
}

func jsonStringArray(items []string) string {
	if len(items) == 0 {
		return ""
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = `"` + it + `"`
	}
	return strings.Join(quoted, ",")
}

func splitReviewers(csv string) []string {
	var out []string
	for _, r := range strings.Split(csv, ",") {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}
