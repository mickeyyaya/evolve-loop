package ipcenv

import (
	"maps"
	"slices"
	"strings"
)

type CredentialSet struct {
	Set   map[string]string
	Unset []string
}

func CredentialIsolation(ghConfigDir, gitConfigGlobal string) CredentialSet {
	return CredentialSet{
		Set: map[string]string{
			"GIT_CONFIG_GLOBAL":   gitConfigGlobal,
			"GIT_CONFIG_NOSYSTEM": "1",
			"GIT_TERMINAL_PROMPT": "0",
			"GIT_SSH_COMMAND":     "ssh -o BatchMode=yes",
			"GH_CONFIG_DIR":       ghConfigDir,
		},
		Unset: []string{"GH_TOKEN", "GITHUB_TOKEN"},
	}
}

func GitIdentityConfig(name, email string) string {
	if name == "" && email == "" {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("[user]\n")
	if name != "" {
		b.WriteString("\tname = " + quoteGitConfig(name) + "\n")
	}
	if email != "" {
		b.WriteString("\temail = " + quoteGitConfig(email) + "\n")
	}
	return b.String()
}

func quoteGitConfig(v string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v) + `"`
}

func (c CredentialSet) Env() []string {
	out := make([]string, 0, len(c.Set))
	for _, k := range slices.Sorted(maps.Keys(c.Set)) {
		out = append(out, k+"="+c.Set[k])
	}
	return out
}
