package supermoto

import (
	"html/template"
	"net/http"
	"path/filepath"
	"time"
)

var funcMap = template.FuncMap{
	"formatDate": func(t time.Time) string {
		return t.Format("2006-01-02")
	},
}

func Serve(w http.ResponseWriter, data any, templatePaths []string) {
	if len(templatePaths) == 0 {
		http.Error(w, "Error parsing templates", http.StatusInternalServerError)
		return
	}

	t := template.New("").Funcs(funcMap)
	t, err := t.ParseFiles(templatePaths...)
	if err != nil {
		http.Error(w, "Error parsing templates", http.StatusInternalServerError)
		return
	}

	templateName := filepath.Base(templatePaths[0])
	if err = t.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, "Error executing templates", http.StatusInternalServerError)
		return
	}
}
