package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

type srcDir struct {
	Path  string
	Files []srcFile
	Dirs  []srcDir
}

type srcFile struct {
	Path string
	Size int64
}

func (sf srcFile) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"name": "%s", "size": %d}`, html.EscapeString(sf.Path), sf.Size)), nil
}

func (sd srcDir) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{\n")
	_, err := buffer.WriteString(fmt.Sprintf(` "name": "%s"`, html.EscapeString(strings.ReplaceAll(sd.Path, "\\", "\\\\"))))
	if err != nil {
		return nil, err
	}

	// handle children (always true)
	if len(sd.Files)+len(sd.Dirs) > 0 {
		// children opener
		_, err = buffer.WriteString(`,"children": [`)
		if err != nil {
			return nil, err
		}

		for i, f := range sd.Files {
			data, err := json.Marshal(f)
			if err != nil {
				return nil, err
			}
			_, err = buffer.WriteString(string(data))
			if err != nil {
				return nil, err
			}
			// not last
			if i+1 != len(sd.Files) {
				_, err = buffer.WriteString(",")
				if err != nil {
					return nil, err
				}

			}
		}
		if len(sd.Files) > 0 && len(sd.Dirs) > 0 {
			_, err = buffer.WriteString(",")
			if err != nil {
				return nil, err
			}
		}
		for i, f := range sd.Dirs {
			data, err := f.MarshalJSON()
			if err != nil {
				return nil, err
			}
			_, err = buffer.WriteString(string(data))
			if err != nil {
				return nil, err
			}
			// not last
			if i+1 != len(sd.Dirs) {
				_, err = buffer.WriteString(",")
				if err != nil {
					return nil, err
				}

			}
		}

		// children closer
		_, err = buffer.WriteString("\n]\n")
		if err != nil {
			return nil, err
		}
	}

	_, err = buffer.WriteString("}")
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (sd *srcDir) AddFile(f srcFile) error {
	// log.Printf("r.Path '%s' f.Path: '%s' \n", r.Path, f.Path)
	if !strings.Contains(f.Path, string(filepath.Separator)) {
		// add to current directory
		sd.Files = append(sd.Files, f)
		// log.Printf("ADDING f.Path '%s' to r.Path: '%s'\n", f.Path, r.Path)
		return nil
	}

	// else, need to do recursion
	pathSegs := strings.Split(f.Path, string(filepath.Separator))
	nextDirName := pathSegs[0]
	f.Path = strings.Join(pathSegs[1:], string(filepath.Separator))
	// log.Println("nextDirName:", nextDirName, "for:", f.Path)

	// check if already have the dir
	for i, d := range sd.Dirs {
		if d.Path == nextDirName {
			return sd.Dirs[i].AddFile(f)
		}
	}

	// didn't find dir. Need to make it
	nd := srcDir{Path: nextDirName}
	sd.Dirs = append(sd.Dirs, nd)
	return sd.Dirs[len(sd.Dirs)-1].AddFile(f)
}

func buildFileHeirchy(base string, wantedExts []string) (hierarchy srcDir, err error) {
	hierarchy.Path = ""
	err = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		if path == base {
			// if we "reached" the file which is the "dir" we wanted to crawl..
			// then that means we're crawling a file (not a dir)
			return fmt.Errorf("Attempting to scan single file")
		}
		if !isSrcFile(path, wantedExts) {
			return nil
		}
		relPath, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}

		hierarchy.AddFile(srcFile{Path: relPath, Size: info.Size()})
		return nil
	})

	return hierarchy, err
}

func netHandleDisplay(w http.ResponseWriter, r *http.Request, hierarchy srcDir, displayTitle string, errorStr string) {

	tmpl, err := template.ParseFiles("FileMap.html")
	if err != nil {
		log.Panic(err)
	}

	err = tmpl.Execute(w, struct {
		DisplayTitle string
		ErrorStr     string
	}{DisplayTitle: html.EscapeString(displayTitle), ErrorStr: html.EscapeString(errorStr)})
	if err != nil {
		log.Panic(err)
	}
}

func netHandleHierchyJSON(w http.ResponseWriter, r *http.Request, hierarchy srcDir, rootDir string) {
	temp := hierarchy.Path
	_, f := filepath.Split(rootDir)
	hierarchy.Path = f

	if len(hierarchy.Files) == 0 && len(hierarchy.Dirs) == 0 {
		hierarchy.Files = append(hierarchy.Files, srcFile{Path: "(No source code files found)", Size: 1})
	}

	data, err := json.Marshal(hierarchy)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, string(data))
	hierarchy.Path = temp
}

func netHandleCrawl(r *http.Request, rootDir *string, srcHierchy *srcDir, displayTitle *string) string {
	r.ParseForm()
	*rootDir = r.Form["scanPath"][0]
	wantedExts := strings.Split(r.Form["wantedExts"][0], " ")
	*displayTitle = "🔍 Visualizing " + *rootDir
	var err error
	log.Println("scanning", *rootDir)
	*srcHierchy, err = buildFileHeirchy(*rootDir, wantedExts)
	if err != nil {
		*displayTitle = "❌ Failed Visualizing " + *rootDir
		return "An error has occured. Ensure you entered a valid directory path. Error: " + err.Error()
	}
	return ""
}

func isSrcFile(p string, wantedExts []string) bool {
	pe := filepath.Ext(p)
	if wantedExts[0] == "*" {
		return true
	}
	for _, e := range wantedExts {
		if e == pe {
			return true
		}
	}
	return false
}

func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

func main() {
	log.Println("starting")

	unsetDirValue := `(No path set)`
	rootDir := unsetDirValue
	srcHierchy := srcDir{Files: []srcFile{srcFile{Size: 1, Path: rootDir}}}
	displayTitle := "🔍 Source Code Visualizer"

	// main page
	http.HandleFunc("/display", func(w http.ResponseWriter, r *http.Request) {
		errorString := ""
		if r.Method == "POST" {
			// crawl
			log.Println("handling /display POST")
			errorString = netHandleCrawl(r, &rootDir, &srcHierchy, &displayTitle)
		} else {
			log.Println("handling /display")
		}
		// display
		netHandleDisplay(w, r, srcHierchy, displayTitle, errorString)
	})

	// convert srcHierchy -> json format for js
	http.HandleFunc("/dirdata.json", func(w http.ResponseWriter, r *http.Request) {
		log.Println("handling /dirdata.json")
		netHandleHierchyJSON(w, r, srcHierchy, rootDir)
	})

	// start server & open browser
	serverAddr := "localhost:8080"
	err := openBrowser("http://" + serverAddr + "/display")
	if err != nil {
		log.Panic(err)
	}
	err = http.ListenAndServe(serverAddr, nil)
	if err != nil {
		log.Panic(err)
	}
}
