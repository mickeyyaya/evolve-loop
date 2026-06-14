package flock

// withpath.go — the single home for the "<datafile>.lock" sidecar convention.
//
// Every short read-modify-write serializer in the tree locks a SIDECAR file
// named "<datafile>.lock", never the data file itself: the data file is
// rename-replaced by the atomic writers (writeJSONAtomic) out from under any fd
// held on it, so a lock on the data inode would protect the wrong object after
// the first write. state.json (CA.3), cycle-state.json (G7), and ship's map RMW
// all share this one convention — PathLock/WithPathLock is the projection they
// derive from instead of each re-spelling `flock.Lock(path + ".lock")`.

// PathLock acquires the advisory sidecar lock "<dataPath>.lock". BLOCKING (it
// reuses Lock): concurrent read-modify-writers serialize, never refuse. Call the
// returned release exactly once — defer it.
func PathLock(dataPath string) (release func(), err error) {
	return Lock(dataPath + ".lock")
}

// WithPathLock runs fn while holding PathLock(dataPath). The lock is released on
// every exit path, including a panic in fn.
func WithPathLock(dataPath string, fn func() error) error {
	release, err := PathLock(dataPath)
	if err != nil {
		return err
	}
	defer release()
	return fn()
}
