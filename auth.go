package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
)

func registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			if r.Form.Has("delete") {
				logout(w, r)
			} else {
				login(w, r)
			}
		}
	})

	mux.HandleFunc("/login", loginPage)
}

func goToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusFound)
}

func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	goToLogin(w, r)
}

func login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" {
		goToLogin(w, r)
		return
	}

	db, err := openDB()
	if err != nil {
		httpErrorLog(w, "could not open database", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(
		"SELECT salt, hash, admin FROM users WHERE username = $1",
		username,
	)
	if err != nil || !rows.Next() {
		logger.Info(fmt.Sprint(username, " failed authentication"))
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
		httpErrorLog(w, "could not fetch user's information from database", err)
		return
	}
	copy(user.hash[:], hash[0:32])
	copy(user.salt[:], salt[0:32])

	if !user.validate(password) {
		logger.Info(fmt.Sprint(username, " failed authentication"))
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
		httpErrorLog(w, "could not fetch jwt signing key", err)
		return
	}

	tokString, err := token.SignedString(key[:])
	if err != nil {
		httpErrorLog(w, "could not encode jwt", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokString,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(oneDay),
		Path:     "/",
	})
	http.Redirect(w, r, "/home", http.StatusFound)
}

func loginPage(w http.ResponseWriter, _ *http.Request) {
	tmpl, err := getTemplate("login")
	if err != nil {
		httpErrorLog(w, "could not get template", err)
		return
	}
	err = tmpl.Execute(w, map[string]string{})
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
