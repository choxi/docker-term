package server

import (
	"context"
	"dre/db"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func signupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var (
		creds    = &db.Credentials{}
		err      error
		database *db.DB
		user     db.User
	)

	if err = json.NewDecoder(r.Body).Decode(creds); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	database = dbFromContext(r.Context())
	if user, err = database.CreateUser(creds.Username, creds.Password); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func signinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var (
		creds    = &db.Credentials{}
		err      error
		database *db.DB
		user     db.User
		token    string
	)

	if err = json.NewDecoder(r.Body).Decode(creds); err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	database = dbFromContext(r.Context())
	user, err = database.SignInUser(creds.Username, creds.Password)
	token, err = db.CreateToken(&user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

const userKey = "USER_KEY"

func authenticateMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("start authenticateMiddleware")

		var (
			user          db.User
			authorization string
			token         string
			err           error
			database      *db.DB
		)

		if authorization = r.Header.Get("Authorization"); authorization == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		database = dbFromContext(r.Context())
		token = strings.Split(authorization, "Bearer ")[1]

		if user, err = database.AuthenticateToken(token); err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(ctx))

		log.Println("end authenticateMiddleware")
	}
}

func userFromContext(ctx context.Context) db.User {
	return ctx.Value(userKey).(db.User)
}
