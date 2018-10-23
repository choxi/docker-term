package server

import (
	"Dre/db"
	"Dre/docker"
	"Dre/streams"
	"Dre/utils"
	"Dre/ws"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

var secret = []byte("PANCAKES")

// Server is a http server
type Server struct {
}

// New returns a new Server with initialized handlers
func New() Server {
	server := Server{}

	return server
}

// Start runs the server and listens on the provided port
func (s *Server) Start(staticDir string, port string) error {
	finalHandler := http.HandlerFunc(ptyHandler)

	http.Handle("/v1/pty", ws.Middleware(finalHandler))
	http.HandleFunc("/v1/containers", authenticateMiddleware(containersHandler))
	http.HandleFunc("/v1/users", signupHandler)
	http.HandleFunc("/v1/sessions", signinHandler)
	http.Handle("/", http.FileServer(http.Dir(staticDir)))

	addr := "localhost:" + port
	err := http.ListenAndServe(addr, nil)

	if err != nil {
		message := fmt.Sprintf("net.http could not listen on address '%s': %s\n", addr, err)
		return errors.New(message)
	}

	return nil
}

type parameters struct {
	SourceURL   string `json:"source_url"`
	ContainerID string `json:"container_id"`
}

var containerPool = make(map[string]*streams.Adapter)

func ptyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		pty       docker.Pty
		dctr      docker.Container
		webSocket ws.WS
		params    parameters
		adapter   *streams.Adapter
		ctr       db.Container
		database  db.DB
		image     db.Image
	)

	webSocket = ws.FromContext(r.Context())
	params = parseParams(r.URL.Query())
	adapter = containerPool[params.ContainerID]
	database = db.Connect()

	if params.ContainerID == "" {
		http.Error(w, "No container_id", http.StatusBadRequest)
		return
	}

	if params.ContainerID != "" && adapter != nil {
		log.Println("Connecting to ContainerID: " + params.ContainerID)
		adapter.AddStream(&webSocket)
		return
	}

	if ctr, err = database.FindContainer(params.ContainerID); err != nil {
		log.Println(err)
		http.Error(w, "Container not found", http.StatusBadRequest)
		return
	}

	if image, err = database.FindImage(ctr.ImageID); err != nil {
		log.Println(err)
		http.Error(w, "Image not found", http.StatusBadRequest)
		return
	}

	uid, _ := uuid.FromString(ctr.UUID)
	if dctr, err = docker.CreateContainer(uid, image.SourceURL); err != nil {
		log.Println(err)
		http.Error(w, "Container could not be built", http.StatusInternalServerError)
		return
	}

	defer dctr.Stop()

	if pty, err = dctr.Bash(); err != nil {
		log.Println(err)
		http.Error(w, "Container could not be started", http.StatusInternalServerError)
		return
	}
	defer pty.Stop()

	newAdapter := streams.NewAdapter(&pty, &webSocket)
	containerPool[dctr.ID.String()] = &newAdapter

	fmt.Println("Connecting to ContainerID: " + dctr.ID.String())

	err = newAdapter.Connect()
}

func containersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var (
		params    parameters
		database  db.DB
		err       error
		image     db.Image
		container db.Container
		user      db.User
	)

	user = userFromContext(r.Context())

	if params, err = parseJSON(r); err != nil {
		fmt.Println(err)
	}

	database = db.Connect()

	if image, err = database.CreateImage(user, params.SourceURL); err != nil {
		fmt.Println(err)
	}

	if container, err = database.CreateContainer(&image); err != nil {
		fmt.Println(err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(container)
}

type Credentials struct {
	Password string `json:"password", db:"password"`
	Username string `json:"username", db:"username"`
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	creds := &Credentials{}
	err := json.NewDecoder(r.Body).Decode(creds)
	database := db.Connect()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), 8)

	if _, err = database.Connection().Query("INSERT INTO users (username, password) VALUES ($1, $2)", creds.Username, string(hashedPassword)); err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func signinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	creds := &Credentials{}
	err := json.NewDecoder(r.Body).Decode(creds)
	database := db.Connect()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	result := database.Connection().QueryRow("SELECT password FROM users WHERE username=$1", creds.Username)
	storedCreds := &Credentials{}
	err = result.Scan(&storedCreds.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(storedCreds.Password), []byte(creds.Password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": creds.Username,
	})

	tokenString, err := token.SignedString(secret)

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
	})
}

const userKey = "USER_KEY"

func authenticateMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("start authenticateMiddleware")
		authorization := r.Header.Get("Authorization")
		database := db.Connect()

		if authorization == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		tokenString := strings.Split(authorization, "Bearer ")[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			return secret, nil
		})

		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			var username string
			var user db.User
			if username, ok = claims["username"].(string); !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if user, err = database.FindUser(username); err != nil {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), userKey, user)
			next(w, r.WithContext(ctx))
		} else {
			fmt.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
		}

		log.Println("end authenticateMiddleware")
	}
}

func userFromContext(ctx context.Context) db.User {
	return ctx.Value(userKey).(db.User)
}

func parseJSON(r *http.Request) (parameters, error) {
	var (
		params parameters
		err    error
	)

	decoder := json.NewDecoder(r.Body)
	if err = decoder.Decode(&params); err != nil {
		return params, err
	}

	return params, nil
}

func parseParams(values url.Values) parameters {
	var params parameters

	sourceURLKey := "source_url"
	containerIDKey := "container_id"

	if len(values[sourceURLKey]) > 0 {
		params.SourceURL = utils.Decode64(values[sourceURLKey][0])
	}

	if len(values[containerIDKey]) > 0 {
		params.ContainerID = values["container_id"][0]
	}

	return params
}
