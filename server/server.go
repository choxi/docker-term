package server

import (
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
	sourceURL   string
	containerID string
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
	adapter = containerPool[params.containerID]

	if params.containerID != "" && adapter == nil {
		fmt.Println("Container not found: " + params.containerID)
		return
	}

	if params.containerID != "" && adapter != nil {
		fmt.Println("Connecting to ContainerID: " + params.containerID)
		adapter.AddStream(&webSocket)
		return
	}

	if container, err = docker.CreateContainer(params.sourceURL); err != nil {
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"KEY": "VALUE"})
}

func parseParams(values url.Values) parameters {
	var params parameters

	sourceURLKey := "source_url"
	containerIDKey := "container_id"

	if len(values[sourceURLKey]) > 0 {
		params.sourceURL = utils.Decode64(values[sourceURLKey][0])
	}

	if len(values[containerIDKey]) > 0 {
		params.containerID = values["container_id"][0]
	}

	return params
}
