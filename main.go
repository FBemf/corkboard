package main

import (
	"bytes"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path"

	"github.com/julienschmidt/httprouter"
)

type IndexData struct {
}

type NoteData struct {
	Title string
	Body  string
}

type Config struct {
	templatePath string
	notePath     string
	staticPath   string
	credentials  string
}

func main() {
	config := parseArgs()
	templates := makeTemplates(config.templatePath)

	router := httprouter.New()
	router.GET("/", Auth(Index(templates), config.credentials))
	router.GET("/note/:note", Auth(Note(templates, config.notePath), config.credentials))
	router.POST("/note/:note", Auth(PostNote(config.notePath), config.credentials))
	router.GET("/raw/:note", Auth(RawNote(config.notePath), config.credentials))
	router.ServeFiles("/static/*filepath", http.Dir(config.staticPath))
	log.Print("Running")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func parseArgs() Config {
	config := Config{}
	flag.StringVar(&config.notePath, "note-path", "./notes/", "Path to the directory where the notes are stored")
	flag.StringVar(&config.templatePath, "template-path", "./templates/", "Path to the directory where html templates are stored")
	flag.StringVar(&config.staticPath, "static-path", "./static/", "Path to the directory where static assets are stored")
	credentialFile := flag.String("creds-file", "", "Path to a file holding login credentials in the form \"username:password\"")
	flag.StringVar(&config.credentials, "creds", "", "Access credentials in the form \"username:password\". Overrides -creds-file")
	flag.Parse()

	if config.notePath == "" {
		log.Fatal("bad arguments: -note-path must be given")
	}
	if config.templatePath == "" {
		log.Fatal("bad arguments: -template-path must be given")
	}
	if config.staticPath == "" {
		log.Fatal("bad arguments: -static-path must be given")
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

func makeTemplates(templatePath string) template.Template {
	t, err := template.ParseGlob(path.Join(templatePath, "*"))
	if err != nil {
		log.Fatal(err)
	}
	return *t
}

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

func Index(templates template.Template) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
		err := templates.ExecuteTemplate(resp, "index.html", IndexData{})
		if err != nil {
			log.Printf("rendering page: %v", err)
		}
	}
}

func Note(templates template.Template, notePath string) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		notePath := path.Join(notePath, params[0].Value)
		data, err := ioutil.ReadFile(notePath)
		if err != nil {
			log.Printf("reading %s: %v", notePath, err)
			return
		}
		resp.Header().Set("Content-Type", "text/html; charset=UTF-8")
		err = templates.ExecuteTemplate(resp, "note.html", NoteData{Title: params[0].Value, Body: string(data)})
		if err != nil {
			log.Printf("writing template: %v", err)
		}
	}
}

func RawNote(notePath string) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		notePath := path.Join(notePath, params[0].Value)
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
}

func PostNote(notePath string) httprouter.Handle {
	return func(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
		body := bytes.NewBuffer(nil)
		_, err := body.ReadFrom(req.Body)
		if err != nil {
			log.Printf("error reading request body: %v", err)
			return
		}
		notePath := path.Join(notePath, params[0].Value)
		err = ioutil.WriteFile(notePath, body.Bytes(), 0755)
		if err != nil {
			log.Printf("error writing to file %s: %v", notePath, err)
			return
		}
		log.Printf("New note %s", notePath)
	}
}
