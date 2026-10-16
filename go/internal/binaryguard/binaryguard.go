// Package binaryguard rejects large compiled executables from a change set
// before they reach a commit.
package binaryguard

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const DefaultThresholdBytes int64 = 1 << 20

var (
	elfMagic             = []byte{0x7f, 'E', 'L', 'F'}
	machO32Magic         = []byte{0xfe, 0xed, 0xfa, 0xce}
	machO64Magic         = []byte{0xfe, 0xed, 0xfa, 0xcf}
	machO64ReversedMagic = []byte{0xcf, 0xfa, 0xed, 0xfe}
	machO32ReversedMagic = []byte{0xce, 0xfa, 0xed, 0xfe}
	machOUniversalMagic  = []byte{0xca, 0xfe, 0xba, 0xbe}
	peMagic              = []byte{'M', 'Z'}
)

var magicSignatures = [][]byte{
	elfMagic,
	machO32Magic,
	machO64Magic,
	machO64ReversedMagic,
	machO32ReversedMagic,
	machOUniversalMagic,
	peMagic,
}

func widestMagicLength() int {
	widest := 0
	for _, sig := range magicSignatures {
		widest = max(widest, len(sig))
	}
	return widest
}

func HasExecutableMagic(head []byte) bool {
	for _, sig := range magicSignatures {
		if bytes.HasPrefix(head, sig) {
			return true
		}
	}
	return false
}

type Offender struct {
	Path string
	Size int64
}

func Scan(root string, files []string, threshold int64) ([]Offender, error) {
	var offenders []Offender
	for _, f := range files {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, f)
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("binaryguard: stat %s: %w", f, err)
		}
		if info.IsDir() || info.Size() <= threshold {
			continue
		}
		head, err := readHead(path, widestMagicLength())
		if err != nil {
			return nil, fmt.Errorf("binaryguard: read %s: %w", f, err)
		}
		if HasExecutableMagic(head) {
			offenders = append(offenders, Offender{Path: f, Size: info.Size()})
		}
	}
	sort.Slice(offenders, func(i, j int) bool { return offenders[i].Path < offenders[j].Path })
	return offenders, nil
}

func readHead(path string, n int) ([]byte, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fh.Close() }()
	buf := make([]byte, n)
	read, err := fh.Read(buf)
	if err != nil && read == 0 {
		return nil, err
	}
	return buf[:read], nil
}
