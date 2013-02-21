// Command vanitypkg is an example of using vanity, and is the backend that serves
// the vanity packages from kylelemons.net.
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"kylelemons.net/go/vanitypkg/vanity"
)

var (
	logFile = flag.String("log", "vanitypkg.log", "Log file")

	templateDir = flag.String("templates", "templates", "Directory containing templates (*.tpl.*)")
	httpAddr    = flag.String("http", ":8002", "Address on which to listen for http connections")

	projFile = flag.String("projects", "projects.json", "JSON Project file")
)

var GAID = "UA-1350650-1"

func main() {
	flag.Parse()

	logOut, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("open log: %s", err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, logOut))

	vserv := &vanity.Server{
		Analytics: GAID,
		Reload:    true,
	}
	if err := vserv.LoadTemplates(filepath.Join(*templateDir, "*.tpl.*")); err != nil {
		log.Fatal(err)
	}
	if err := vserv.LoadProjects(*projFile); err != nil {
		log.Fatal(err)
	}

	log.Printf("Listening on %q...", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, vserv))
}
