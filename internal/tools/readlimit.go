package tools

import (
	"fmt"
	"io"
	"os"
)

// ReadRegularFilePrefix читает ограниченный префикс обычного файла.
func ReadRegularFilePrefix(path string, limit int64) ([]byte, os.FileInfo, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, info, false, fmt.Errorf("not a regular file: %s", path)
	}
	if limit < 0 {
		limit = 0
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, info, false, err
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, info, false, err
	}
	if int64(len(raw)) > limit {
		return raw[:limit], info, true, nil
	}
	return raw, info, false, nil
}
