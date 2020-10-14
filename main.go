package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path"

	"github.com/julienschmidt/httprouter"
)

// TODO grab from CLI args
var TEMPLATES_PATH = "templates/"
var NOTE_PATH = "notes/"
var STATIC_PATH = "static/"
var PASSWORD_FILE = "password.txt"
var PASSWORD_VALUE = ""

var Templates = makeTemplates()

type IndexData struct {
}

type NoteData struct {
	Title string
	Body  string
}

/*
type PostNoteData struct {
	Body     string
	Password string
}
*/

func main() {
	if b, err := ioutil.ReadFile(PASSWORD_FILE); err == nil {
		PASSWORD_VALUE = string(b)
	} else {
		log.Fatalf("couldn't read password from %s", PASSWORD_FILE)
	}

	router := httprouter.New()
	router.GET("/", Index)
	router.GET("/note/:note", Note)
	router.POST("/note/:note", PostNote)
	router.GET("/raw/:note", RawNote)
	router.ServeFiles("/static/*filepath", http.Dir(STATIC_PATH))
	log.Print("Running")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func makeTemplates() template.Template {
	t, err := template.ParseGlob(path.Join(TEMPLATES_PATH, "*"))
	if err != nil {
		log.Fatal(err)
	}
	return *t
}

func Index(resp http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
	err := Templates.ExecuteTemplate(resp, "index.html", IndexData{})
	if err != nil {
		log.Print(err)
	}
}

func Note(resp http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	notePath := path.Join(NOTE_PATH, params[0].Value)
	data, err := ioutil.ReadFile(notePath)
	if err != nil {
		log.Printf("reading %s: %v", notePath, err)
		return
	}
	resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
	err = Templates.ExecuteTemplate(resp, "note.html", NoteData{Title: params[0].Value, Body: string(data)})
	if err != nil {
		log.Printf("writing template: %v", err)
	}
}

func RawNote(resp http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	notePath := path.Join(NOTE_PATH, params[0].Value)
	data, err := ioutil.ReadFile(notePath)
	if err != nil {
		log.Printf("reading %s: %v", notePath, err)
		return
	}
	resp.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	_, err = resp.Write(data)
	if err != nil {
		log.Printf("writing raw text: %v", err)
	}
}

func PostNote(rsp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	body := bytes.NewBuffer(nil)
	_, err := body.ReadFrom(req.Body)
	if err != nil {
		log.Printf("error reading request body: %v", err)
		return
	}
	notePath := path.Join(NOTE_PATH, params[0].Value)
	if req.Header.Get("X-Password") != "\""+PASSWORD_VALUE+"\"" {
		rsp.WriteHeader(http.StatusForbidden)
		log.Printf("unauthorized attempt to create %s", notePath)
		return
	}
	err = ioutil.WriteFile(notePath, body.Bytes(), 0755)
	if err != nil {
		log.Printf("error writing to file %s: %v", notePath, err)
		return
	}
	log.Printf("New note %s", notePath)
}
