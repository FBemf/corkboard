package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

const (
	// storage modes
	// if storage mode is "database," notes are stored in a sqlite db
	// if storage mode is "flatFile," notes are stored as text files
	database = iota
	flatFile
)

// delete expired notes every hour
const cleanupInterval = time.Hour

func main() {
	config := parseArgs()
	templates := makeTemplates(config.templatePath)

	var datastore Datastore
	if config.storageMode == database {
		// set up sqlite database
		db, err := sql.Open("sqlite3", config.databasePath)
		if err != nil {
			log.Fatalf("error opening db %s", config.databasePath)
		}
		defer db.Close()
		datastore = Datastore{
			database,
			db,
			"",
		}
		if config.noteExpiryTime != 0 {
			// begin deleting expired notes every hour
			go func() {
				for {
					time.Sleep(cleanupInterval)
					datastore.deleteOldNotes(config.noteExpiryTime)
				}
			}()
		}
	} else {
		// get ready to store notes as text files
		err := os.MkdirAll(config.notePath, 0755)
		if err != nil {
			log.Fatalf("creating note directory: %s", config.notePath)
		}
		datastore = Datastore{
			flatFile,
			nil,
			config.notePath,
		}
	}

	router := makeRouter(templates, config, datastore)
	log.Print("Running")
	log.Fatal(http.ListenAndServe(":"+str(config.port), router))
}

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

// Config stores data derived from the command line arguments
type Config struct {
	templatePath   string
	notePath       string
	staticPath     string
	databasePath   string
	credentials    string
	port           int
	noteExpiryTime time.Duration
	numRecentNotes int
	storageMode    int // database or flatFile
}

// parses command line arguments
// all configs are passed in through here
func parseArgs() Config {
	config := Config{}
	flag.StringVar(&config.databasePath, "db-path", "./notes.db", "Path to the sqlite db")
	credentialFile := flag.String("creds-file", "", "Path to a file holding login credentials in the form \"username:password\"")
	flag.StringVar(&config.credentials, "creds", "", "Access credentials in the form \"username:password\". Overrides -creds-file")
	flag.IntVar(&config.port, "port", 8080, "Port to serve the application on")
	noteExpiryTime := flag.Int("note-expiry", 7, "Notes which have not been viewed in this many days will be deleted. If set to zero, notes never expire.")
	flag.StringVar(&config.templatePath, "template-path", "./templates/", "Path to the directory where html templates are stored")
	flag.StringVar(&config.staticPath, "static-path", "./static/", "Path to the directory where static assets are stored")
	flag.IntVar(&config.numRecentNotes, "recent-notes", 8, "Display this many recent notes on the main page")
	flag.StringVar(&config.notePath, "note-path", "", "Path to the directory where the notes are stored. If this is set, store notes as flat files instead of in a db")
	flag.Parse()

	if *noteExpiryTime < 0 {
		log.Fatal("bad arguments: -note-expiry must be non-negative")
	} else {
		// convert from number of hours into time.Duration
		config.noteExpiryTime = time.Duration(*noteExpiryTime*24) * time.Hour
	}

	if config.numRecentNotes < 0 {
		log.Fatal("bad arguments: -recent-notes must be non-negative")
	}

	// storageMode is based on the presence of notePath
	if config.notePath == "" {
		config.storageMode = database
	} else {
		config.storageMode = flatFile
	}

	if *credentialFile != "" && config.credentials == "" {
		b, err := ioutil.ReadFile(*credentialFile)
		if err != nil {
			log.Fatalf("bad arguments: unable to open credential file %s", *credentialFile)
		}
		config.credentials = string(b)
	}

	return config
}

// builds all the templates under the templates directory
func makeTemplates(templatePath string) template.Template {
	t, err := template.ParseGlob(path.Join(templatePath, "*"))
	if err != nil {
		log.Fatal(err)
	}
	return *t
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

// simple handler to display an error code to the client
func ErrCode(code int, msg string, resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	_, err := resp.Write([]byte(fmt.Sprintf("%d %s", code, msg)))
	if err != nil {
		log.Printf("writing error response: %v", err)
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

type Datastore struct {
	mode      int     // database or flatFile
	database  *sql.DB // nil if mode == flatFile
	directory string  // "" if mode == database
}

func (ds *Datastore) getNote(name string) ([]byte, bool, error) {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		data, err := ioutil.ReadFile(notePath)
		return data, true, err
	} else { // database
		row := ds.database.QueryRow(`SELECT (Body) FROM Notes WHERE Name = ?`, name)
		buf := []byte{}
		if err := row.Scan(&buf); err != nil {
			if err == sql.ErrNoRows {
				return nil, false, nil
			} else {
				return nil, false, err
			}
		}
		_, err := ds.database.Exec(
			`UPDATE Notes SET LastViewed = datetime("now") WHERE Name = ?`, name)
		if err != nil {
			return buf, true, err
		}
		return buf, true, nil
	}
}

func (ds *Datastore) createNote(name string, body []byte) (bool, error) {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		if _, err := os.Stat(notePath); err == nil {
			return false, nil
		}
		err := ioutil.WriteFile(notePath, body, 0755)
		return true, err
	} else { //database
		_, err := ds.database.Exec(`INSERT INTO Notes (Name, Body)
			VALUES (?, ?)`, name, body)
		if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return true, err
	}
}

func (ds *Datastore) deleteNote(name string) error {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		err := os.Remove(notePath)
		return err
	} else { //database
		_, err := ds.database.Exec("DELETE FROM Notes WHERE Name = ?", name)
		return err
	}
}

// gets the `maxNotes` most recently-created notes
func (ds *Datastore) getLatestNotes(maxNotes int) ([]string, error) {
	var names = make([]string, 0)
	if ds.mode == database {
		rows, err := ds.database.Query(
			"SELECT (Name) FROM Notes ORDER BY Created DESC LIMIT ?", maxNotes)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			if err != nil {
				return names, err
			}
			names = append(names, name)
		}
		err = rows.Err()
		return names, err
	}
	return names, nil
}

// deletes notes older than `age`
func (ds *Datastore) deleteOldNotes(age time.Duration) error {
	if ds.mode == database {
		_, err := ds.database.Exec(
			`DELETE FROM Notes WHERE strftime("%s", "now") - strftime("%s", LastViewed) > ?`,
			age/time.Second)
		return err
	}
	return nil
}
