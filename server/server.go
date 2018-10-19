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

func ptyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		pty       docker.Pty
		container docker.Container
		webSocket ws.WS
		sourceURL string
	)

	webSocket = ws.FromContext(r.Context())
	sourceURL = utils.Decode64(r.URL.Query()["source_url"][0])
	container = docker.CreateContainer(sourceURL)
	defer container.Stop()

	if pty, err = container.Bash(); err != nil {
		return
	}
	defer pty.Stop()

	adapter := streams.NewAdapter(&pty, &webSocket)
	err = adapter.Connect()
	log.Fatalf("Adapter disconnected: %s\n", err)
}
