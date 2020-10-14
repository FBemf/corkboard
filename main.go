package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/julienschmidt/httprouter"
)

// TODO grab from CLI args
var TEMPLATES_PATH = "templates/"
var NOTE_PATH = "notes/"
var STATIC_PATH = "static/"
var USERNAME = "user"
var PASSWORD_FILE = "password.txt"
var PASSWORD_VALUE = ""
var TOKEN_EXPIRY = 10 * time.Minute

var Templates = makeTemplates()
var Tokens = make(map[string]time.Time)

type IndexData struct {
	Token string
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
	router.GET("/", Auth(Index, USERNAME, PASSWORD_VALUE))
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

func makeToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	token := base64.StdEncoding.EncodeToString(b)
	Tokens["token "+token] = time.Now()
	return token, nil
}

func Auth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		if hasAuth && user == requiredUser && password == requiredPassword {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			auth := r.Header.Get("Authentication")
			val, ok := Tokens[auth]
			if ok && time.Since(val) < TOKEN_EXPIRY {
				h(w, r, ps)
			} else {
				if ok {
					delete(Tokens, auth)
				}
				// Request Basic Authentication otherwise
				w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		}
	}
}

func Index(resp http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
	token, err := makeToken()
	if err != nil {
		log.Printf("making token: %v", err)
		return
	}
	err = Templates.ExecuteTemplate(resp, "index.html", IndexData{Token: token})
	if err != nil {
		log.Printf("rendering page: %v", err)
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
