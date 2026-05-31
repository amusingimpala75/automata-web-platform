package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/crypto/argon2"
)

type user struct {
	salt     [32]byte
	Username string
	hash     [32]byte
	Admin    bool
}

func registerUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/user", adminPage(func(w http.ResponseWriter, r *http.Request, u user) {
		switch r.Method {
		case "POST":
			err := addUser(r.FormValue("username"), r.FormValue("password"))
			if err != nil {
				httpErrorLog(w, "could not add user", err)
			} else {
				http.Redirect(w, r, "/home", http.StatusFound)
			}
		}
	}))
	mux.HandleFunc("/user/{id}", adminPage(func(w http.ResponseWriter, r *http.Request, u user) {
		switch r.Method {
		case "POST":
			err := deleteUser(r.PathValue("id"))
			if err != nil {
				httpErrorLog(w, "could not remove user", err)
			} else {
				http.Redirect(w, r, "/home", http.StatusFound)
			}
		}
	}))
	mux.HandleFunc("/password", authenticatedPage(updatePassword))
}

func createUserDatabase() error {
	db, err := openDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"CREATE TABLE IF NOT EXISTS users (id INT PRIMARY KEY, username TEXT UNIQUE NOT NULL, salt BLOB NOT NULL, hash BLOB NOT NULL, admin BOOL)",
	)
	return err
}

func updatePassword(w http.ResponseWriter, r *http.Request, user user) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	newPassword := r.FormValue("password")

	pepper, err := getPepper()
	if err != nil {
		httpErrorLog(w, "could not fetch pepper", err)
		return
	}

	db, err := openDB()
	if err != nil {
		httpErrorLog(w, "could not open database", err)
		return
	}

	err = user.fetchSalt()
	if err != nil {
		httpErrorLog(w, "could not fetch user's salt", err)
		return
	}

	hash, err := hash(newPassword, user.salt, *pepper)
	if err != nil {
		httpErrorLog(w, "could not hash new password", err)
		return
	}

	affected, err := db.Exec(
		"UPDATE users SET hash = $1 WHERE username = $2",
		hash[:],
		user.Username,
	)
	if err != nil {
		httpErrorLog(w, "could not update database with new password", err)
		return
	}

	rows, err := affected.RowsAffected()
	if err != nil || rows != 1 {
		httpErrorLog(w, "should have modified exactly 1 row in database", err)
		return
	}

	http.Redirect(w, r, "/home", http.StatusFound)
}

func addUser(username string, password string) error {
	logger.Info(fmt.Sprintln("adding user", username))

	var salt [32]byte
	n, err := rand.Read(salt[:])
	if err != nil || n != len(salt) {
		return errors.New("Could not generate salt for new user")
	}

	pepper, err := getPepper()
	if err != nil {
		return err
	}

	hash, err := hash(password, salt, *pepper)
	if err != nil {
		return err
	}

	user := user{
		salt:     salt,
		Username: username,
		hash:     *hash,
		Admin:    false,
	}

	db, err := openDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"INSERT INTO users (hash, salt, username, admin) values ($1, $2, $3, $4)",
		user.hash[:],
		user.salt[:],
		user.Username,
		user.Admin,
	)

	return err
}

func deleteUser(username string) error {
	db, err := openDB()
	if err != nil {
		return err
	}

	res, err := db.Exec("DELETE FROM users WHERE username = $1", username)
	if err != nil {
		return err
	} else if i, err := res.RowsAffected(); err != nil || i != 1 {
		return err
	}

	return nil
}

func getUsers() ([]user, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT username, admin from USERS")
	if err != nil {
		return nil, err
	}

	var users []user

	for rows.Next() {
		var u user
		err := rows.Scan(&u.Username, &u.Admin)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

func hash(password string, salt [32]byte, pepper [32]byte) (*[32]byte, error) {
	input := append([]byte(password), pepper[:]...)
	key := argon2.IDKey(input, salt[:], 3, 32*1024, 4, 32)

	var ret [32]byte
	copy(ret[:], key[0:32])

	return &ret, nil
}

func (u user) validate(password string) bool {
	pepper, err := getPepper()
	if err != nil {
		return false
	}

	guessPtr, err := hash(password, u.salt, *pepper)
	guess := *guessPtr

	return err == nil && subtle.ConstantTimeCompare(u.hash[:], guess[:]) == 1
}

func (u *user) fetchSalt() error {
	db, err := openDB()
	if err != nil {
		return err
	}

	rows, err := db.Query("SELECT salt FROM users WHERE username = $1", u.Username)
	if err != nil {
		return err
	} else if !rows.Next() {
		return errors.New("cannot find row")
	}

	var buf []byte

	err = rows.Scan(&buf)
	if err != nil {
		return err
	}

	copy(u.salt[:], buf[0:32])

	return nil
}

func getPepper() (*[32]byte, error) {
	var pepper [32]byte
	buf, err := base64.StdEncoding.DecodeString(os.Getenv("AUTOMATA_WEB_PLATFORM_PEPPER"))
	if err != nil {
		return nil, err
	} else if len(buf) != 32 {
		return nil, errors.New("pepper was not 32 bytes long")
	}
	copy(pepper[:], buf[0:32])

	return &pepper, nil
}

func getJwtKey() (*[32]byte, error) {
	b64 := os.Getenv("AUTOMATA_WEB_PLATFORM_KEY")
	buf, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	} else if len(buf) != 32 {
		return nil, errors.New("key was not 32 bytes long")
	}
	var key [32]byte
	copy(key[:], buf[0:32])
	return &key, nil
}
