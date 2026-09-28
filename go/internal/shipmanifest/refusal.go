package shipmanifest

import "strings"

const addRefusalHeader = "The following paths are ignored by one of your .gitignore files:"

func IgnoredInAddRefusal(stderr string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, addRefusalHeader):
			in = true
		case in && strings.HasPrefix(trimmed, "hint:"):
			return out
		case in && trimmed != "":
			if p := UnquoteGitPath(trimmed); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func StageRetrying(paths []string, stage func([]string) (string, error)) (staged, refused []string, err error) {
	if len(paths) == 0 {
		return nil, nil, nil
	}
	stderr, err := stage(paths)
	if err == nil {
		return paths, nil, nil
	}
	offenders := IgnoredInAddRefusal(stderr)
	retry := without(paths, offenders)
	if len(retry) == 0 || len(retry) == len(paths) {
		return paths, nil, err
	}
	_, err = stage(retry)
	return retry, offenders, err
}

func without(paths, drop []string) []string {
	dropped := make(map[string]bool, len(drop))
	for _, d := range drop {
		dropped[d] = true
	}
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if !dropped[p] {
			kept = append(kept, p)
		}
	}
	return kept
}
