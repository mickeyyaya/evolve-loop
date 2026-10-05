package profiles

import (
	"slices"
	"strings"
)

func BaseCLI(cli string) string {
	s := strings.TrimSpace(cli)
	for {
		next := strings.TrimSuffix(strings.TrimSuffix(s, "-tmux"), "-p")
		if next == s {
			return next
		}
		s = next
	}
}

func (p *Profile) AllowedFamilies() []string {
	if p == nil {
		return nil
	}
	var out []string
	for _, entry := range p.AllowedCLIs {
		family := BaseCLI(entry)
		if family == "all" {
			return nil
		}
		if !slices.Contains(out, family) {
			out = append(out, family)
		}
	}
	return out
}

func (p *Profile) AllowsFamily(cli string) bool {
	families := p.AllowedFamilies()
	return families == nil || slices.Contains(families, BaseCLI(cli))
}
