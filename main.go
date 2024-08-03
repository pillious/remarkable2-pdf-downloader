package main

import (
	"encoding/json"
	"errors"
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

var (
	// -- Flags --
	backupsDir  string
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
	visited := &Set[string]{}

	if verbose {
		log.Println("Begin Remarkable PDF backups.")
		log.Println("-------------------")
	}

	backupDocsAsPdfs(Stack[string]{}, "", &includeSet, &excludeSet, prevBackupInfo, visited)
	// syncDeletedDocs(prevBackupInfo, visited, &includeSet, &excludeSet)
	writeBackupInfo(prevBackupInfo)

	if verbose {
		log.Println("-------------------")
		log.Println("Complete Remarkable PDF backups.")
	}

}

func parseFlags() {
	flag.StringVar(&backupsDir, "backupsDir", ".", "The directory to store backups.")
	flag.StringVar(&baseUrl, "url", "http://10.11.99.1:80", "Address of the Remarkable2 web UI.")

	flag.Var(&includeList, "i", "A notebook/folder `path` to download. All paths are assumed to start from the root folder of your Remarkable.\nExample - path to notebook: foo/bar/mynotebook\nExample - path to folder: foo/bar/ (trailing forward slash REQUIRED)\n(Add this flag multiple times to include multiple items.)\n")
	flag.Var(&excludeList, "e", "A notebook/folder `path` to skip. All paths are assumed to start from the root folder of your Remarkable.\nExample - path to notebook: foo/bar/mynotebook\nExample - path to folder: foo/bar/ (trailing forward slash REQUIRED)\n(Add this flag multiple times to skip multiple items.)\n")

	flag.BoolVar(&doLogToFile, "l", false, "Write logs to {backupsDir}/.backup.logs instead of STDOUT.")
	flag.BoolVar(&verbose, "v", false, "Enable verbose output.")

	flag.Parse()
}

func backupDocsAsPdfs(path Stack[string], folderId string, include *Set[string], exclude *Set[string], prevBackupMap *DocumentBackupMap, visited *Set[string]) {
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

		if exclude.has(name) {
			continue
		} else if !include.isEmpty() && !include.has(name) && !inIncludedFolder(include, path) {
			continue
		}

		visited.add(doc.Id)

		switch doc.Type {
		case "DocumentType":
			if isDocMoved(doc.Id, doc.Name, relPathStr, prevBackupMap) {
				savedPath := (*prevBackupMap)[doc.Id].Path + "/" + (*prevBackupMap)[doc.Id].Name
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

			if isNotebookModified(doc.Id, doc.UpdatedAt, strToUint64(doc.Size), prevBackupMap) {
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

			(*prevBackupMap)[doc.Id] = DocumentBackup{doc.Name, doc.UpdatedAt, relPathStr, strToUint64(doc.Size), false}
		case "CollectionType":
			symlinkPath := ""
			if isDocMoved(doc.Id, doc.Name, relPathStr, prevBackupMap) {
				savedPath := (*prevBackupMap)[doc.Id].Path + "/" + (*prevBackupMap)[doc.Id].Name
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
			backupDocsAsPdfs(path, doc.Id, include, exclude, prevBackupMap, visited)
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
			(*prevBackupMap)[doc.Id] = DocumentBackup{doc.Name, doc.UpdatedAt, relPathStr, 0, true}
			path.Pop()
		}
	}
}

func syncDeletedDocs(prevBackupInfo *DocumentBackupMap, visited *Set[string], include *Set[string], exclude *Set[string]) {
	log.Println(visited)
	log.Println(include)
	log.Println(exclude)
	log.Println(prevBackupInfo)

	keySet := Set[string]{}
	for k := range *prevBackupInfo {
		keySet.add(k)
	}
	notVisited := keySet.difference(visited)

	for id := range notVisited {
		// TODO: dont delete if notVisited_id is in the exclusion set.
		// TODO: dont delete if notVisited_id is not in the inclusion set.
		// TODO: update the prev backupinfo -- remove any keys that have been deleted.

		if entry, ok := (*prevBackupInfo)[id]; ok {
			relPath := "."
			if entry.Path != "" {
				relPath = entry.Path + "/" + entry.Name
			}
			if entry.IsFolder {
				relPath += "/"
			}
			absPath := genFullPath(entry.Path + "/" + entry.Name)

			if _, err := os.Stat(absPath); err == nil {
				err = os.RemoveAll(absPath)
				if err != nil {
					log.Panic(err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				log.Panic(err)
			}

			delete(*prevBackupInfo, id)
			if verbose {
				log.Printf("Removed %s\n.", relPath)
			}
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

	// Content-Disposition header contains the notebook name.
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

// Checks if a document has been renamed and/or moved.
func isDocMoved(id string, newName string, newPath string, backupMap *DocumentBackupMap) bool {
	if entry, ok := (*backupMap)[id]; ok {
		return entry.Name != newName || entry.Path != newPath || (entry.Path == "" && newPath == ".")
	}
	return false
}

// Checks modification time AND size change b/c renaming a doc also updates the mod time.
func isNotebookModified(id string, newTime string, newSize uint64, backupMap *DocumentBackupMap) bool {
	if entry, ok := (*backupMap)[id]; ok {
		if !entry.IsFolder {
			return didModificationTimeChange(id, newTime, backupMap) && didSizeChange(id, newSize, backupMap)
		}
	}
	return true
}

// Assumes id is in backupMap
func didSizeChange(id string, newSize uint64, backupMap *DocumentBackupMap) bool {
	return (*backupMap)[id].Size != newSize
}

// Strips milliseconds and trailing Z from ISO 8601 timestamp
func stripMsecFromTime(time string) string {
	i := strings.LastIndex(time, ".")
	return time[:i]
}

// Assumes id is in backupMap
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

func inIncludedFolder(include *Set[string], path []string) bool {
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
