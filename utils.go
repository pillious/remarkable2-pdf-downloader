package main

import (
	"fmt"
	"os"
)

func makeDir(path string) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}
}

func removeEmptyDir(path string) {
	os.Remove(path)
}

func formatSize(size int) string {
	const (
		B  = 1
		KB = 1000 * B
		MB = 1000 * KB
		GB = 1000 * MB
		TB = 1000 * GB
	)

	switch {
	case size < KB:
		return fmt.Sprintf("%d B", size)
	case size < MB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	case size < GB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size < TB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	default:
		return fmt.Sprintf("%.2f TB", float64(size)/TB)
	}
}
