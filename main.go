package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var tmpl *template.Template

type storage struct {
	URLToID map[string]string
	IDToURL map[string]string
}

var db storage

var updated bool

var saveFile string

var mu sync.RWMutex

func init() {

	saveFile = os.Getenv("SAVEFILE")
	if saveFile == "" {
		log.Fatalf("No SAVEFILE env var provided")
	}

	// Initialize in case there's no JSON file.
	db = storage{URLToID: make(map[string]string), IDToURL: make(map[string]string)}
	loadSave()

	var err error
	tmpl, err = template.New("form").Parse(formTemplate)

	if err != nil {
		log.Fatalf("Failed to initialize template: %s", err)
	}
}

type rec struct {
	URL string
	ID  string
}

func main() {
	go toDisk()
	http.HandleFunc("/", index)
	log.Fatalf("Server failed: %s", http.ListenAndServe(":8001", nil))
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		post(w, r)
		return
	}
	path := r.URL.Path[1:]
	u, found := getURL(path)
	if found && strings.HasPrefix(u, "http") {
		http.Redirect(w, r, u, http.StatusFound)
	}
	msg := u
	if path != "" && msg == "" {
		msg = fmt.Sprintf("No match found for %q", path)
	}
	tmpl.Execute(w, msg)

}

func post(w http.ResponseWriter, r *http.Request) {
	u := strings.TrimSpace(r.FormValue("url"))
	id := getID(u)
	tmpl.Execute(w, fmt.Sprintf("ID: %s", id))
}

func getID(u string) string {
	mu.RLock()
	id, found := db.URLToID[u]
	mu.RUnlock()
	if found {
		return id
	}
	return createID(u)
}

func getURL(id string) (string, bool) {
	mu.RLock()
	u, found := db.IDToURL[id]
	mu.RUnlock()
	return u, found
}

func createID(u string) string {
	mu.Lock()
	defer mu.Unlock()
	id := fmt.Sprintf("%#x", len(db.URLToID)+1)[2:]
	db.URLToID[u] = id
	db.IDToURL[id] = u
	updated = true
	return id
}

var formTemplate = `<!DOCTYPE html>
<html>
    <body>
    <p>URL Shortener</p>
    <p>{{ . }}</p>
        <form method="POST" action="/">
            <input type="text" name="url" id="url">
            <input type="submit" name="btnSubmit" id="btnSubmit" value="submit">
        </form>
    </body>
</html>`

func toDisk() {
	for {
		time.Sleep(time.Second * 5)
		var doStuff bool
		mu.RLock()
		doStuff = updated
		if !doStuff {
			mu.RUnlock()
			continue
		}
		j, err := json.Marshal(db)
		if err != nil {
			log.Printf("Failed to marshal DB: %s", err)
			mu.RUnlock()
			continue
		}
		err = ioutil.WriteFile(saveFile, j, 0644)
		if err != nil {
			log.Printf("Failed to write to storage: %s", err)
		}
		mu.RUnlock()
		mu.Lock()
		updated = false
		mu.Unlock()
	}
}

func loadSave() {
	mu.Lock()
	defer mu.Unlock()

	b, err := ioutil.ReadFile(saveFile)
	if err != nil {
		log.Printf("Failed to open %s: %s", saveFile, err)
	}
	var data storage
	err = json.Unmarshal(b, &data)
	if err != nil {
		log.Printf("Failed to unmarshal JSON: %s", err)
		return
	}
	db = data
}
