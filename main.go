package main

import (
	"database/sql"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

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
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(config.port), router))
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
