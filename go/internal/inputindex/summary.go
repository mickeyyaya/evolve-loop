package inputindex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
)

const summaryMax = 160

var rawOutputSuffixes = []string{"-stdout.log", "-stderr.log", "-stdout.clean.txt", "-final-pane.txt", "-final-scrollback.txt", "-pane-watch.json"}

func IsRawOutput(path string) bool {
	slashed := "/" + filepath.ToSlash(path)
	if strings.Contains(slashed, "/.bridge/") {
		return true
	}
	for _, suffix := range rawOutputSuffixes {
		if strings.HasSuffix(slashed, suffix) {
			return true
		}
	}
	return false
}

func RawOutputSuffixes() []string {
	return append([]string(nil), rawOutputSuffixes...)
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func summaryOf(name string, body []byte) string {
	var line string
	switch filepath.Ext(name) {
	case ".md":
		line = markdownSummary(string(body))
	case ".json":
		var doc map[string]any
		if json.Unmarshal(body, &doc) == nil {
			line, _ = doc["summary"].(string)
		}
	case ".yaml", ".yml":
		line = yamlSummary(string(body))
	}
	line = strings.Join(strings.Fields(line), " ")
	if r := []rune(line); len(r) > summaryMax {
		line = string(r[:summaryMax])
	}
	return line
}

func markdownSummary(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "<!--") {
			return strings.TrimLeft(line, "# ")
		}
	}
	return ""
}

func yamlSummary(body string) string {
	for _, line := range strings.Split(body, "\n") {
		for _, key := range []string{"summary:", "title:"} {
			if value, ok := strings.CutPrefix(line, key); ok {
				return strings.Trim(strings.TrimSpace(value), `"'`)
			}
		}
	}
	return ""
}
