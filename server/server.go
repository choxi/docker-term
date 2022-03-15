package server

import (
	"context"
	"dre/db"
	"dre/docker"
	"dre/streams"
	"dre/utils"
	"dre/ws"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	uuid "github.com/satori/go.uuid"
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
	var (
		portStr string
		addr    string
		err     error
	)

	http.Handle("/v1/pty", dbMiddleware(s.database, ws.Middleware(ptyHandler)))
	http.Handle("/", http.FileServer(http.Dir(staticDir)))

	portStr = strconv.FormatInt(int64(port), 10)
	addr = "localhost:" + portStr
	fmt.Println("Listening on: " + addr)

	if err = http.ListenAndServe(addr, nil); err != nil {
		return fmt.Errorf("net.http could not listen on address '%s': %s", addr, err)
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
		ctx       context.Context
	)

	ctx = r.Context()
	webSocket = ws.FromContext(ctx)
	params = parseParams(r.URL.Query())

	log.Println(params)
	if params.SourceURL == "" {
		// containerID, _ := uuid.FromString(params.ContainerID)
		// container := docker.Container{ID: containerID}
		adapter := containerPool[params.ContainerID]
		adapter.AddStream(&webSocket)
		// if pty, err = container.Connect("/bin/bash"); err != nil {
		// 	fmt.Println(err)
		// 	http.Error(w, "Could not connect to container", http.StatusInternalServerError)
		// }
		// newAdapter := streams.NewAdapter(&pty, &webSocket)
		// newAdapter.OnDisconnect = func() error {
		// 	var err error

		// 	if err = pty.Stop(); err != nil {
		// 		return err
		// 	}

		// 	// Need to wait to see if others are still connected
		// 	if err = dctr.Stop(); err != nil {
		// 		return err
		// 	}

		// 	return nil
		// }
		log.Println("Connecting to ContainerID: " + params.ContainerID)

		// go func() {
		// 	newAdapter.Connect()
			// containerPool[dctr.ID.String()] = nil
		// }()

		return
	}

	if dctr, err = docker.CreateContainer(uuid.NewV4(), params.SourceURL); err != nil {
		log.Println("Container could not be built")
		log.Println(err)
		return
	}

	log.Println("Starting container...")

	if pty, err = dctr.Bash(); err != nil {
		fmt.Println(err)
		http.Error(w, "Container could not be started", http.StatusInternalServerError)
		return
	}

	newAdapter := streams.NewAdapter(&pty, &webSocket)
	containerPool[dctr.ID.String()] = &newAdapter
	newAdapter.OnDisconnect = func() error {
		var err error

		if err = pty.Stop(); err != nil {
			return err
		}

		// Need to wait to see if others are still connected
		if err = dctr.Stop(); err != nil {
			return err
		}

		return nil
	}

	fmt.Println("Connecting to ContainerID: " + dctr.ID.String())

	go func() {
		newAdapter.Connect()
		// containerPool[dctr.ID.String()] = nil
	}()

	fmt.Println("Done")
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
