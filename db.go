package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

func openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./database.sqlite3")
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}
