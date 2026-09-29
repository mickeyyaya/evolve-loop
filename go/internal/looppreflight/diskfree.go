package looppreflight

func DiskFreeBytes(path string) (uint64, error) { return defaultDiskFreeBytes(path) }
