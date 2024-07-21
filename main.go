package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	baseUrl        = "http://10.11.99.1:80/"
	backupLocation = "./mypdfbackups/"
)

type Document struct {
	Id        string `json:"ID"`
	UpdatedAt string `json:"ModifiedClient"`
	Parent    string `json:"Parent"`
	Type      string `json:"Type"`
	Name      string `json:"VissibleName"`
	// Only defined a subset of the fields
}

func main() {
	includeSet := make(map[string]struct{})
	includeSet["uuuu"] = struct{}{}
	includeSet["Quick sheets"] = struct{}{}
	includeSet["ANTH 222"] = struct{}{}
	excludeSet := make(map[string]struct{})
	excludeSet["uuuu"] = struct{}{}

	traverseFileSystem(Stack[string]{}, "", &includeSet, &excludeSet)
}

// The exclude set has priority over include set.
func traverseFileSystem(path Stack[string], folderId string, include *map[string]struct{}, exclude *map[string]struct{}) {
	isRoot := folderId == ""
	pathStr := strings.Join(path, "/")

	makeDir(backupLocation + pathStr)

	var docs []Document
	if isRoot {
		docs = *getDocumentsAtRoot()
	} else {
		docs = *getDocuments(folderId)
	}

	for _, doc := range docs {
		if _, ok := (*exclude)[doc.Id]; ok {
			continue
		}
		if _, ok := (*exclude)[doc.Name]; ok {
			continue
		}

		switch doc.Type {
		case "DocumentType":
			if _, ok := (*include)[doc.Id]; !ok {
				if _, ok := (*include)[doc.Name]; !ok {
					continue
				}
			}

			if isRoot {
				fmt.Printf("%s ... ", doc.Name)
			} else {
				fmt.Printf("%s/%s ... ", pathStr, doc.Name)
			}

			startTime := time.Now()
			pdfName, content := downloadDocument(doc.Id)
			duration := time.Since(startTime).Seconds()
			fmt.Printf("time: %.2fs | size: %s\n", duration, formatSize(len(*content)))

			createPdf(pdfName, content, backupLocation+pathStr)

		case "CollectionType":
			path.Push(doc.Name)
			traverseFileSystem(path, doc.Id, include, exclude)
			removeEmptyDir(backupLocation + strings.Join(path, "/"))
			path.Pop()
		}
	}
}

func getDocumentsAtRoot() *[]Document {
	return getDocuments("")
}

func getDocuments(folderId string) *[]Document {
	url := baseUrl + "documents/" + folderId

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	docs := &[]Document{}
	err = json.NewDecoder(res.Body).Decode(docs)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusOK {
		panic(res.StatusCode)
	}

	return docs
}

func downloadDocument(documentId string) (string, *[]byte) {
	url := baseUrl + "download/" + documentId + "/placeholder"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusOK {
		fmt.Println(string(body))
		panic(res.StatusCode)
	}

	re := regexp.MustCompile(`"(.+)"$`)
	matches := re.FindStringSubmatch(res.Header["Content-Disposition"][0])
	fileName := matches[1]

	return fileName, &body
}

func createPdf(fileName string, content *[]byte, path string) {
	pdf, err := os.Create(path + "/" + fileName)
	if err != nil {
		panic(err)
	}
	defer pdf.Close()

	_, err = pdf.Write(*content)
	if err != nil {
		panic(err)
	}
}
