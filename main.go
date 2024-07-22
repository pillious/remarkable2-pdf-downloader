package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	backupsDir  string
	baseUrl     string
	includeList listFlags
	excludeList listFlags
	doLogToFile bool
	verbose     bool

	logFileName                 = ".backup.logs"
	lastModifiedFileName string = ".last_modified.json"
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
	parseFlags()
	makeDir(backupsDir)

	if doLogToFile {
		setupFileLogger(logFileName)
	}
	defer closeFileLogger()

	includeSet := sliceToSet(includeList)
	excludeSet := sliceToSet(excludeList)
	modificationTimes := getLastModifiedTimes()

	if verbose {
		log.Println("Starting Downloads.")
		log.Println("-------------------")
	}

	traverseFileSystem(Stack[string]{}, "", &includeSet, &excludeSet, modificationTimes)
	updateLastModifiedTimes(modificationTimes)

	if verbose {
		log.Println("-------------------")
		log.Println("Downloads complete.")
	}
}

func parseFlags() {
	flag.StringVar(&backupsDir, "backupsDir", ".", "The directory to store backups.")
	flag.StringVar(&baseUrl, "url", "http://10.11.99.1:80", "Address of the Remarkable2 web UI.")

	flag.Var(&includeList, "i", "A document/folder name or id to download. Add this flag multiple times to include multiple items.")
	flag.Var(&excludeList, "e", "A document/folder name or id to skip. Add this flag multiple times to include multiple items. (Note: The exclude flag has higher precedence than the include flag)")

	flag.BoolVar(&doLogToFile, "l", false, "Write logs to {backupsDir}/backup.logs instead of STDOUT.")
	flag.BoolVar(&verbose, "v", false, "Enable verbose output.")

	flag.Parse()
}

func traverseFileSystem(path Stack[string], folderId string, include *map[string]struct{}, exclude *map[string]struct{}, modificationTimes *map[string]string) {
	isRoot := folderId == ""
	pathStr := strings.Join(path, "/")

	makeDir(backupsDir + "/" + pathStr)

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
		if !didModificationTimeChange(doc.Id, doc.UpdatedAt, modificationTimes) {
			continue
		}

		switch doc.Type {
		case "DocumentType":
			if _, ok := (*include)[doc.Id]; !ok {
				if _, ok := (*include)[doc.Name]; !ok {
					continue
				}
			}

			if verbose {
				if isRoot {
					log.Printf("%s ...\n", doc.Name)
				} else {
					log.Printf("%s/%s ...\n", pathStr, doc.Name)
				}
			}

			startTime := time.Now()
			pdfName, content := downloadDocument(doc.Id)
			duration := time.Since(startTime).Seconds()
			if verbose {
				log.Printf("time: %.2fs | size: %s\n", duration, formatSize(len(*content)))
			}

			createPdf(pdfName, content, backupsDir+"/"+pathStr)
			(*modificationTimes)[doc.Id] = doc.UpdatedAt

		case "CollectionType":
			path.Push(doc.Name)
			traverseFileSystem(path, doc.Id, include, exclude, modificationTimes)
			removeEmptyDir(backupsDir + "/" + strings.Join(path, "/"))
			path.Pop()
		}
	}
}

func getDocumentsAtRoot() *[]Document {
	return getDocuments("")
}

func getDocuments(folderId string) *[]Document {
	url := baseUrl + "/documents/" + folderId

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Panic(err)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Panic(err)
	}
	defer res.Body.Close()

	docs := &[]Document{}
	err = json.NewDecoder(res.Body).Decode(docs)
	if err != nil {
		log.Panic(err)
	}
	if res.StatusCode != http.StatusOK {
		log.Panic(res.StatusCode)
	}

	return docs
}

func downloadDocument(documentId string) (string, *[]byte) {
	url := baseUrl + "/download/" + documentId + "/placeholder"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Panic(err)
	}

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		log.Panic(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Panic(err)
	}
	if res.StatusCode != http.StatusOK {
		log.Println(string(body))
		log.Panic(res.StatusCode)
	}

	re := regexp.MustCompile(`"(.+)"$`)
	matches := re.FindStringSubmatch(res.Header["Content-Disposition"][0])
	fileName := matches[1]

	return fileName, &body
}

func getLastModifiedTimes() *map[string]string {
	path := backupsDir + "/" + lastModifiedFileName
	result := &map[string]string{}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(result)
	if err == io.EOF {
		return result
	} else if err != nil {
		log.Panic(err)
	}

	return result
}

func updateLastModifiedTimes(modificationTimes *map[string]string) {
	path := backupsDir + "/" + lastModifiedFileName

	file, err := os.Create(path)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	result, err := json.Marshal(modificationTimes)
	if err != nil {
		log.Panic(err)
	}
	file.WriteString(string(result))
}

func didModificationTimeChange(id string, newTime string, modificationTimes *map[string]string) bool {
	if _, ok := (*modificationTimes)[id]; !ok {
		return true
	}

	layout := "2006-01-02T15:04:05.000Z"
	t1, err := time.Parse(layout, (*modificationTimes)[id])
	if err != nil {
		log.Println(err)
		return true
	}
	t2, err := time.Parse(layout, newTime)
	if err != nil {
		log.Println(err)
		return true
	}

	return t1.Before(t2)
}
