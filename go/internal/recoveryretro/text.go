package recoveryretro

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

const maxCellRunes = 240

func quote(s string) string {
	clean := strings.Join(strings.Fields(strings.ReplaceAll(s, "`", "'")), " ")
	if clean == "" {
		return "—"
	}
	return "`" + clean + "`"
}

func cell(s string) string {
	return strings.ReplaceAll(quote(textcap.TruncateRunes(s, maxCellRunes)), "|", `\|`)
}

func quoteAll(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = quote(item)
	}
	if len(quoted) < 2 {
		return strings.Join(quoted, "")
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

const (
	maxSlugLen = 80
	emptySlug  = "item"
)

func itemSlug(id string) string {
	s := slug(id)
	if len(s) > maxSlugLen {
		s = strings.TrimSuffix(s[:maxSlugLen], "-")
	}
	if s == "" {
		return emptySlug
	}
	return s
}
