package reviewcheck

import (
	"bufio"
	"fmt"
	"slices"
	"strings"
)

const consumerTrailer = "widening-consumer:"

type Grants struct {
	AllowedTools   []string
	WriteSubpaths  []string
	EnvPassthrough []string
}

type FileChange struct {
	Path string
	Base *Grants
	Head *Grants
}

type grantField struct {
	name string
	of   func(Grants) []string
}

var grantFields = []grantField{
	{"allowed_tools", func(g Grants) []string { return g.AllowedTools }},
	{"sandbox.write_subpaths", func(g Grants) []string { return g.WriteSubpaths }},
	{"env_passthrough", func(g Grants) []string { return g.EnvPassthrough }},
}

type consumer struct {
	file, field, path, entry string
}

type grantEntry struct {
	file, field, entry string
}

type trace struct {
	consumers []consumer
	tracked   func(string) bool
}

func PermissionWidening(changes []FileChange, bodies []string, tracked func(string) bool) []Refusal {
	t := trace{consumers: consumersOf(bodies), tracked: tracked}
	var refusals []Refusal
	for _, c := range changes {
		if c.Head == nil {
			continue
		}
		base := Grants{}
		if c.Base != nil {
			base = *c.Base
		}
		for _, field := range grantFields {
			for _, entry := range field.of(*c.Head) {
				if slices.Contains(field.of(base), entry) {
					continue
				}
				if reason, ok := t.untraced(grantEntry{file: c.Path, field: field.name, entry: entry}); ok {
					refusals = append(refusals, Refusal{Check: CheckPermissionWidening, Subject: c.Path + " " + field.name + " " + entry, Reason: reason})
				}
			}
		}
	}
	return refusals
}

func (t trace) untraced(g grantEntry) (string, bool) {
	var missing []string
	for _, c := range t.consumers {
		if c.file != g.file || c.field != g.field || c.entry != g.entry {
			continue
		}
		if t.tracked(c.path) {
			return "", false
		}
		missing = append(missing, c.path)
	}
	if len(missing) > 0 {
		return fmt.Sprintf("the widening names the consumer %s, which is not in the tree at the head", strings.Join(missing, ", ")), true
	}
	return fmt.Sprintf("the widening names no consumer: add the trailer `Widening-consumer: %s %s <consumer path> %s` to a commit of the range", g.file, g.field, g.entry), true
}

func consumersOf(bodies []string) []consumer {
	var out []consumer
	for _, body := range bodies {
		lines := bufio.NewScanner(strings.NewReader(body))
		for lines.Scan() {
			line := strings.TrimSpace(lines.Text())
			if len(line) < len(consumerTrailer) || !strings.EqualFold(line[:len(consumerTrailer)], consumerTrailer) {
				continue
			}
			rest := line[len(consumerTrailer):]
			var words []string
			for len(words) < 3 {
				word, tail, ok := cutWord(rest)
				if !ok {
					break
				}
				words, rest = append(words, word), tail
			}
			entry := strings.TrimSpace(rest)
			if len(words) < 3 || entry == "" {
				continue
			}
			out = append(out, consumer{file: words[0], field: words[1], path: words[2], entry: entry})
		}
	}
	return out
}

func cutWord(s string) (string, string, bool) {
	s = strings.TrimLeft(s, " \t")
	end := strings.IndexAny(s, " \t")
	if s == "" || end < 0 {
		return "", "", false
	}
	return s[:end], s[end:], true
}
