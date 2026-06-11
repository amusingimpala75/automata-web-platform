package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"text/template"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		_, err1 := os.Stat(".env")
		_, err2 := os.Stat("database.sqlite3")
		if !(errors.Is(err1, os.ErrNotExist) && errors.Is(err2, os.ErrNotExist)) {
			log.Fatal("error reading .env file: ", err.Error())
		}
		logger.Info("Could not find .env file or database; assuming first run and generating")
		firstTimeSetup()
	}

	logger.Info("Server is starting")

	mux := http.NewServeMux()

	registerAuthRoutes(mux)
	registerHomeRoutes(mux)
	registerHealthRoute(mux)
	registerUserRoutes(mux)
	registerAssignmentRoutes(mux)
	registerStaticRoute(mux)

	log.Fatal(http.ListenAndServe(":5050", http.NewCrossOriginProtection().Handler(mux)))
}

func firstTimeSetup() {
	var (
		key    [32]byte
		pepper [32]byte
	)

	if n, err := rand.Read(key[:]); err != nil || n != len(key) {
		log.Fatal("could not create platform key: ", err.Error())
	}

	if n, err := rand.Read(pepper[:]); err != nil || n != len(pepper) {
		log.Fatal("could not create database pepper: ", err.Error())
	}

	key_b64 := base64.StdEncoding.EncodeToString(key[:])
	pepper_b64 := base64.StdEncoding.EncodeToString(pepper[:])

	tmpl, err := template.New("generated-.env").Parse(`
AUTOMATA_WEB_PLATFORM_KEY={{ .key }}
AUTOMATA_WEB_PLATFORM_PEPPER={{ .pepper }}
`)
	if err != nil {
		log.Fatal("could not create .env template: ", err.Error())
	}

	f, err := os.OpenFile(".env", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("could not open .env file for writing: ", err.Error())
	}
	defer f.Close()

	if err := tmpl.Execute(f, map[string]string{"key": key_b64, "pepper": pepper_b64}); err != nil {
		log.Fatal("could not generate .env file: ", err.Error())
	}

	err = godotenv.Load()
	if err != nil {
		log.Fatal("error reading .env file: ", err.Error())
	}

	if err := createUserTable(); err != nil {
		log.Fatal("could not create users table: ", err.Error())
	}

	if err := addUser("administrator", "administrator"); err != nil {
		log.Fatal("could not add default user: ", err.Error())
	}
	if err := makeAdmin("administrator"); err != nil {
		log.Fatal("could not promote the default user to admin: ", err.Error())
	}

	if err := createAssignmentTable(); err != nil {
		log.Fatal("could not create assignment table: ", err.Error())
	}

	logger.Info("Created default administrator account with password administrator")
	logger.Info("PLEASE CHANGE THIS VALUE IMMEDIATELY")
}
