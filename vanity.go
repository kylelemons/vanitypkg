package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var (
	logFile = flag.String("log", "vanitypkg.log", "Log file")

	templateDir = flag.String("templates", ".", "Directory containing templates")
	httpAddr    = flag.String("http", ":8002", "Address on which to listen for http connections")
)

func loadTemplates() (*template.Template, error) {
	return template.ParseGlob(filepath.Join(*templateDir, "*.tpl.*"))
}

type Project struct {
	// Display
	Name  string            // name of the project
	Desc  string            // short textual project description
	Links map[string]string // Extra links

	// Meta Import
	Import string // exact import path (e.g. "kylelemons.net/go/gofr")
	VCS    string // version control system (e.g. "git", "hg", "svn", "bzr")
	Repo   string // repository checkout URI (e.g. "git://kylelemons.net/go/gofr.git")

}

type AnalyticsInfo struct {
	GAID string
	Host string
}

var GAID = "UA-30511466-1"

var Projects = map[string]Project{
	"rx": {
		Name: "rx",
		Desc: "Package version and dependency manager",
		Links: map[string]string{
			"GitHub":       "https://github.com/kylelemons/rx",
			"Report Issue": "https://github.com/kylelemons/rx/issues",
			"Blog Post":    "http://kylelemons.net/blog/2012/04/22-rx-for-go-headaches.article",
		},

		Import: "kylelemons.net/go/rx",
		VCS:    "git",
		Repo:   "git://github.com/kylelemons/rx.git",
	},
	/*
	   'rpcgen' => array(
	     'name' => 'RPC Generator',
	     'vcs'  => 'git',
	     'repo' => 'git://github.com/kylelemons/go-rpcgen.git',
	     'real' => 'https://github.com/kylelemons/go-rpcgen',
	     'desc' => 'RPC stub generator for net/rpc (supports gob, JSON, and protobuf)',
	   ),
	*/
	/*
	   'blightbot' => array(
	     'name' => 'BlightBot IRC Bot Framework',
	     'vcs'  => 'git',
	     'repo' => 'git://github.com/kylelemons/blightbot.git',
	     'real' => 'https://github.com/kylelemons/blightbot',
	     'desc' => 'Extensible IRC Bot Framework (powers the GoDoc bot on #go-nuts)',
	   ),
	*/
}

func root(w http.ResponseWriter, r *http.Request) {
	tpl, err := loadTemplates()
	if err != nil {
		log.Printf("load templates: %s", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Projects": Projects,
		//"gaID":     GAID,
		//"gaAction": "List",
	}

	if err := tpl.ExecuteTemplate(w, "main", data); err != nil {
		log.Printf("execute main: %s", err)
	}
}

func main() {
	flag.Parse()

	logOut, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("open log: %s", err)
	}
	log.SetOutput(logOut)

	//strip non [-_A-Za-z0-9/]
	//check get param 'go-get'
	//Refresh: 1;url=refreshURI
	log.Printf("Listening on %q...", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, http.HandlerFunc(root)))
}
