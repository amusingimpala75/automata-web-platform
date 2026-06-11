package main

import (
	"net/http"
)

func registerHomeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/home", authenticatedPage(home))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/home", http.StatusFound)
	})
}

func home(w http.ResponseWriter, r *http.Request, user user) {
	tmpl, err := getTemplate("home")
	if err != nil {
		httpErrorLog(w, "could not fetch home template", err)
		return
	}

	users, err := getUsers()
	if err != nil {
		httpErrorLog(w, "could not fetch users", err)
		return
	}

	assignments, err := getAssignments()
	if err != nil {
		httpErrorLog(w, "could not fetch assignments", err)
		return
	}

	amap := []map[string]any{}
	for _, assignment := range assignments {
		amap = append(amap, assignment.formValues())
	}

	err = tmpl.Execute(w, map[string]any{
		"me":          user,
		"users":       users,
		"assignments": amap,
	})

	if err != nil {
		httpErrorLog(w, "error templating home page", err)
	}
}
