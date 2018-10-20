package server

import (
	"Dre/docker"
	"Dre/streams"
	"Dre/utils"
	"Dre/ws"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

// Server is a http server
type Server struct {
	handlers []*handle
}

type handle struct {
	path    string
	handler http.Handler
}

// New returns a new Server with initialized handlers
func New(staticDir string) Server {
	server := Server{}

	finalHandler := http.HandlerFunc(ptyHandler)
	ptyHandler := handle{"/pty", ws.Middleware(finalHandler)}
	staticHandler := handle{"/", http.FileServer(http.Dir(staticDir))}

	server.handlers = []*handle{&ptyHandler, &staticHandler}

	return server
}

// Start runs the server and listens on the provided port
func (s *Server) Start(port string) error {
	for _, handle := range s.handlers {
		http.Handle(handle.path, handle.handler)
	}

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
