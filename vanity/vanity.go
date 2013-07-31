// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package vanity provides an http.Handler to serve links for
// vanity packages.
package vanity

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GoDoc is the prefix applied to import paths to redirect to a documentation aggregator.
var GoDoc = "http://godoc.org/"

// A Project is the main element used to render the project display page.
type Project struct {
	// Display
	Name   string            // name of the project
	Desc   string            // short textual project description
	Links  map[string]string // Extra links
	Hidden bool              // not shown to humans, but visible to the go tool

	// Meta Imports
	Import string // exact import path (e.g. "kylelemons.net/go/gofr")
	VCS    string // version control system (e.g. "git", "hg", "svn", "bzr")
	Repo   string // repository checkout URI (e.g. "git://kylelemons.net/go/gofr.git")
	Source string // Source template for "go-source" meta tag
}

// A Server serves vanity package pages.
//
// The template has the following available:
//    {{.Projects}}     - The Projects map from the Server
//    {{.RedirectURL}}  - The URL to which the user is being redirected (if any)
//
// The following are also available for analytics:
//    {{.gaID}}       - Analytics property ID (from the Server)
//    {{.gaAction}}   - The action being performed (e.g. "List" or "Documentation")
//    {{.gaArg}}      - The action argument, if any (package name for Documentation)
//
// Any key/val pairs in Extra will be available to the template as well, and will override
// any vanity-defined keys (to maintain forward compatibiity).
type Server struct {
	// Rendering
	Analytics string                 // Google Analytics Property ID (e.g. UA-12345678-1)
	Templates *template.Template     // templates from which "main" will be rendered
	Projects  map[string]Project     // Projects[subpath] = project
	Extra     map[string]interface{} // extra data available in the templates

	// Serving
	Reload       bool   // true if templates and projects should be reloaded when stale
	TemplateGlob string // if nonzero and Reload is true, template glob to load
	ProjectGlob  string // if nonzero and Reload is true, project file to load

	// Internal
	templatesModified map[string]time.Time
	projectsModified  map[string]time.Time
}

// LoadTemplates loads templates into the server's Templates field from the
// given glob and, if successful, stores the glob in the TemplateGlob field for
// use with Reload.
func (s *Server) LoadTemplates(glob string) error {
	files, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("vanity: LoadTemplates(%q).glob: %s", glob, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("vanity: LoadTemplate(%q).glob: no files found", glob, err)
	}

	tpl := template.New("")
	mtimes := map[string]time.Time{}
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("vanity: LoadTemplates(%q).stat: %s", file, err)
		}

		if _, err := tpl.ParseFiles(file); err != nil {
			return fmt.Errorf("vanity: LoadTemplates(%q).parse: %s", glob, err)
		}

		mtimes[file] = stat.ModTime()
	}
	s.TemplateGlob = glob
	s.Templates = tpl
	s.templatesModified = mtimes
	return nil
}

// LoadProjects loads json map[string]Project files into the server's Projects
// field from the given glob and, if successful, stores the glob in the
// ProjectGlob field for use with Reload.
func (s *Server) LoadProjects(glob string) error {
	files, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("vanity: LoadProjects(%q).glob: %s", glob, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("vanity: LoadTemplate(%q).glob: no files found", glob, err)
	}

	proj := map[string]Project{}
	mtimes := map[string]time.Time{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("vanity: LoadProjects(%q).open: %s", file, err)
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return fmt.Errorf("vanity: LoadProjects(%q).stat: %s", file, err)
		}

		if err := json.NewDecoder(f).Decode(&proj); err != nil {
			return fmt.Errorf("vanity: LoadProjects(%q).json: %s", file, err)
		}

		mtimes[file] = stat.ModTime()
	}
	s.ProjectGlob = glob
	s.Projects = proj
	s.projectsModified = mtimes
	return nil
}

// Stale returns true if any of the files matching the ProjectGlob or TemplateGlob
// have been modified since they were last Loaded.  If an error is encountered,
// it is considered stale.  Errors and stale files are logged.
func (s *Server) Stale() bool {
	stale := func(glob string, mtime map[string]time.Time) bool {
		files, err := filepath.Glob(glob)
		if err != nil {
			log.Printf("vanity: Stale(%q).glob: %s", glob, err)
			return true
		}
		// If the number of files is different
		if len(files) != len(mtime) {
			log.Printf("vanity: File list is stale")
			return true
		}
		for _, file := range files {
			stat, err := os.Stat(file)
			if err != nil {
				log.Printf("vanity: Stale(%q).stat: %s", file, err)
				return true
			}
			// the zero value in mtime is before any real time
			if stat.ModTime().After(mtime[file]) {
				log.Printf("vanity: File is stale: %q", file)
				return true
			}
		}
		return false
	}
	return stale(s.ProjectGlob, s.projectsModified) || stale(s.TemplateGlob, s.templatesModified)
}

// ServeHTTP serves on a directory (prefixes should be stripped) and
// executes the Server's templates based on the path.  The template is
// always executed, but if the subpath of this handler matches the key
// of a project in the Projects map, a Refresh header is sent to the
// browser to redirect to the documentation for the matched package
// after 3 seconds.  See the Server documentation for the fields which
// are available to the templates.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Reload && s.Stale() {
		if s.TemplateGlob != "" {
			if err := s.LoadTemplates(s.TemplateGlob); err != nil {
				log.Print(err)
			}
		}
		if s.ProjectGlob != "" {
			if err := s.LoadProjects(s.ProjectGlob); err != nil {
				log.Print(err)
			}
		}
	}

	data := map[string]interface{}{
		"Projects": s.Projects,
		"gaID":     s.Analytics,
	}

	if sub := strings.TrimPrefix(r.URL.Path, "/"); sub == "" {
		data["gaAction"] = "List"
	} else if proj, ok := s.Projects[sub]; ok {
		data["gaAction"] = "Documentation"
		data["gaArg"] = sub

		redir := GoDoc + proj.Import
		data["RedirectURL"] = redir
		w.Header().Set("Refresh", "3;url="+redir)
	}

	for k, v := range s.Extra {
		data[k] = v
	}

	if err := s.Templates.ExecuteTemplate(w, "main", data); err != nil {
		log.Printf("execute main: %s", err)
	}
}
