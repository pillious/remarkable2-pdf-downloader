package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type listFlags []string

func (l *listFlags) String() string {
	return strings.Join(*l, ", ")
}

func (l *listFlags) Set(value string) error {
	*l = append(*l, value)
	return nil
}

var logFile *os.File

func setupFileLogger(fileName string) string {
	filePath := backupsDir + "/" + fileName

	var err error
	logFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}

	log.SetOutput(logFile)

	return filePath
}

func closeFileLogger() {
	if logFile != nil {
		log.SetOutput(os.Stdout)
		err := logFile.Close()
		if err != nil {
			log.Println("Error closing log file:", err)
		}
		log.Printf("Logs written to %s\n", logFile.Name())
	}
}

func sliceToSet(lst []string) Set[string] {
	set := make(Set[string])
	for _, s := range lst {
		s = strings.TrimSpace(s)
		set.add(s)
	}
	return set
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

func makeDir(path string) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Panic(err)
	}
}

func removeEmptyDir(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Panic(err)
	}
	if !fileInfo.IsDir() {
		log.Panicf("%s is a file not a directory.\n", path)
	}

	err = os.Remove(path)
	if err != nil {
		if errors.Is(err, syscall.ENOTEMPTY) {
			return false
		} else {
			log.Panic(err)
		}
	}
	return true
}

func genFullPath(s string) string {
	absPath, err := filepath.Abs(backupsDir)
	if err != nil {
		log.Panic(err)
	}
	return absPath + "/" + s
}

func createPdf(fileName string, content *[]byte, path string) {
	pdf, err := os.Create(path + "/" + fileName)
	if err != nil {
		log.Panic(err)
	}
	defer pdf.Close()

	_, err = pdf.Write(*content)
	if err != nil {
		log.Panic(err)
	}
}

// returns 0 if the string can't be converted.
func strToUint64(s string) uint64 {
	num, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return num
}
