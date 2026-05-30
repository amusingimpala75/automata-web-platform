package main

import (
	"fmt"
	"log/slog"
	"net/http"
)

var logger *slog.Logger = slog.Default()

func httpErrorLog(w http.ResponseWriter, msg string, err error) {
	http.Error(w, msg, http.StatusInternalServerError)

	if err != nil {
		msg = fmt.Sprintln(msg, ": ", err.Error())
	}

	logger.Error(msg)
}
