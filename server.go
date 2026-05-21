package main

import (
	"html/template"
	"log"
	"log/slog"
	"net/http"

	"github.com/joho/godotenv"
)

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
	err = tmpl.Execute(w, user)
}

var logger *slog.Logger

func main() {
	logger = slog.Default()
	err := godotenv.Load()
	if err != nil {
		log.Fatal("could not load .env files")
	}
	log.Printf("Server is starting")
	if createUserDatabase() != nil {
		log.Fatal("could not ensure users table exists")
	}
	err = addUser("admin", "admin")
	if err != nil {
		log.Println("[warn] could not add default user: ", err.Error())
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/login", loginRoute)
	mux.HandleFunc("/home", authenticatedPage(home))
	mux.HandleFunc("/logout", logout)
	mux.HandleFunc("/user", adminPage(addUserRoute))
	mux.HandleFunc("/password", authenticatedPage(updatePassword))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/home", http.StatusFound)
	})
	log.Fatal(http.ListenAndServe(":5050", http.NewCrossOriginProtection().Handler(mux)))
}
