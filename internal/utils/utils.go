package utils

import (
	"fmt"
	"os"
	"strconv"
)

// BytesToUnits 函数将字节转换为KB、MB、GB或TB
func BytesToUnits(bytes int64) string {
	const (
		KB = 1 << (10 * (iota + 1)) // 1024
		MB                          // 1024^2
		GB                          // 1024^3
		TB                          // 1024^4
	)

	if bytes >= TB {
		return fmt.Sprintf("%.2fTB", float64(bytes)/float64(TB))
	} else if bytes >= GB {
		return fmt.Sprintf("%.2fGB", float64(bytes)/float64(GB))
	} else if bytes >= MB {
		return fmt.Sprintf("%.2fMB", float64(bytes)/float64(MB))
	} else if bytes >= KB {
		return fmt.Sprintf("%.2fKB", float64(bytes)/float64(KB))
	} else {
		return strconv.FormatInt(bytes, 10)
	}
}

// IsDir 是否是目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
