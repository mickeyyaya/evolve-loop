package cycle104

import "os"

func readFileBytes(path string) ([]byte, error) { return os.ReadFile(path) }
