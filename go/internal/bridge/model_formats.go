package bridge

import (
	"errors"
	"regexp"
	"strings"
)

const ollamaHeaderName = "NAME"

var errNoModelRows = errors.New("parsed no models from output")

var agyModelID = regexp.MustCompile(`^[a-z0-9][a-z0-9.:_-]*$`)

type modelsFormat struct {
	argv  []string
	parse func(string) ([]string, error)
}

var modelsFormats = map[string]modelsFormat{
	"agy-models-v1":  {argv: []string{"models"}, parse: parseAgyModels},
	"ollama-list-v1": {argv: []string{"list"}, parse: parseOllamaList},
}

func knownModelsFormat(id string) bool {
	_, ok := modelsFormats[id]
	return ok
}

func parseAgyModels(out string) ([]string, error) {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		id, display, hasTab := strings.Cut(line, "\t")
		isModelRow := hasTab && agyModelID.MatchString(strings.TrimSpace(id))
		if !isModelRow {
			continue
		}
		if name := strings.TrimSpace(display); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, errNoModelRows
	}
	return names, nil
}

func parseOllamaList(out string) ([]string, error) {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == ollamaHeaderName {
			continue
		}
		ids = append(ids, fields[0])
	}
	return ids, nil
}
