package main

import (
	"bufio"
	"database/sql"
	"embed"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed schema
var schemaFS embed.FS

// delete expired notes every hour
const cleanupInterval = time.Hour

// Config stores data derived from the command line arguments
type Config struct {
	migrate        bool
	databasePath   string
	credentials    map[string]bool
	port           int
	noteExpiryTime time.Duration
	numRecentNotes int
	storageMode    int // database or flatFile
}

func main() {
	config := parseArgs()

	templates, err := template.ParseFS(templateFS, "templates/*")
	if err != nil {
		log.Fatal(err)
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	migrations, err := fs.Sub(schemaFS, "schema")
	if err != nil {
		log.Fatal(err)
	}

	var datastore Datastore
	// set up sqlite database
	db, err := sql.Open("sqlite3", config.databasePath)
	if err != nil {
		log.Fatalf("error opening db %s", config.databasePath)
	}
	datastore = Datastore{db}
	defer datastore.Close()

	if config.migrate {
		err = datastore.RunMigrations(migrations)
		if err != nil {
			log.Fatalf("error running schema: %s\n", err)
		}
		datastore.Close()
		return
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

	router := makeRouter(templates, static, config, datastore)
	log.Print("Running")
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(config.port), router))
}

// parses command line arguments
// all configs are passed in through here
func parseArgs() Config {
	config := Config{}
	flag.StringVar(&config.databasePath, "db-path", "./notes.db", "Path to the sqlite db.")
	credentialFile := flag.String("creds-file", "", "Path to a file holding login credentials in the form\n\"username:password\". Each line holds a valid set of credentials.")
	credentials := flag.String("creds", "", "Access credentials in the form\n\"username:password\".")
	flag.IntVar(&config.port, "port", 8080, "Port to serve the application on.")
	noteExpiryTime := flag.Int("note-expiry", 7, "Notes which have not been viewed in this many days will be deleted.\nIf set to zero, notes never expire.")
	flag.IntVar(&config.numRecentNotes, "recent-notes", 8, "Display this many recent notes on the main page.\n")
	flag.BoolVar(&config.migrate, "migrate", false, "Run schema and all migrations upon database, then exit.")
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

	// config.credentials is a map of all valid user:password strings
	if *credentialFile == "" && *credentials == "" {
		// if config.credentials is nil, authentication is turned off
		config.credentials = nil
	} else {
		config.credentials = make(map[string]bool)
		if *credentialFile != "" {
			// if a file was provided, each line is a valid set of creds
			file, err := os.Open(*credentialFile)
			if err != nil {
				log.Fatalf("bad arguments: unable to open credentials file %s: %v", *credentialFile, err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if scanner.Text() != "" {
					config.credentials[scanner.Text()] = true
				}
			}

			if err := scanner.Err(); err != nil {
				log.Fatalf("bad arguments: unable to read credentials file %s: %v", *credentialFile, err)
			}
		}
		if *credentials != "" {
			config.credentials[*credentials] = true
		}
	}

	return config
}
