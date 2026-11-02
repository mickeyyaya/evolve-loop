package qualitypanel

import (
	"path"
	"strings"
)

func ParseHunks(diff string) []Hunk {
	var out []Hunk
	var file string
	var header, body []string
	flush := func() {
		if file != "" && len(body) > 0 {
			out = append(out, Hunk{Package: path.Dir(file), File: file, Patch: strings.Join(append(append([]string(nil), header...), body...), "\n") + "\n"})
		}
		body = nil
	}
	for _, line := range strings.Split(strings.TrimSuffix(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			file, header = productionGoFile(line), []string{line}
		case strings.HasPrefix(line, "@@ "):
			flush()
			body = []string{line}
		case body != nil:
			body = append(body, line)
		default:
			header = append(header, line)
		}
	}
	flush()
	return out
}

func productionGoFile(diffLine string) string {
	fields := strings.Fields(diffLine)
	file := strings.TrimPrefix(fields[len(fields)-1], "b/")
	if strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") {
		return file
	}
	return ""
}
