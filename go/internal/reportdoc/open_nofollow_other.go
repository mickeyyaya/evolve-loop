//go:build !darwin && !linux

package reportdoc

import "os"

func openCitationNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
