//go:build !darwin && !linux

package reportdoc

import "os"

func OpenRegularNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
