// Package binaryguard rejects large compiled executables from a change set
// before they reach a commit. It exists because an ACS predicate's
// `go build <main pkg>` once left an 18MB Mach-O `evolve` binary in its package
// dir that was then committed (tracked-binary-in-acs-dir). The root cause is
// fixed (build predicates now use `-o os.DevNull`); this guard is the general,
// repo-wide backstop wired into the commit chokepoint so no compiled artifact of
// any kind can be re-added silently.
//
// The guard is deliberately narrow: it fires only when a file BOTH exceeds a
// size threshold AND begins with a known executable magic signature. That
// two-condition rule keeps legitimate large text/data assets (fixtures,
// manifests) from ever tripping it.
package binaryguard

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DefaultThresholdBytes is the size floor above which a file that also carries an
// executable magic signature is rejected. 1 MiB is far above any source file yet
// far below a real compiled binary (the offending evolve binary was ~18 MiB).
const DefaultThresholdBytes int64 = 1 << 20

// magicSignatures are the leading bytes of compiled executables we never want
// committed: ELF (Linux), Mach-O 32/64/fat (macOS), and PE (Windows "MZ").
var magicSignatures = [][]byte{
	{0x7f, 'E', 'L', 'F'},    // ELF
	{0xfe, 0xed, 0xfa, 0xce}, // Mach-O 32-bit
	{0xfe, 0xed, 0xfa, 0xcf}, // Mach-O 64-bit
	{0xcf, 0xfa, 0xed, 0xfe}, // Mach-O 64-bit, reversed byte order
	{0xce, 0xfa, 0xed, 0xfe}, // Mach-O 32-bit, reversed byte order
	{0xca, 0xfe, 0xba, 0xbe}, // Mach-O universal (fat) binary
	{'M', 'Z'},               // PE / DOS executable (Windows)
}

// HasExecutableMagic reports whether head begins with a known executable magic
// signature. Pure and allocation-free — safe to call on the first few bytes of a
// file rather than the whole thing.
func HasExecutableMagic(head []byte) bool {
	for _, sig := range magicSignatures {
		if bytes.HasPrefix(head, sig) {
			return true
		}
	}
	return false
}

// Offender is a change-set file rejected by the guard.
type Offender struct {
	Path string
	Size int64
}

// Scan inspects each file (interpreted relative to root when not absolute) and
// returns the offenders that are both larger than threshold and start with an
// executable magic signature. Files that do not exist on disk are skipped (a
// deletion in the change set is not an offender). Offenders are returned sorted
// by path for a stable, testable order. A non-existence read error is skipped; a
// real read error (permission, etc.) is returned.
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
		head, err := readHead(path, len(magicSignatures[0]))
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

// readHead reads up to n bytes (we only need the widest magic prefix; ELF/Mach-O
// are 4, PE is 2) so we never load a multi-megabyte binary to classify it.
func readHead(path string, n int) ([]byte, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	buf := make([]byte, n)
	read, err := fh.Read(buf)
	if err != nil && read == 0 {
		return nil, err
	}
	return buf[:read], nil
}
