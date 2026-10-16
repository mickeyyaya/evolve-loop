package flock

func PathLock(dataPath string) (release func(), err error) {
	return Lock(dataPath + ".lock")
}

func WithPathLock(dataPath string, fn func() error) error {
	release, err := PathLock(dataPath)
	if err != nil {
		return err
	}
	defer release()
	return fn()
}
