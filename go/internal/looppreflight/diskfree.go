package looppreflight

type DiskUsage struct {
	FreeBytes  uint64
	TotalBytes uint64
}

func DiskUsageOf(path string) (DiskUsage, error) { return defaultDiskUsageOf(path) }

func DiskFreeBytes(path string) (uint64, error) { return defaultDiskFreeBytes(path) }
