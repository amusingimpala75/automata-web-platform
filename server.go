package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/health", health)
	log.Fatal(http.ListenAndServe(":5050", nil))
}
