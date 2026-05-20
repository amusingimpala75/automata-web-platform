package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func home(w http.ResponseWriter, r *http.Request, user string) {
	text, err := f.ReadFile("templates/home.gohtml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("login").Parse(string(text))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, map[string]string{
		"user": user,
	})
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("could not load .env files")
	}
	log.Printf("Server is starting")
	if createUserDatabase() != nil {
		log.Fatal("could not ensure users table exists")
	}
	err = addUser("user", "password")
	if err != nil {
		log.Println("[warn] could not add default user: ", err.Error())
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/login", loginRoute)
	mux.HandleFunc("/home", authenticatedPage(home))
	mux.HandleFunc("/logout", logout)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/home", http.StatusFound)
	})
	log.Fatal(http.ListenAndServe(":5050", http.NewCrossOriginProtection().Handler(mux)))
}
