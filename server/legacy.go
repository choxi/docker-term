package server

import (
	"context"
	"dre/db"
	"dre/docker"
	"dre/streams"
	"dre/ws"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	uuid "github.com/satori/go.uuid"
)

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

func legacyPtyHandler(w http.ResponseWriter, r *http.Request) {
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

	log.Println("Starting container...")

	dctr.OnStart = ctr.Start
	dctr.OnStop = ctr.End

	// defer dctr.Stop()

	if pty, err = dctr.Bash(); err != nil {
		fmt.Println(err)
		http.Error(w, "Container could not be started", http.StatusInternalServerError)
		return
	}
	// defer pty.Stop()

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
		containerPool[dctr.ID.String()] = nil
	}()

	fmt.Println("Done")
}
