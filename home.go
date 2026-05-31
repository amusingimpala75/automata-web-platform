package main

import (
	"html/template"
	"net/http"
)

func registerHomeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/home", authenticatedPage(home))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/home", http.StatusFound)
	})
}

func home(w http.ResponseWriter, r *http.Request, user user) {
	text, err := f.ReadFile("templates/home.gohtml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("home").Parse(string(text))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	users, err := getUsers()
	if err != nil {
		httpErrorLog(w, "could not fetch users", err)
	}

	err = tmpl.Execute(w, map[string]any{
		"me":    user,
		"users": users,
	})

	if err != nil {
		httpErrorLog(w, "error templating home page", err)
	}
}
