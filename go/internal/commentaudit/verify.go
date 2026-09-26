package commentaudit

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// VerifyChanges checks each changed Go file for a comment-only edit and
// returns one violation per file that fails.
func VerifyChanges(files []string, before, after func(string) ([]byte, error)) ([]string, error) {
	var violations []string
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		b, a, err := readBoth(before, after, f)
		if err != nil {
			return nil, err
		}
		switch {
		case b == nil:
			violations = append(violations, f+": file added")
		case a == nil:
			violations = append(violations, f+": file deleted")
		default:
			ok, reason, err := Equivalent(f, b, a)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			if !ok {
				violations = append(violations, f+": "+reason)
			}
		}
	}
	return violations, nil
}

func readOptional(read func(string) ([]byte, error), name string) ([]byte, error) {
	src, err := read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return src, err
}

// readBoth reads both sides of one file, treating either side's absence as
// nil; a genuine read error is wrapped with the file name.
func readBoth(before, after func(string) ([]byte, error), name string) (b, a []byte, err error) {
	b, bErr := readOptional(before, name)
	a, aErr := readOptional(after, name)
	if err := errors.Join(bErr, aErr); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	return b, a, nil
}
