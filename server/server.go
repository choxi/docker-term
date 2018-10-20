package server

import (
	"Dre/db"
	"Dre/docker"
	"Dre/streams"
	"Dre/utils"
	"Dre/ws"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

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
	http.HandleFunc("/v1/containers", containersHandler)
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
		container docker.Container
		webSocket ws.WS
		params    parameters
		adapter   *streams.Adapter
	)

	webSocket = ws.FromContext(r.Context())
	params = parseParams(r.URL.Query())
	adapter = containerPool[params.ContainerID]

	if params.ContainerID != "" && adapter == nil {
		fmt.Println("Container not found: " + params.ContainerID)
		return
	}

	if params.ContainerID != "" && adapter != nil {
		fmt.Println("Connecting to ContainerID: " + params.ContainerID)
		adapter.AddStream(&webSocket)
		return
	}

	if container, err = docker.CreateContainer(params.SourceURL); err != nil {
		log.Fatalln(err)
		return
	}

	defer container.Stop()

	if pty, err = container.Bash(); err != nil {
		log.Fatalln(err)
		return
	}
	defer pty.Stop()

	newAdapter := streams.NewAdapter(&pty, &webSocket)
	containerPool[container.ID.String()] = &newAdapter

	fmt.Println("Connecting to ContainerID: " + container.ID.String())

	err = newAdapter.Connect()
}

func containersHandler(w http.ResponseWriter, r *http.Request) {
	var (
		params    parameters
		database  db.DB
		err       error
		image     db.Image
		container db.Container
	)

	if params, err = parseJSON(r); err != nil {
		fmt.Println(err)
	}

	database = db.Connect()

	if image, err = database.CreateImage(params.SourceURL); err != nil {
		fmt.Println(err)
	}

	if container, err = database.CreateContainer(&image); err != nil {
		fmt.Println(err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(container)
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
