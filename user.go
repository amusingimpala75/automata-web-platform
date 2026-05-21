package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/argon2"
)

type user struct {
	salt     [32]byte
	Username string
	hash     [32]byte
	Admin    bool
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

func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	goToLogin(w, r)
}

func goToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusFound)

}

func loginRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		login(w, r)
	} else {
		loginPage(w, r)
	}
}

func loginPage(w http.ResponseWriter, _ *http.Request) {
	text, err := f.ReadFile("templates/login.gohtml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("login").Parse(string(text))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, map[string]string{})
}

func login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	db, err := openDB()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(
		"SELECT salt, hash, admin FROM users WHERE username = $1",
		username,
	)
	if err != nil || !rows.Next() {
		goToLogin(w, r)
		return
	}

	user := user{Username: username}
	var (
		hash []byte
		salt []byte
	)
	err = rows.Scan(&salt, &hash, &user.Admin)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	copy(user.hash[:], hash[0:32])
	copy(user.salt[:], salt[0:32])

	if !user.validate(password) {
		goToLogin(w, r)
		return
	}

	now := time.Now()
	oneDay, _ := time.ParseDuration("24h")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":  username,
		"admin": user.Admin,
		"iat":   now.Unix(),
		"exp":   now.Add(oneDay).Unix(),
	})

	key, err := getJwtKey()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tokString, err := token.SignedString(key[:])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokString,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(oneDay),
		Path:     "/",
	})
	http.Redirect(w, r, "/home", http.StatusFound)
}

func authenticatedPage(route func(http.ResponseWriter, *http.Request, user)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("jwt")
		if err != nil {
			logout(w, r)
			return
		}

		t, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("bad signing method")
			}
			key, err := getJwtKey()
			if err != nil {
				return nil, err
			}
			return key[:], nil
		})
		if err != nil || !t.Valid {
			logout(w, r)
			return
		}

		if claims, ok := t.Claims.(jwt.MapClaims); !ok {
			logout(w, r)
			return
		} else if username, ok := claims["user"]; !ok {
			logout(w, r)
			return

		} else if admin, ok := claims["admin"]; !ok {
			logout(w, r)
			return
		} else {
			route(w, r, user{
				Username: username.(string),
				Admin:    admin.(bool),
			})
		}
	}
}

func adminPage(route func(http.ResponseWriter, *http.Request, user)) func(http.ResponseWriter, *http.Request) {
	return authenticatedPage(func(w http.ResponseWriter, r *http.Request, user user) {
		if !user.Admin {
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}
		route(w, r, user)
	})
}

func updatePassword(w http.ResponseWriter, r *http.Request, user user) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	newPassword := r.FormValue("password")

	pepper, err := getPepper()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("could not fetch pepper")
		return
	}

	db, err := openDB()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("could not open database")
		return
	}

	err = user.fetchSalt()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("could not fetch user's salt")
		return
	}

	hash, err := hash(newPassword, user.salt, *pepper)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("could not hash new password")
		return
	}

	affected, err := db.Exec(
		"UPDATE users SET hash = $1 WHERE username = $2",
		hash[:],
		user.Username,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("could not update database with new password: ", err.Error(), "")
		return
	}

	rows, err := affected.RowsAffected()
	if err != nil || rows != 1 {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("Should have affected 1 row in password update")
		return
	}

	http.Redirect(w, r, "/home", http.StatusFound)
}

func addUserRoute(w http.ResponseWriter, r *http.Request, _ user) {
	if r.Method == "POST" {
		err := addUser(r.FormValue("username"), r.FormValue("password"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			http.Redirect(w, r, "/home", http.StatusFound)
		}
	}
}

func addUser(username string, password string) error {
	log.Printf("adding user %s\n", username)

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
