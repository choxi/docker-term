package server

import (
	"context"
	"dre/db"
	"dre/docker"
	"dre/streams"
	"dre/utils"
	"dre/ws"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/satori/go.uuid"
)

// Server is a http server
type Server struct {
	database *db.DB
}

// New returns a new Server with initialized handlers
func New(database *db.DB) Server {
	server := Server{database}

	return server
}

// Start runs the server and listens on the provided port
func (s *Server) Start(staticDir string, port int) error {
	finalHandler := http.HandlerFunc(ptyHandler)

	http.Handle("/v1/pty", dbMiddleware(s.database, ws.Middleware(finalHandler)))
	http.HandleFunc("/v1/containers", dbMiddleware(s.database, authenticateMiddleware(containersHandler)))
	http.HandleFunc("/v1/users", dbMiddleware(s.database, signupHandler))
	http.HandleFunc("/v1/sessions", dbMiddleware(s.database, signinHandler))
	http.Handle("/", http.FileServer(http.Dir(staticDir)))

	portStr := strconv.FormatInt(int64(port), 10)
	addr := "localhost:" + portStr
	fmt.Println("Listening on: " + addr)
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
		database  *db.DB
		image     db.Image
		ctx       context.Context
	)

	ctx = r.Context()
	webSocket = ws.FromContext(ctx)
	database = dbFromContext(ctx)
	params = parseParams(r.URL.Query())
	adapter = containerPool[params.ContainerID]

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
		database  *db.DB
		err       error
		image     db.Image
		container db.Container
		user      db.User
		ctx       context.Context
	)

	ctx = r.Context()
	user = userFromContext(ctx)
	database = dbFromContext(ctx)

	if params, err = parseJSON(r); err != nil {
		fmt.Println(err)
	}

	if image, err = database.CreateImage(user, params.SourceURL); err != nil {
		fmt.Println(err)
	}

	if container, err = database.CreateContainer(&image); err != nil {
		fmt.Println(err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(container)
}

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
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	database = dbFromContext(r.Context())
	if user, err = database.CreateUser(creds.Username, creds.Password); err != nil {
		fmt.Println(err)
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

var dbKey = "DB_KEY"

func dbMiddleware(database *db.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("begin dbMiddleware")
		ctx := context.WithValue(r.Context(), dbKey, database)
		next(w, r.WithContext(ctx))
		log.Println("end dbMiddleware")
	}
}

func dbFromContext(ctx context.Context) *db.DB {
	return ctx.Value(dbKey).(*db.DB)
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
