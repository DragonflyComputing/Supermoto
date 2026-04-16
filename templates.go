package supermoto

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

/*
Supermoto Templates

Provides a simple wrapper around Go's html/template package for serving HTML pages.

Features:
- Render multiple templates in a single call (primary template + partials)
- Built-in template function map for common formatting tasks
- Errors logged to stderr and returned as 500 responses

Template functions available in .html files:
- formatDate: formats a time.Time as YYYY-MM-DD
*/

// funcMap contains functions available to all templates.
var funcMap = template.FuncMap{
	"formatDate": func(t time.Time) string {
		return t.Format("2006-01-02")
	},
}

// Serve parses and executes a set of templates, writing the result to the response.
// The first path in templatePaths is the entry point template. Additional paths
// (e.g. partials) are parsed into the same set so {{template "name" .}} calls resolve.
// Errors are logged and written as 500 responses.
// Pass nil for logger to use the default standard library logger.
func Serve(w http.ResponseWriter, data any, templatePaths []string, logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}

	if len(templatePaths) == 0 {
		http.Error(w, "Error parsing templates", http.StatusInternalServerError)
		logger.Println("error parsing templates: templatePaths cannot be empty")
		return
	}

	t := template.New("").Funcs(funcMap)
	t, err := t.ParseFiles(templatePaths...)
	if err != nil {
		http.Error(w, "Error parsing templates", http.StatusInternalServerError)
		logger.Printf("error parsing templates: %v", err)
		return
	}

	// Execute by filename so the entry point is always the first template provided
	templateName := filepath.Base(templatePaths[0])
	if err = t.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, "Error executing templates", http.StatusInternalServerError)
		logger.Printf("error executing template: %v", err)
		return
	}
}
