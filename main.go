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

/*
TODO:
[-] handle file moves/renames
[-] handle folder moves/renames
[] remove deleted files
[] remove deleted folders
[-] handle download files inside an included folder
*/

/*
NOTES:

include/exclude sets -- folders denoted with "/" at the end
folder eg: test/ test/test2/
file eg: abc test/abc

don't allow ids in include/exclude sets
*/

var (
	// -- Flags --
	backupsDir  string // converted to absolute path after the value is read in.
	baseUrl     string
	includeList listFlags
	excludeList listFlags
	doLogToFile bool
	verbose     bool
	// -- Flags --

	logFileName               = ".backup.logs"
	backupInfoFileName string = ".backup_info.json"
)

type Document struct {
	Id        string `json:"ID"`
	UpdatedAt string `json:"ModifiedClient"`
	Parent    string `json:"Parent"`
	Type      string `json:"Type"`
	Name      string `json:"VissibleName"`
	Size      string `json:"sizeInBytes"`
	// Only defined a subset of the fields returned from the API
}

type DocumentBackup struct {
	Name      string `json:"name"`
	UpdatedAt string `json:"updatedAt"`
	Path      string `json:"path"`
	Size      uint64 `json:"size"`
	IsFolder  bool   `json:"isFolder"`
}

type DocumentBackupMap map[string]DocumentBackup

func main() {
	parseFlags()
	makeDir(backupsDir)

	if doLogToFile {
		setupFileLogger(logFileName)
	}
	defer closeFileLogger()

	includeSet := sliceToSet(includeList)
	excludeSet := sliceToSet(excludeList)
	prevBackupInfo := getPrevBackupInfo()

	if verbose {
		log.Println("Starting Downloads.")
		log.Println("-------------------")
	}

	traverseFileSystem(Stack[string]{}, "", &includeSet, &excludeSet, prevBackupInfo)
	writeBackupInfo(prevBackupInfo)

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

func traverseFileSystem(path Stack[string], folderId string, include *map[string]struct{}, exclude *map[string]struct{}, backupMap *DocumentBackupMap) {
	log.Println(path)
	isRoot := folderId == ""
	relPathStr := "."
	if !path.isEmpty() {
		relPathStr = strings.Join(path, "/")
	}
	absPathStr := genFullPath(strings.Join(path, "/"))

	makeDir(absPathStr)

	var docs []Document
	if isRoot {
		docs = *getDocumentsAtRoot()
	} else {
		docs = *getDocuments(folderId)
	}

	for _, doc := range docs {
		doc.UpdatedAt = stripMsecFromTime(doc.UpdatedAt)

		name := doc.Name
		if relPathStr != "." {
			name = relPathStr + "/" + name
		}
		if doc.Type == "CollectionType" {
			name += "/"
		}

		if _, ok := (*exclude)[name]; ok {
			continue
		}
		if len((*include)) != 0 {
			if _, ok := (*include)[name]; !ok && !inIncludedFolder(include, path) {
				continue
			}
		}

		switch doc.Type {
		case "DocumentType":
			if isDocMoved(doc.Id, doc.Name, relPathStr, backupMap) {
				savedPath := (*backupMap)[doc.Id].Path + "/" + (*backupMap)[doc.Id].Name
				oldPath := genFullPath(savedPath + ".pdf")
				newPath := absPathStr + "/" + doc.Name + ".pdf"

				err := os.Rename(oldPath, newPath)
				if err != nil {
					log.Panic(err)
				}
				if verbose {
					log.Printf("Moved notebook '%s' to '%s'.\n", savedPath, relPathStr+"/"+doc.Name+".pdf")
				}
			}

			if isNotebookModified(doc.Id, doc.UpdatedAt, strToUint64(doc.Size), backupMap) {
				if verbose {
					if isRoot {
						log.Printf("%s ...\n", doc.Name)
					} else {
						log.Printf("%s/%s ...\n", relPathStr, doc.Name)
					}
				}

				startTime := time.Now()
				pdfName, content := downloadDocument(doc.Id)
				duration := time.Since(startTime).Seconds()
				if verbose {
					log.Printf("time: %.2fs | size: %s\n", duration, formatSize(len(*content)))
				}

				createPdf(pdfName, content, absPathStr)
			}

			(*backupMap)[doc.Id] = DocumentBackup{doc.Name, doc.UpdatedAt, relPathStr, strToUint64(doc.Size), false}
		case "CollectionType":
			symlinkPath := ""
			if isDocMoved(doc.Id, doc.Name, relPathStr, backupMap) {
				savedPath := (*backupMap)[doc.Id].Path + "/" + (*backupMap)[doc.Id].Name
				oldPath := genFullPath(savedPath)
				newPath := absPathStr + "/" + doc.Name

				err := os.Rename(oldPath, newPath)
				if err != nil {
					log.Panic(err)
				}
				if verbose {
					log.Printf("Moved folder '%s/' -> '%s/'.\n", savedPath, relPathStr+"/"+doc.Name)
				}

				err = os.Symlink(newPath, oldPath)
				if err != nil {
					log.Panic(err)
				}
				symlinkPath = oldPath
			}

			path.Push(doc.Name)
			traverseFileSystem(path, doc.Id, include, exclude, backupMap)
			if symlinkPath != "" {
				err := os.Remove(symlinkPath)
				if err != nil {
					log.Panic(err)
				}
			}
			// TODO: do i need this??
			// didRemove := removeEmptyDir(genFullPath(strings.Join(path, "/")))
			// if !didRemove {
			// 	(*backupMap)[doc.Id] = DocumentBackup{doc.Name, doc.UpdatedAt, relPathStr, 0, true}
			// }
			(*backupMap)[doc.Id] = DocumentBackup{doc.Name, doc.UpdatedAt, relPathStr, 0, true}
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

func getPrevBackupInfo() *DocumentBackupMap {
	path := genFullPath(backupInfoFileName)
	backupInfo := &DocumentBackupMap{}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(backupInfo)
	if err == io.EOF {
		return backupInfo
	} else if err != nil {
		log.Panic(err)
	}
	return backupInfo
}

func writeBackupInfo(backupInfo *DocumentBackupMap) {
	path := genFullPath(backupInfoFileName)
	file, err := os.Create(path)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	result, err := json.Marshal(backupInfo)
	if err != nil {
		log.Panic(err)
	}
	file.WriteString(string(result))
}

func isDocMoved(id string, newName string, newPath string, backupMap *DocumentBackupMap) bool {
	if entry, ok := (*backupMap)[id]; ok {
		return entry.Name != newName || entry.Path != newPath || (entry.Path == "" && newPath == ".")
	}
	return false
}

// checks modification time AND size change b/c renaming a doc also updates the mod time.
func isNotebookModified(id string, newTime string, newSize uint64, backupMap *DocumentBackupMap) bool {
	if entry, ok := (*backupMap)[id]; ok {
		if !entry.IsFolder {
			return didModificationTimeChange(id, newTime, backupMap) && didSizeChange(id, newSize, backupMap)
		}
	}
	return true
}

func didSizeChange(id string, newSize uint64, backupMap *DocumentBackupMap) bool {
	return (*backupMap)[id].Size != newSize
}

func stripMsecFromTime(time string) string {
	i := strings.LastIndex(time, ".")
	return time[:i]
}

func didModificationTimeChange(id string, newTime string, backupMap *DocumentBackupMap) bool {
	layout := "2006-01-02T15:04:05"
	t1, err := time.Parse(layout, (*backupMap)[id].UpdatedAt)
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

func inIncludedFolder(include *map[string]struct{}, path []string) bool {
	for k := range *include {
		if k[len(k)-1:] == "/" {
			kSplit := strings.Split(k, "/")
			if len(kSplit)-1 <= len(path) {
				for i := 0; i < len(kSplit)-1; i++ {
					if kSplit[i] != path[i] {
						break
					}
				}
				return true
			}
		}
	}
	return false
}
