package main

import (
	"Dre/docker"
	"Dre/streams"
	"Dre/utils"
	"Dre/ws"
	"flag"
	"log"
	"net/http"
	"os"
)

var addrFlag, cmdFlag, staticFlag string

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

func init() {
	cwd, _ := os.Getwd()
	flag.StringVar(&addrFlag, "addr", ":9000", "IP:PORT or :PORT address to listen on")
	flag.StringVar(&cmdFlag, "cmd", "/bin/bash", "command to execute on slave side of the pty")
	flag.StringVar(&staticFlag, "static", cwd, "path to static content")
	// TODO: make sure paths exist and have correct permissions
}

func main() {
	flag.Parse()

	finalHandler := http.HandlerFunc(ptyHandler)
	http.Handle("/pty", ws.Middleware(finalHandler))

	// serve html & javascript
	http.Handle("/", http.FileServer(http.Dir(staticFlag)))

	err := http.ListenAndServe(addrFlag, nil)
	if err != nil {
		log.Fatalf("net.http could not listen on address '%s': %s\n", addrFlag, err)
	}
}
