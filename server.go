package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("could not load .env files")
	}

	logger.Info("Server is starting")
	if createUserDatabase() != nil {
		log.Fatal("could not ensure users table exists")
	}

	err = addUser("administrator", "administrator")
	if err != nil {
		logger.Warn(fmt.Sprint("could not add default user: ", err.Error()))
	}

	mux := http.NewServeMux()

	registerAuthRoutes(mux)
	registerHomeRoutes(mux)
	registerHealthRoute(mux)
	registerUserRoutes(mux)

	log.Fatal(http.ListenAndServe(":5050", http.NewCrossOriginProtection().Handler(mux)))
}
