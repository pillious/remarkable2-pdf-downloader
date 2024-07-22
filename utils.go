package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

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

func sliceToSet(lst []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, s := range lst {
		s = strings.TrimSpace(s)
		set[s] = struct{}{}
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

func removeEmptyDir(path string) {
	os.Remove(path)
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

type listFlags []string

func (l *listFlags) String() string {
	return strings.Join(*l, ", ")
}

func (l *listFlags) Set(value string) error {
	*l = append(*l, value)
	return nil
}
