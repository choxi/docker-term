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

	"github.com/satori/go.uuid"
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

	// if params.ContainerID != "" && adapter == nil {
	// 	container, err = db.FindContainer(params.ContainerID)
	// 	fmt.Println("Container not found: " + params.ContainerID)
	// 	return
	// }
	database = db.Connect()

	if params.ContainerID == "" {
		fmt.Println("No ContainerID")
		return
	}

	if params.ContainerID != "" && adapter != nil {
		fmt.Println("Connecting to ContainerID: " + params.ContainerID)
		adapter.AddStream(&webSocket)
		return
	}

	if ctr, err = database.FindContainer(params.ContainerID); err != nil {
		fmt.Println("Container not found")
		return
	}

	if image, err = database.FindImage(ctr.ImageID); err != nil {
		fmt.Println("Image not found")
	}

	uid, _ := uuid.FromString(ctr.UUID)
	if dctr, err = docker.CreateContainer(uid, image.SourceURL); err != nil {
		log.Fatalln(err)
		return
	}

	defer dctr.Stop()

	if pty, err = dctr.Bash(); err != nil {
		log.Fatalln(err)
		return
	}
	defer pty.Stop()

	newAdapter := streams.NewAdapter(&pty, &webSocket)
	containerPool[dctr.ID.String()] = &newAdapter

	fmt.Println("Connecting to ContainerID: " + dctr.ID.String())

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
