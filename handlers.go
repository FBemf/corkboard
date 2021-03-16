package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// creates an http router and registers all the endpoints
func makeRouter(templates *template.Template, static fs.FS, config Config, datastore Datastore) *httprouter.Router {
	router := httprouter.New()
	router.GET("/", Auth(Index(templates, datastore, config.numRecentNotes), config.credentials))
	router.GET("/note/:note", Auth(Note(templates, datastore), config.credentials))
	router.POST("/api/note/:note", Auth(SetNote(datastore, false), config.credentials))
	router.PUT("/api/note/:note", Auth(SetNote(datastore, true), config.credentials))
	router.DELETE("/api/note/:note", Auth(DeleteNote(datastore), config.credentials))
	router.GET("/api/note/:note", Auth(RawNote(datastore), config.credentials))
	router.ServeFiles("/static/*filepath", http.FS(static))
	return router
}

// basic authentication middleware
// disabled if credentials == ""
func Auth(h httprouter.Handle, credentials map[string]bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		_, credsValid := credentials[user+":"+password]
		if (hasAuth && credsValid) || credentials == nil {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			ErrorPage(w, http.StatusUnauthorized)
		}
	}
}

// IndexData is passed to the index.html template
type IndexData struct {
	RecentNotes []string
}

// NoteData is passed to the note.html template
type NoteData struct {
	Title string
	Body  string
}

// displays index page
// numRecentPosts is the number of recent posts to display
func Index(templates *template.Template, datastore Datastore, numRecentPosts int) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
		recentNotes, err := datastore.getLatestNotes(numRecentPosts)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("getting recent posts: %v", err)
			return
		}
		err = templates.ExecuteTemplate(resp, "index.html", IndexData{RecentNotes: recentNotes})
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("rendering page: %v", err)
		}
	}
}

// displays a note on a pretty html page
func Note(templates *template.Template, datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		data, ok, err := datastore.getNote(noteName)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("accessing %s: %v", noteName, err)
			return
		}
		if !ok {
			ErrorPage(resp, http.StatusNotFound)
			return
		}
		resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
		err = templates.ExecuteTemplate(resp, "note.html",
			NoteData{Title: noteName, Body: string(data)})
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("writing template: %v", err)
			return
		}
	}
}

// displays a note entirely raw. good for binaries or curl
func RawNote(datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		data, ok, err := datastore.getNote(noteName)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("accessing %s: %v", noteName, err)
			return
		}
		if !ok {
			ErrorPage(resp, http.StatusNotFound)
			return
		}
		resp.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		_, err = resp.Write(data)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("responding with raw file: %v", err)
		}
	}
}

// posts a note
// request body is the note, not a json
func SetNote(datastore Datastore, clobber bool) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		body := bytes.NewBuffer(nil)
		_, err := body.ReadFrom(req.Body)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("error reading request body: %v", err)
			return
		}
		status, err := datastore.setNote(noteName, body.Bytes(), clobber)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("error writing note %s: %v", noteName, err)
			return
		}
		if status == NO_CLOBBER {
			ErrorPage(resp, http.StatusConflict)
			return
		} else if status == CREATED {
			ErrorPage(resp, http.StatusCreated)
			log.Printf("New note %s", noteName)
			return
		}
		log.Printf("Updated note %s", noteName)
	}
}

// handles note deletion
func DeleteNote(datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		err := datastore.deleteNote(noteName)
		if err != nil {
			ErrorPage(resp, http.StatusInternalServerError)
			log.Printf("error writing note %s: %v", noteName, err)
			return
		}
		log.Printf("Deleted note %s", noteName)
	}
}

func ErrorPage(resp http.ResponseWriter, code int) {
	http.Error(resp, fmt.Sprintf("%d %s", code, http.StatusText(code)), code)
}
