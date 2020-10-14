package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// creates an http router and registers all the endpoints
func makeRouter(templates template.Template, config Config, datastore Datastore) *httprouter.Router {
	router := httprouter.New()
	router.GET("/", Auth(Index(templates, datastore, config.numRecentNotes), config.credentials))
	router.GET("/note/:note", Auth(Note(templates, datastore), config.credentials))
	router.POST("/note/:note", Auth(PostNote(datastore), config.credentials))
	router.DELETE("/note/:note", Auth(DeleteNote(datastore), config.credentials))
	router.GET("/raw/:note", Auth(RawNote(datastore), config.credentials))
	router.ServeFiles("/static/*filepath", http.Dir(config.staticPath))
	return router
}

// basic authentication middleware
// disabled if credentials == ""
func Auth(h httprouter.Handle, credentials string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		if (hasAuth && user+":"+password == credentials) || credentials == "" {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
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
func Index(templates template.Template, datastore Datastore, numRecentPosts int) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
		recentNotes, err := datastore.getLatestNotes(numRecentPosts)
		if err != nil {
			ErrCode(http.StatusInternalServerError, "Internal Server Error", resp, req)
			log.Printf("getting recent posts: %v", err)
			return
		}
		err = templates.ExecuteTemplate(resp, "index.html", IndexData{RecentNotes: recentNotes})
		if err != nil {
			ErrCode(http.StatusInternalServerError, "Internal Server Error", resp, req)
			log.Printf("rendering page: %v", err)
		}
	}
}

// displays a note on a pretty html page
func Note(templates template.Template, datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		data, ok, err := datastore.getNote(noteName)
		if err != nil {
			ErrCode(http.StatusInternalServerError, "Internal Server Error", resp, req)
			log.Printf("accessing %s: %v", noteName, err)
			return
		}
		if !ok {
			ErrCode(http.StatusNotFound, "Not Found", resp, req)
			return
		}
		resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
		err = templates.ExecuteTemplate(resp, "note.html",
			NoteData{Title: noteName, Body: string(data)})
		if err != nil {
			log.Printf("writing template: %v", err)
		}
	}
}

// displays a note entirely raw. good for binaries or curl
func RawNote(datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		data, ok, err := datastore.getNote(noteName)
		if err != nil {
			ErrCode(http.StatusInternalServerError, "Internal Server Error", resp, req)
			log.Printf("accessing %s: %v", noteName, err)
			return
		}
		if !ok {
			ErrCode(http.StatusNotFound, "Not Found", resp, req)
		}
		resp.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		_, err = resp.Write(data)
		if err != nil {
			log.Printf("responding with raw file: %v", err)
		}
	}
}

// posts a note
// request body is the note, not a json
func PostNote(datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		body := bytes.NewBuffer(nil)
		_, err := body.ReadFrom(req.Body)
		if err != nil {
			ErrCode(http.StatusInternalServerError, "Internal Server Error", resp, req)
			log.Printf("error reading request body: %v", err)
			return
		}
		ok, err := datastore.createNote(noteName, body.Bytes())
		if err != nil {
			ErrCode(http.StatusInternalServerError, "Internal Server Error", resp, req)
			log.Printf("error writing note %s: %v", noteName, err)
			return
		}
		if !ok {
			ErrCode(http.StatusConflict, "Conflict", resp, req)
		}
		log.Printf("New note %s", noteName)
	}
}

// handles note deletion
func DeleteNote(datastore Datastore) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		noteName := params[0].Value
		err := datastore.deleteNote(noteName)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			log.Printf("error writing note %s: %v", noteName, err)
			return
		}
		log.Printf("Deleted note %s", noteName)
	}
}

// simple handler to display an error code to the client
func ErrCode(code int, msg string, resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	_, err := resp.Write([]byte(fmt.Sprintf("%d %s", code, msg)))
	if err != nil {
		log.Printf("writing error response: %v", err)
	}
}
